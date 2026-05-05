package etcd

import "hvc/internal/config"

// Registry 表示 etcd 注册中心包装。
type Registry struct {
	Endpoint string
}

// NewRegistry 创建 etcd 注册中心包装。
func NewRegistry(cfg config.RuntimeConfig) *Registry {
	return &Registry{Endpoint: cfg.ConfigCenter.Endpoint}
}
