package live

import (
	"context"
	"sync"
	"time"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

// StreamState 表示推流状态。
type StreamState int

const (
	StreamStateIdle       StreamState = iota
	StreamStateConnecting
	StreamStatePublishing
	StreamStateInterrupted
	StreamStateWaitResume
	StreamStateResuming
	StreamStateStopped
	StreamStateError
)

// StreamEvent 表示推流状态机事件。
type StreamEvent int

const (
	EventConnect      StreamEvent = iota
	EventPublishStart
	EventInterrupt
	EventResume
	EventTimeout
	EventStop
	EventError
)

// stateTransition 表示状态转换规则。
type stateTransition struct {
	from  StreamState
	event StreamEvent
	to    StreamState
}

var transitions = []stateTransition{
	{StreamStateIdle, EventConnect, StreamStateConnecting},
	{StreamStateConnecting, EventPublishStart, StreamStatePublishing},
	{StreamStateConnecting, EventError, StreamStateError},
	{StreamStatePublishing, EventInterrupt, StreamStateInterrupted},
	{StreamStatePublishing, EventStop, StreamStateStopped},
	{StreamStatePublishing, EventError, StreamStateError},
	{StreamStateInterrupted, EventResume, StreamStateResuming},
	{StreamStateInterrupted, EventTimeout, StreamStateStopped},
	{StreamStateInterrupted, EventStop, StreamStateStopped},
	{StreamStateInterrupted, EventError, StreamStateError},
	{StreamStateWaitResume, EventResume, StreamStateResuming},
	{StreamStateWaitResume, EventTimeout, StreamStateStopped},
	{StreamStateWaitResume, EventStop, StreamStateStopped},
	{StreamStateWaitResume, EventError, StreamStateError},
	{StreamStateResuming, EventPublishStart, StreamStatePublishing},
	{StreamStateResuming, EventError, StreamStateError},
	{StreamStateResuming, EventTimeout, StreamStateStopped},
	{StreamStateError, EventConnect, StreamStateConnecting},
	{StreamStateError, EventStop, StreamStateStopped},
	{StreamStateStopped, EventConnect, StreamStateConnecting},
}

// ResumeConfig 表示断流恢复配置。
type ResumeConfig struct {
	// WaitResumeTimeout 断流后等待恢复的超时时间。
	// 超过此时间未恢复，则将会话标记为已停止。
	WaitResumeTimeout time.Duration
	// MaxResumeCount 单次直播会话最大恢复次数。
	MaxResumeCount int
	// HealthCheckInterval 健康检查间隔。
	HealthCheckInterval time.Duration
}

// DefaultResumeConfig 返回默认断流恢复配置。
func DefaultResumeConfig() ResumeConfig {
	return ResumeConfig{
		WaitResumeTimeout:   30 * time.Second,
		MaxResumeCount:      5,
		HealthCheckInterval: 5 * time.Second,
	}
}

// StreamStateMachine 表示推流状态机。
//
// 管理单个推流会话的状态转换，支持断流恢复。
// 当推流中断时，状态机进入 WaitResume 状态，
// 在超时时间内如果推流恢复，则自动回到 Publishing 状态；
// 如果超时未恢复，则会话被标记为已停止。
type StreamStateMachine struct {
	mu           sync.RWMutex
	channelKey   string
	state        StreamState
	resumeCount  int
	config       ResumeConfig
	interruptedAt time.Time
	session      *model.LiveSession
	onStateChange func(channelKey string, oldState, newState StreamState)
}

// NewStreamStateMachine 创建推流状态机。
func NewStreamStateMachine(channelKey string, config ResumeConfig) *StreamStateMachine {
	return &StreamStateMachine{
		channelKey: channelKey,
		state:      StreamStateIdle,
		config:     config,
	}
}

// SetSession 绑定直播会话。
func (sm *StreamStateMachine) SetSession(session *model.LiveSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.session = session
}

// SendEvent 发送事件到状态机。
//
// 根据当前状态和事件类型，执行状态转换。
// 如果转换合法，更新状态并调用回调函数。
// 返回转换后的状态和是否成功。
func (sm *StreamStateMachine) SendEvent(event StreamEvent) (StreamState, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	oldState := sm.state
	for _, t := range transitions {
		if t.from == oldState && t.event == event {
			sm.state = t.to
			sm.onTransition(oldState, t.to, event)
			if sm.onStateChange != nil {
				sm.onStateChange(sm.channelKey, oldState, t.to)
			}
			return sm.state, true
		}
	}

	logx.Info("live.statemachine.transition_rejected", logx.Fields{
		"channel_key": sm.channelKey,
		"old_state":   stateName(oldState),
		"event":       eventName(event),
	})
	return oldState, false
}

// State 返回当前状态。
func (sm *StreamStateMachine) State() StreamState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// ResumeCount 返回当前恢复次数。
func (sm *StreamStateMachine) ResumeCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.resumeCount
}

// CanResume 判断是否还能恢复。
func (sm *StreamStateMachine) CanResume() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.resumeCount < sm.config.MaxResumeCount
}

// SetOnStateChange 设置状态变更回调。
func (sm *StreamStateMachine) SetOnStateChange(fn func(channelKey string, oldState, newState StreamState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onStateChange = fn
}

// onTransition 处理状态转换的副作用。
func (sm *StreamStateMachine) onTransition(oldState, newState StreamState, event StreamEvent) {
	switch {
	case newState == StreamStateInterrupted || newState == StreamStateWaitResume:
		sm.interruptedAt = time.Now()
		logx.Info("live.statemachine.interrupted", logx.Fields{
			"channel_key":    sm.channelKey,
			"interrupted_at": sm.interruptedAt,
			"resume_count":   sm.resumeCount,
		})
	case oldState == StreamStateInterrupted && newState == StreamStateResuming:
		sm.resumeCount++
		logx.Info("live.statemachine.resuming", logx.Fields{
			"channel_key":  sm.channelKey,
			"resume_count": sm.resumeCount,
		})
	case oldState == StreamStateWaitResume && newState == StreamStateResuming:
		sm.resumeCount++
		logx.Info("live.statemachine.resuming_from_wait", logx.Fields{
			"channel_key":  sm.channelKey,
			"resume_count": sm.resumeCount,
		})
	case newState == StreamStatePublishing:
		if sm.session != nil {
			sm.session.ResumeCount = sm.resumeCount
			sm.session.Status = model.LiveSessionStatusPublishing
		}
		logx.Info("live.statemachine.publishing", logx.Fields{
			"channel_key":  sm.channelKey,
			"resume_count": sm.resumeCount,
		})
	case newState == StreamStateStopped:
		logx.Info("live.statemachine.stopped", logx.Fields{
			"channel_key":  sm.channelKey,
			"resume_count": sm.resumeCount,
		})
	case newState == StreamStateError:
		logx.Info("live.statemachine.error", logx.Fields{
			"channel_key":  sm.channelKey,
		})
	}
}

// InterruptedDuration 返回中断持续时间。
func (sm *StreamStateMachine) InterruptedDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.interruptedAt.IsZero() {
		return 0
	}
	return time.Since(sm.interruptedAt)
}

// IsInterruptedTooLong 判断中断时间是否超过等待恢复超时。
func (sm *StreamStateMachine) IsInterruptedTooLong() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.interruptedAt.IsZero() {
		return false
	}
	return time.Since(sm.interruptedAt) > sm.config.WaitResumeTimeout
}

// StreamGuard 推流守护器。
//
// 周期性检查所有活跃会话的中断状态，
// 对超时未恢复的会话自动触发超时事件。
type StreamGuard struct {
	mu         sync.RWMutex
	machines   map[string]*StreamStateMachine
	config     ResumeConfig
	cancelFunc context.CancelFunc
}

// NewStreamGuard 创建推流守护器。
func NewStreamGuard(config ResumeConfig) *StreamGuard {
	return &StreamGuard{
		machines: make(map[string]*StreamStateMachine),
		config:   config,
	}
}

// Register 注册推流状态机。
func (g *StreamGuard) Register(channelKey string, sm *StreamStateMachine) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.machines[channelKey] = sm
}

// Unregister 注销推流状态机。
func (g *StreamGuard) Unregister(channelKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.machines, channelKey)
}

// Start 启动守护器。
func (g *StreamGuard) Start(ctx context.Context) {
	childCtx, cancel := context.WithCancel(ctx)
	g.cancelFunc = cancel
	ticker := time.NewTicker(g.config.HealthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-childCtx.Done():
			return
		case <-ticker.C:
			g.checkOnce()
		}
	}
}

// Stop 停止守护器。
func (g *StreamGuard) Stop() {
	if g.cancelFunc != nil {
		g.cancelFunc()
	}
}

// checkOnce 执行一次健康检查。
func (g *StreamGuard) checkOnce() {
	g.mu.RLock()
	machines := make([]*StreamStateMachine, 0, len(g.machines))
	for _, sm := range g.machines {
		machines = append(machines, sm)
	}
	g.mu.RUnlock()

	for _, sm := range machines {
		state := sm.State()
		if (state == StreamStateInterrupted || state == StreamStateWaitResume) && sm.IsInterruptedTooLong() {
			sm.SendEvent(EventTimeout)
			logx.Info("live.guard.session_timeout", logx.Fields{
				"channel_key":        sm.channelKey,
				"interrupted_duration": sm.InterruptedDuration().String(),
			})
		}
	}
}

// stateName 返回状态名称。
func stateName(s StreamState) string {
	switch s {
	case StreamStateIdle:
		return "IDLE"
	case StreamStateConnecting:
		return "CONNECTING"
	case StreamStatePublishing:
		return "PUBLISHING"
	case StreamStateInterrupted:
		return "INTERRUPTED"
	case StreamStateWaitResume:
		return "WAIT_RESUME"
	case StreamStateResuming:
		return "RESUMING"
	case StreamStateStopped:
		return "STOPPED"
	case StreamStateError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// eventName 返回事件名称。
func eventName(e StreamEvent) string {
	switch e {
	case EventConnect:
		return "CONNECT"
	case EventPublishStart:
		return "PUBLISH_START"
	case EventInterrupt:
		return "INTERRUPT"
	case EventResume:
		return "RESUME"
	case EventTimeout:
		return "TIMEOUT"
	case EventStop:
		return "STOP"
	case EventError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
