// Package etcd 提供 etcd 服务注册与发现实现。
//
// 支持集群模式和单机模式：
//   - 集群模式：通过 etcd 实现服务注册、发现、心跳保活和优雅下线
//   - 单机模式：当 etcd 不可用时，退化为内存注册表，仅维护本机节点信息
//
// 注册机制：
//  1. 服务启动时在 etcd 创建带 TTL 的租约（Lease）
//  2. 将节点信息注册到 etcd 的 key 上，绑定租约
//  3. 通过 KeepAlive 心跳续租，保持注册信息存活
//  4. 服务停止时主动撤销租约，实现优雅下线
//
// 集群模式下多节点发现：
//   - 所有节点注册到 /hvc/nodes/ 前缀下
//   - 调度器通过 Discover 查询前缀获取所有在线节点
//   - Watch 监听前缀变化，实时感知节点上下线
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
	"hvc/internal/config"
	"hvc/pkg/logx"
)

// NodeInfo 表示注册到 etcd 的节点信息。
type NodeInfo struct {
	NodeID    uint64 `json:"node_id"`
	WorkerID  string `json:"worker_id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Status    string `json:"status"`
	StartTime int64  `json:"start_time"`
}

// Registry 表示 etcd 注册中心包装。
type Registry struct {
	Endpoint string
	cfg      config.ConfigCenterConfig
	nodeInfo NodeInfo

	mu             sync.RWMutex
	local          bool
	client         *clientv3.Client
	leaseID        clientv3.LeaseID
	cancelKeepAlive context.CancelFunc
	nodesCache     []NodeInfo
}

// NewRegistry 创建 etcd 注册中心包装。
func NewRegistry(cfg config.RuntimeConfig) *Registry {
	return &Registry{
		Endpoint: cfg.ConfigCenter.Endpoint,
		cfg:      cfg.ConfigCenter,
		local:    !cfg.ConfigCenter.Enabled,
	}
}

// connect 连接到 etcd 集群。
func (r *Registry) connect(ctx context.Context) error {
	if r.client != nil {
		return nil
	}

	endpoints := []string{r.Endpoint}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		r.local = true
		logx.Error("registry.etcd.connect_failed", err, logx.Fields{
			"endpoint": r.Endpoint,
		})
		return fmt.Errorf("连接 etcd 失败: %w", err)
	}

	r.client = cli
	r.local = false
	return nil
}

// Register 注册当前节点到 etcd。
func (r *Registry) Register(ctx context.Context, nodeInfo NodeInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodeInfo = nodeInfo

	if r.local || r.Endpoint == "" {
		logx.Info("registry.etcd.single_mode", logx.Fields{
			"node_id":   nodeInfo.NodeID,
			"worker_id": nodeInfo.WorkerID,
			"mode":      "standalone",
		})
		return nil
	}

	if err := r.connect(ctx); err != nil {
		logx.Error("registry.etcd.connect_failed_fallback_single", err, nil)
		r.local = true
		return nil
	}

	key := r.nodeKey(nodeInfo.WorkerID)
	value, err := json.Marshal(nodeInfo)
	if err != nil {
		return fmt.Errorf("序列化节点信息失败: %w", err)
	}

	leaseResp, err := r.client.Grant(ctx, 30)
	if err != nil {
		return fmt.Errorf("创建租约失败: %w", err)
	}
	r.leaseID = leaseResp.ID

	_, err = r.client.Put(ctx, key, string(value), clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("注册节点失败: %w", err)
	}

	keepAliveCtx, cancel := context.WithCancel(context.Background())
	r.cancelKeepAlive = cancel

	go func() {
		ch, kaErr := r.client.KeepAlive(keepAliveCtx, r.leaseID)
		if kaErr != nil {
			logx.Error("registry.etcd.keepalive_failed", kaErr, logx.Fields{
				"worker_id": nodeInfo.WorkerID,
			})
			return
		}
		for range ch {
		}
	}()

	logx.Info("registry.etcd.register_success", logx.Fields{
		"key":       key,
		"node_id":   nodeInfo.NodeID,
		"worker_id": nodeInfo.WorkerID,
		"lease_id":  r.leaseID,
	})
	return nil
}

// Deregister 从 etcd 注销当前节点。
func (r *Registry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelKeepAlive != nil {
		r.cancelKeepAlive()
		r.cancelKeepAlive = nil
	}

	if r.local {
		logx.Info("registry.etcd.deregister_single_mode", logx.Fields{
			"node_id":   r.nodeInfo.NodeID,
			"worker_id": r.nodeInfo.WorkerID,
		})
		return nil
	}

	if r.client != nil && r.leaseID != 0 {
		_, err := r.client.Revoke(ctx, r.leaseID)
		if err != nil {
			logx.Error("registry.etcd.revoke_failed", err, logx.Fields{
				"lease_id": r.leaseID,
			})
		}
	}

	logx.Info("registry.etcd.deregister_success", logx.Fields{
		"node_id":   r.nodeInfo.NodeID,
		"worker_id": r.nodeInfo.WorkerID,
	})
	return nil
}

// Discover 从 etcd 发现指定服务的所有节点。
func (r *Registry) Discover(ctx context.Context, servicePrefix string) ([]NodeInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.local {
		return []NodeInfo{r.nodeInfo}, nil
	}

	if r.client == nil {
		return r.nodesCache, nil
	}

	resp, err := r.client.Get(ctx, servicePrefix, clientv3.WithPrefix())
	if err != nil {
		logx.Error("registry.etcd.discover_failed", err, logx.Fields{
			"prefix": servicePrefix,
		})
		return r.nodesCache, nil
	}

	var nodes []NodeInfo
	for _, kv := range resp.Kvs {
		var info NodeInfo
		if jsonErr := json.Unmarshal(kv.Value, &info); jsonErr != nil {
			logx.Error("registry.etcd.unmarshal_failed", jsonErr, logx.Fields{
				"key": string(kv.Key),
			})
			continue
		}
		nodes = append(nodes, info)
	}

	r.nodesCache = nodes
	return nodes, nil
}

// Watch 监听 etcd 中指定前缀的节点变化。
func (r *Registry) Watch(ctx context.Context, servicePrefix string) <-chan []NodeInfo {
	ch := make(chan []NodeInfo, 8)

	if r.local {
		r.mu.RLock()
		ch <- []NodeInfo{r.nodeInfo}
		r.mu.RUnlock()
		close(ch)
		return ch
	}

	if r.client == nil {
		close(ch)
		return ch
	}

	go func() {
		defer close(ch)
		watchCh := r.client.Watch(ctx, servicePrefix, clientv3.WithPrefix())
		for {
			select {
			case <-ctx.Done():
				return
			case watchResp, ok := <-watchCh:
				if !ok {
					return
				}
				var nodes []NodeInfo
				for _, event := range watchResp.Events {
					var info NodeInfo
					if err := json.Unmarshal(event.Kv.Value, &info); err != nil {
						continue
					}
					nodes = append(nodes, info)
				}
				if len(nodes) > 0 {
					select {
					case ch <- nodes:
					default:
					}
				}
			}
		}
	}()

	return ch
}

// Close 关闭 etcd 客户端连接。
func (r *Registry) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// IsLocalMode 返回是否运行在单机模式。
func (r *Registry) IsLocalMode() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.local
}

// GetNodeInfo 获取当前节点信息。
func (r *Registry) GetNodeInfo() NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nodeInfo
}

// nodeKey 生成 etcd 中节点注册的 key。
func (r *Registry) nodeKey(workerID string) string {
	return fmt.Sprintf("/hvc/nodes/%s", workerID)
}
