package hotpath

import (
	"context"
	"hvc/internal/model"
	"sync"
)

// MemoryBus 表示热路径内存总线。
type MemoryBus struct {
	mu       sync.RWMutex
	progress map[uint64]model.TranscodeProgress
}

// NewMemoryBus 创建热路径内存总线。
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{progress: make(map[uint64]model.TranscodeProgress)}
}

// SaveProgress 保存实时进度。
func (b *MemoryBus) SaveProgress(ctx context.Context, progress model.TranscodeProgress) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.progress[progress.JobID] = progress
}

// ActiveProgressCount 返回当前内存总线中仍保留的实时进度条目数量。
//
// 这里返回的是当前进程热路径里仍在维护的任务进度数量，适合被 Worker 当作“当前活跃转码会话数”的近似值使用。
// 当前实现不区分任务是否已经最终完成，因此调用方仍应结合自身执行节奏决定如何解释这个数量。
func (b *MemoryBus) ActiveProgressCount(ctx context.Context) int {
	_ = ctx
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.progress)
}
