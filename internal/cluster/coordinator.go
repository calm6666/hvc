package cluster

import (
	"context"
	"hvc/internal/cluster/membership"
	"hvc/internal/config"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"time"
)

// Coordinator 表示集群协调模块。
type Coordinator struct {
	nodeMode             string
	nodeID               uint64
	nodeName             string
	hostIP               string
	httpHost             string
	grpcHost             string
	nodeRole             string
	registry             *membership.Registry
	nodeRepo             *mysql.ClusterNodeRepository
	lastHeartbeatPersist time.Time
}

const coordinatorHeartbeatPersistInterval = 15 * time.Second

// NewCoordinator 创建集群协调模块。
func NewCoordinator(nodeMode string, nodeID uint64, nodeName, hostIP, httpHost, grpcHost string, nodeRepo *mysql.ClusterNodeRepository, redisClient *rediscache.Client) *Coordinator {
	return &Coordinator{
		nodeMode: nodeMode,
		nodeID:   nodeID,
		nodeName: nodeName,
		hostIP:   hostIP,
		httpHost: httpHost,
		grpcHost: grpcHost,
		nodeRole: resolveNodeRole(nodeMode),
		registry: membership.NewRegistry(redisClient),
		nodeRepo: nodeRepo,
	}
}

// Registry 返回当前成员注册表。
func (c *Coordinator) Registry() *membership.Registry {
	return c.registry
}

// Start 启动集群协调模块。
func (c *Coordinator) Start(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			now := time.Now()
			if c.nodeRepo != nil && c.shouldPersistHeartbeat(now) {
				if err := c.nodeRepo.TouchHeartbeat(ctx, c.nodeID, now); err == nil {
					c.lastHeartbeatPersist = now
				}
			}
			c.registry.Upsert(ctx, membership.Node{
				NodeID:          c.nodeID,
				NodeName:        c.nodeName,
				Host:            firstNonEmpty(c.httpHost, c.grpcHost, c.hostIP),
				HostIP:          c.hostIP,
				GRPCHost:        c.grpcHost,
				HTTPHost:        c.httpHost,
				NodeRole:        c.nodeRole,
				LastHeartbeatAt: now,
				Enabled:         true,
				Quarantined:     false,
			})
		}
	}
}

func (c *Coordinator) shouldPersistHeartbeat(now time.Time) bool {
	if c.lastHeartbeatPersist.IsZero() {
		return true
	}
	return now.Sub(c.lastHeartbeatPersist) >= coordinatorHeartbeatPersistInterval
}

func resolveNodeRole(nodeMode string) string {
	switch nodeMode {
	case config.NodeModeStandalone, config.NodeModeClusterAllInOne, config.NodeModeClusterControl, config.NodeModeClusterWorker:
		return nodeMode
	default:
		return "custom"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
