package cluster

import (
	"context"
	"time"

	"hvc/internal/config"
)

// Coordinator 表示集群协调模块。
type Coordinator struct {
	cfg config.DynamicRuntimeConfig
}

// NewCoordinator 创建集群协调模块。
func NewCoordinator(cfg config.DynamicRuntimeConfig) *Coordinator {
	return &Coordinator{cfg: cfg}
}

// Start 启动集群协调模块。
func (c *Coordinator) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.Scheduler.LoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
