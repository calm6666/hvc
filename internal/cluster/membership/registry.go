package membership

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	rediscache "hvc/internal/infra/cache/redis"
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
	mu     sync.RWMutex
	nodes  map[uint64]Node
	client *rediscache.Client
}

const (
	memberKeyPrefix = "hvc:cluster:member:"
	memberTTL       = 2 * time.Minute
)

// NewRegistry 创建节点成员注册表。
//
// 当传入 Redis 客户端时，成员信息会同步写入共享 Redis 热路径，
// 后台多机集群接口读取到的就是全局成员视图，而不是当前进程的本地内存副本。
func NewRegistry(client *rediscache.Client) *Registry {
	return &Registry{
		nodes:  make(map[uint64]Node),
		client: client,
	}
}

// Upsert 更新节点成员信息。
func (r *Registry) Upsert(ctx context.Context, node Node) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[node.NodeID] = node
	r.saveRemote(ctx, node)
}

// List 返回节点列表。
func (r *Registry) List(ctx context.Context) []Node {
	if items, ok := r.listRemote(ctx); ok {
		return items
	}
	return r.listLocal()
}

func (r *Registry) listLocal() []Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		items = append(items, node)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].NodeID < items[j].NodeID
	})
	return items
}

func (r *Registry) saveRemote(ctx context.Context, node Node) {
	if r == nil || r.client == nil || r.client.Engine == nil || node.NodeID == 0 {
		return
	}
	payload, err := json.Marshal(node)
	if err != nil {
		return
	}
	_ = r.client.Engine.Set(ctx, memberKey(node.NodeID), payload, memberTTL).Err()
}

func (r *Registry) listRemote(ctx context.Context) ([]Node, bool) {
	if r == nil || r.client == nil || r.client.Engine == nil {
		return nil, false
	}

	keys, err := r.scanMemberKeys(ctx)
	if err != nil || len(keys) == 0 {
		return nil, false
	}

	values, err := r.client.Engine.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false
	}

	items := make([]Node, 0, len(values))
	for _, raw := range values {
		payload, ok := raw.(string)
		if !ok || payload == "" {
			continue
		}
		var node Node
		if err := json.Unmarshal([]byte(payload), &node); err != nil {
			continue
		}
		if node.NodeID == 0 {
			continue
		}
		items = append(items, node)
	}
	if len(items) == 0 {
		return nil, false
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].NodeID < items[j].NodeID
	})
	return items, true
}

func (r *Registry) scanMemberKeys(ctx context.Context) ([]string, error) {
	if r == nil || r.client == nil || r.client.Engine == nil {
		return nil, nil
	}

	keys := make([]string, 0, 16)
	var cursor uint64
	for {
		items, next, err := r.client.Engine.Scan(ctx, cursor, memberKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, items...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func memberKey(nodeID uint64) string {
	return fmt.Sprintf("%s%d", memberKeyPrefix, nodeID)
}
