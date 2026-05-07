package cluster

import (
	"context"
	"hvc/internal/cluster/membership"
	"hvc/internal/config"
	"time"
)

// Coordinator 表示集群协调模块。
type Coordinator struct {
	cfg      config.DynamicRuntimeConfig
	nodeID   uint64
	host     string
	registry *membership.Registry
}

// NewCoordinator 创建集群协调模块。
func NewCoordinator(cfg config.DynamicRuntimeConfig, nodeID uint64, host string) *Coordinator {
	return &Coordinator{cfg: cfg, nodeID: nodeID, host: host, registry: membership.NewRegistry()}
}

// Registry 返回当前成员注册表。
func (c *Coordinator) Registry() *membership.Registry {
	return c.registry
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
			c.registry.Upsert(ctx, membership.Node{
				NodeID:          c.nodeID,
				Host:            c.host,
				LastHeartbeatAt: time.Now(),
				Enabled:         true,
				Quarantined:     false,
			})
		}
	}
}
