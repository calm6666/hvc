// Package runtime 提供 Worker 运行时会话管理。
//
// SessionGuard 用于保护 Worker 的转码执行会话，防止以下问题：
//   - 超出节点最大并发转码会话数
//   - 超出单 GPU 最大并发会话数
//   - 租约过期后仍在执行已重新分配的任务
//   - 资源不足时继续接受新任务导致系统过载
//
// 使用方式：
//
//	guard := runtime.NewSessionGuard(cfg)
//	if !guard.TryAcquire(job) {
//	    // 无法获取会话，任务需要等待
//	    return
//	}
//	defer guard.Release(job)
//	// 执行转码任务...
package runtime

import (
	"sync"
	"sync/atomic"

	"hvc/internal/config"
	"hvc/internal/model"
)

// SessionGuard 表示执行会话保护器。
//
// 通过原子计数器跟踪当前活跃的转码会话数，
// 在接受新任务前检查是否超出配置的并发上限。
//
// 支持多 GPU 场景：每块 GPU 独立计数会话数，
// 防止某块 GPU 过载而其他 GPU 空闲。
type SessionGuard struct {
	cfg           config.DynamicRuntimeConfig
	totalSessions atomic.Int32
	gpuSessions   map[int]*atomic.Int32
	mu            sync.RWMutex
}

// NewSessionGuard 创建执行会话保护器。
func NewSessionGuard(cfg config.DynamicRuntimeConfig) *SessionGuard {
	return &SessionGuard{
		cfg:         cfg,
		gpuSessions: make(map[int]*atomic.Int32),
	}
}

// TryAcquire 尝试获取一个执行会话槽位。
//
// 检查条件：
//  1. 当前总活跃会话数 < 节点最大并发转码会话数
//  2. 如果任务指定了 GPU 索引，该 GPU 的活跃会话数 < 单 GPU 最大会话数
//
// 获取成功后自动增加计数，调用者必须在任务完成后调用 Release 释放。
func (g *SessionGuard) TryAcquire(job model.TranscodeJob) bool {
	maxSessions := int32(g.cfg.Scheduler.MaxNodeTranscodeSessions)
	if maxSessions <= 0 {
		maxSessions = 10
	}

	current := g.totalSessions.Load()
	if current >= maxSessions {
		return false
	}

	if job.SelectedGPUIndex >= 0 {
		gpuMax := int32(g.cfg.Scheduler.MaxNodeTranscodeSessions)
		gpuCount := g.getGPUSessions(job.SelectedGPUIndex)
		if gpuCount >= gpuMax {
			return false
		}
	}

	if !g.totalSessions.CompareAndSwap(current, current+1) {
		return false
	}

	if job.SelectedGPUIndex >= 0 {
		g.incrementGPUSessions(job.SelectedGPUIndex)
	}

	return true
}

// Release 释放一个执行会话槽位。
//
// 必须在任务完成后（无论成功或失败）调用，否则会话计数会持续增长，
// 最终导致 Worker 无法接受新任务。
func (g *SessionGuard) Release(job model.TranscodeJob) {
	g.totalSessions.Add(-1)

	if job.SelectedGPUIndex >= 0 {
		g.decrementGPUSessions(job.SelectedGPUIndex)
	}
}

// ActiveCount 返回当前活跃会话总数。
func (g *SessionGuard) ActiveCount() int {
	return int(g.totalSessions.Load())
}

// GPUActiveCount 返回指定 GPU 上的活跃会话数。
func (g *SessionGuard) GPUActiveCount(gpuIndex int) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if counter, ok := g.gpuSessions[gpuIndex]; ok {
		return int(counter.Load())
	}
	return 0
}

// CanAcceptMore 判断是否还能接受更多任务。
func (g *SessionGuard) CanAcceptMore() bool {
	maxSessions := int32(g.cfg.Scheduler.MaxNodeTranscodeSessions)
	if maxSessions <= 0 {
		maxSessions = 10
	}
	return g.totalSessions.Load() < maxSessions
}

func (g *SessionGuard) getGPUSessions(gpuIndex int) int32 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if counter, ok := g.gpuSessions[gpuIndex]; ok {
		return counter.Load()
	}
	return 0
}

func (g *SessionGuard) incrementGPUSessions(gpuIndex int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.gpuSessions[gpuIndex]; !ok {
		g.gpuSessions[gpuIndex] = &atomic.Int32{}
	}
	g.gpuSessions[gpuIndex].Add(1)
}

func (g *SessionGuard) decrementGPUSessions(gpuIndex int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if counter, ok := g.gpuSessions[gpuIndex]; ok {
		val := counter.Add(-1)
		if val <= 0 {
			delete(g.gpuSessions, gpuIndex)
		}
	}
}
