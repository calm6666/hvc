package cluster

import (
	"context"
	"hvc/internal/cluster/membership"
	"hvc/internal/config"
	"time"
)

// Coordinator 表示集群协调模块。
type Coordinator struct {
	cfg           config.DynamicRuntimeConfig
	nodeID        uint64
	nodeName      string
	hostIP        string
	httpHost      string
	grpcHost      string
	nodeRole      string
	registry      *membership.Registry
}

// NewCoordinator 创建集群协调模块。
func NewCoordinator(cfg config.DynamicRuntimeConfig, nodeID uint64, nodeName, hostIP, httpHost, grpcHost string) *Coordinator {
	return &Coordinator{
		cfg:      cfg,
		nodeID:   nodeID,
		nodeName: nodeName,
		hostIP:   hostIP,
		httpHost: httpHost,
		grpcHost: grpcHost,
		nodeRole: resolveNodeRole(cfg),
		registry: membership.NewRegistry(),
	}
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
				NodeName:        c.nodeName,
				Host:            firstNonEmpty(c.httpHost, c.grpcHost, c.hostIP),
				HostIP:          c.hostIP,
				GRPCHost:        c.grpcHost,
				HTTPHost:        c.httpHost,
				NodeRole:        c.nodeRole,
				LastHeartbeatAt: time.Now(),
				Enabled:         true,
				Quarantined:     false,
			})
		}
	}
}

func resolveNodeRole(cfg config.DynamicRuntimeConfig) string {
	if cfg.IsStandalone() {
		return "standalone"
	}
	if cfg.IsClusterAllInOne() {
		return "cluster-allinone"
	}
	if cfg.IsClusterControl() {
		return "cluster-control"
	}
	if cfg.IsClusterWorker() {
		return "cluster-worker"
	}
	return "custom"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
