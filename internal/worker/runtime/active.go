// Package runtime 提供 Worker 运行时会话管理。
//
// Active 表示一个活跃的转码执行会话，跟踪任务的完整生命周期。
// 从任务被 Worker 接受开始，到任务完成或失败结束。
//
// Active 记录的信息包括：
//   - 关联的转码任务
//   - 执行开始时间
//   - 当前执行阶段
//   - GPU 分配信息（支持多 GPU 场景）
//   - 取消信号通道
package runtime

import (
	"context"
	"sync"
	"time"

	"hvc/internal/model"
)

// Active 表示活跃执行会话。
//
// 每个 Active 实例对应一个正在执行的转码任务，
// 包含任务执行过程中的所有运行时状态。
type Active struct {
	Job         model.TranscodeJob
	StartedAt   time.Time
	Stage       string
	GPUIndex    int
	CancelFn    context.CancelFunc
	mu          sync.RWMutex
	completed   bool
}

// NewActive 创建活跃执行会话。
//
// 参数：
//   - job: 关联的转码任务
//   - gpuIndex: 分配的 GPU 索引（-1 表示使用软解）
//   - cancelFn: 用于取消任务执行的函数
func NewActive(job model.TranscodeJob, gpuIndex int, cancelFn context.CancelFunc) *Active {
	return &Active{
		Job:       job,
		StartedAt: time.Now(),
		Stage:     model.StageTranscoding,
		GPUIndex:  gpuIndex,
		CancelFn:  cancelFn,
	}
}

// UpdateStage 更新执行阶段。
func (a *Active) UpdateStage(stage string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Stage = stage
}

// GetStage 获取当前执行阶段。
func (a *Active) GetStage() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Stage
}

// MarkCompleted 标记会话为已完成。
func (a *Active) MarkCompleted() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.completed = true
	a.Stage = model.StageCompleted
}

// IsCompleted 判断会话是否已完成。
func (a *Active) IsCompleted() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.completed
}

// Elapsed 返回会话已执行时长。
func (a *Active) Elapsed() time.Duration {
	return time.Since(a.StartedAt)
}

// Cancel 取消任务执行。
//
// 调用后会触发 context 取消，FFmpeg 进程将被终止。
func (a *Active) Cancel() {
	if a.CancelFn != nil {
		a.CancelFn()
	}
}

// ActiveRegistry 管理所有活跃执行会话。
//
// 支持按 JobID 查询、按 GPU 索引统计等功能，
// 用于 Worker 主模块的会话管理和资源保护。
type ActiveRegistry struct {
	mu      sync.RWMutex
	sessions map[uint64]*Active
}

// NewActiveRegistry 创建活跃会话注册表。
func NewActiveRegistry() *ActiveRegistry {
	return &ActiveRegistry{
		sessions: make(map[uint64]*Active),
	}
}

// Register 注册活跃会话。
func (r *ActiveRegistry) Register(active *Active) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[active.Job.JobID] = active
}

// Unregister 注销活跃会话。
func (r *ActiveRegistry) Unregister(jobID uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, jobID)
}

// Get 获取指定任务的活跃会话。
func (r *ActiveRegistry) Get(jobID uint64) (*Active, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	active, ok := r.sessions[jobID]
	return active, ok
}

// Count 返回活跃会话总数。
func (r *ActiveRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// GPUCount 返回指定 GPU 上的活跃会话数。
func (r *ActiveRegistry) GPUCount(gpuIndex int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, active := range r.sessions {
		if active.GPUIndex == gpuIndex {
			count++
		}
	}
	return count
}

// ListByGPU 返回指定 GPU 上的所有活跃会话。
func (r *ActiveRegistry) ListByGPU(gpuIndex int) []*Active {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*Active
	for _, active := range r.sessions {
		if active.GPUIndex == gpuIndex {
			result = append(result, active)
		}
	}
	return result
}

// ListAll 返回所有活跃会话。
func (r *ActiveRegistry) ListAll() []*Active {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Active, 0, len(r.sessions))
	for _, active := range r.sessions {
		result = append(result, active)
	}
	return result
}
