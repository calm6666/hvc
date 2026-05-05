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

// GetProgress 获取实时进度。
func (b *MemoryBus) GetProgress(ctx context.Context, jobID uint64) (model.TranscodeProgress, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	progress, ok := b.progress[jobID]
	return progress, ok
}
