// Package live 提供直播频道领域对象。
//
// Channel 表示一个直播频道，Session 表示一次推流会话。
// 一个频道可以有多次推流会话（断流重连场景）。
//
// 推流会话状态机：
//
//	CONNECTING → PUBLISHING → INTERRUPT_WAIT_RESUME → RESUMED → STOPPED
//	                                           ↓
//	                                      STOPPED（超时）
//	CONNECTING → REJECTED（鉴权失败）
//
// 支持集群模式和单机模式：
//   - 集群模式：频道可被调度到任意节点执行，会话状态通过 Redis 同步
//   - 单机模式：频道在本机执行，会话状态仅保存在内存
package live

import "time"

// Channel 表示直播频道领域对象。
type Channel struct {
	ChannelID       uint64
	ChannelKey      string
	ChannelName     string
	ProfileID       uint64
	PushURL         string
	Status          string
	AssignedNodeID  uint64
	AssignedWorkerID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ChannelStatus 定义频道状态常量。
const (
	ChannelStatusIdle     = "IDLE"
	ChannelStatusStarting = "STARTING"
	ChannelStatusLive     = "LIVE"
	ChannelStatusStopped  = "STOPPED"
	ChannelStatusError    = "ERROR"
)

// IsLive 判断频道是否正在直播。
func (c *Channel) IsLive() bool {
	return c.Status == ChannelStatusLive
}

// CanStart 判断频道是否可以启动直播。
func (c *Channel) CanStart() bool {
	return c.Status == ChannelStatusIdle || c.Status == ChannelStatusStopped || c.Status == ChannelStatusError
}

// CanStop 判断频道是否可以停止直播。
func (c *Channel) CanStop() bool {
	return c.Status == ChannelStatusLive || c.Status == ChannelStatusStarting
}

// Session 表示推流会话领域对象。
type Session struct {
	SessionID       uint64
	ChannelID       uint64
	ChannelKey      string
	Status          string
	PushProtocol    string
	PushURL         string
	AssignedNodeID  uint64
	AssignedWorkerID string
	StartedAt       time.Time
	StoppedAt       time.Time
	ResumeCount     int
}

// SessionStatus 定义会话状态常量。
const (
	SessionStatusConnecting          = "CONNECTING"
	SessionStatusPublishing          = "PUBLISHING"
	SessionStatusInterruptWaitResume = "INTERRUPT_WAIT_RESUME"
	SessionStatusResumed             = "RESUMED"
	SessionStatusStopped             = "STOPPED"
	SessionStatusRejected            = "REJECTED"
)

// IsActive 判断会话是否处于活跃状态。
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusConnecting ||
		s.Status == SessionStatusPublishing ||
		s.Status == SessionStatusInterruptWaitResume ||
		s.Status == SessionStatusResumed
}

// CanResume 判断会话是否可以恢复。
func (s *Session) CanResume() bool {
	return s.Status == SessionStatusInterruptWaitResume
}

// Duration 返回会话持续时间。
func (s *Session) Duration() time.Duration {
	if s.StoppedAt.IsZero() {
		return time.Since(s.StartedAt)
	}
	return s.StoppedAt.Sub(s.StartedAt)
}
