package configcenter

import (
	"sync"

	"hvc/internal/config"
)

// EffectiveConfig 表示当前生效配置。
type EffectiveConfig struct {
	mu      sync.RWMutex
	Current config.DynamicRuntimeConfig
}

// NewEffectiveConfig 创建生效配置容器。
func NewEffectiveConfig(cfg config.DynamicRuntimeConfig) *EffectiveConfig {
	return &EffectiveConfig{Current: cfg}
}

// Replace 替换生效配置。
func (c *EffectiveConfig) Replace(cfg config.DynamicRuntimeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Current = cfg
}

// Snapshot 返回当前生效配置快照。
func (c *EffectiveConfig) Snapshot() config.DynamicRuntimeConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Current
}
