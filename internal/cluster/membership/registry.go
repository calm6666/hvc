package membership

import (
	"context"
	"sync"
	"time"
)

// Node 表示集群成员节点。
type Node struct {
	NodeID          uint64
	NodeName        string
	Host            string
	HostIP          string
	GRPCHost        string
	HTTPHost        string
	NodeRole        string
	LastHeartbeatAt time.Time
	Enabled         bool
	Quarantined     bool
}

// Registry 表示节点成员注册表。
type Registry struct {
	mu    sync.RWMutex
	nodes map[uint64]Node
}

// NewRegistry 创建节点成员注册表。
func NewRegistry() *Registry {
	return &Registry{nodes: make(map[uint64]Node)}
}

// Upsert 更新节点成员信息。
func (r *Registry) Upsert(ctx context.Context, node Node) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[node.NodeID] = node
}

// List 返回节点列表。
func (r *Registry) List(ctx context.Context) []Node {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		items = append(items, node)
	}
	return items
}
