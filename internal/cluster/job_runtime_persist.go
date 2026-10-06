package cluster

import (
	"sync"
	"time"

	"hvc/internal/model"
)

const (
	// jobProgressPersistInterval 控制任务主表进度写库的最小时间间隔。
	//
	// Redis 进度仍然是实时路径，数据库只保留后台列表/详情需要的近实时快照，
	// 不再承接 ffmpeg 高频进度事件的逐条落库。
	jobProgressPersistInterval = 5 * time.Second
	// jobProgressPersistDeltaPermille 控制“进度变化足够明显时立即落库”的阈值。
	//
	// 50 表示 5% 进度变化。这样后台任务列表既不会长期停在旧值，
	// 也不会因为每 0.1% 抖动就反复写库。
	jobProgressPersistDeltaPermille = 50
)

type executionPersistKey struct {
	jobID           uint64
	leaseGeneration uint64
}

type jobProgressPersistState struct {
	persistedAt      time.Time
	status           int
	stage            string
	progressPermille int
}

// JobRuntimePersistGate 控制任务进度和执行心跳的数据库持久化节奏。
type JobRuntimePersistGate struct {
	mu                   sync.RWMutex
	progressStateByJob   map[uint64]jobProgressPersistState
	executionHeartbeatAt map[executionPersistKey]time.Time
}

// NewJobRuntimePersistGate 创建任务运行态持久化门闩。
func NewJobRuntimePersistGate() *JobRuntimePersistGate {
	return &JobRuntimePersistGate{
		progressStateByJob:   make(map[uint64]jobProgressPersistState),
		executionHeartbeatAt: make(map[executionPersistKey]time.Time),
	}
}

// ShouldPersistProgress 判断本次进度是否需要写入数据库。
func (g *JobRuntimePersistGate) ShouldPersistProgress(snapshot model.ProgressSnapshot, now time.Time) bool {
	if g == nil || snapshot.JobID == 0 {
		return true
	}
	g.mu.RLock()
	state, ok := g.progressStateByJob[snapshot.JobID]
	g.mu.RUnlock()
	if !ok || state.persistedAt.IsZero() {
		return true
	}
	if snapshot.Status != state.status || snapshot.Stage != state.stage {
		return true
	}
	if snapshot.ProgressPermille < state.progressPermille {
		return true
	}
	if snapshot.ProgressPermille-state.progressPermille >= jobProgressPersistDeltaPermille {
		return true
	}
	return now.Sub(state.persistedAt) >= jobProgressPersistInterval
}

// MarkProgressPersisted 记录任务进度最近一次成功写库状态。
func (g *JobRuntimePersistGate) MarkProgressPersisted(snapshot model.ProgressSnapshot, at time.Time) {
	if g == nil || snapshot.JobID == 0 {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.progressStateByJob[snapshot.JobID] = jobProgressPersistState{
		persistedAt:      at,
		status:           snapshot.Status,
		stage:            snapshot.Stage,
		progressPermille: snapshot.ProgressPermille,
	}
}

// ShouldPersistExecutionHeartbeat 判断执行实例心跳本轮是否需要落库。
func (g *JobRuntimePersistGate) ShouldPersistExecutionHeartbeat(now time.Time, jobID uint64, leaseGeneration uint64, timeout time.Duration) bool {
	if g == nil || jobID == 0 || leaseGeneration == 0 {
		return true
	}
	key := executionPersistKey{jobID: jobID, leaseGeneration: leaseGeneration}
	g.mu.RLock()
	lastAt := g.executionHeartbeatAt[key]
	g.mu.RUnlock()
	if lastAt.IsZero() {
		return true
	}
	return now.Sub(lastAt) >= ResolveHeartbeatPersistInterval(timeout)
}

// MarkExecutionHeartbeatPersisted 记录执行实例最近一次成功心跳落库时间。
func (g *JobRuntimePersistGate) MarkExecutionHeartbeatPersisted(jobID uint64, leaseGeneration uint64, at time.Time) {
	if g == nil || jobID == 0 || leaseGeneration == 0 {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	key := executionPersistKey{jobID: jobID, leaseGeneration: leaseGeneration}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.executionHeartbeatAt[key] = at
}
