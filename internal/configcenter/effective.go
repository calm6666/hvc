package configcenter

import (
	"sync"

	"hvc/internal/config"
)

// EffectiveConfig 表示当前生效配置。
type EffectiveConfig struct {
	mu      sync.RWMutex
	Current config.DynamicRuntimeConfig
	Version uint64
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

// ReplaceWithVersion 替换当前生效配置，并同步记录对应的配置版本号。
func (c *EffectiveConfig) ReplaceWithVersion(cfg config.DynamicRuntimeConfig, version uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Current = cfg
	c.Version = version
}

// Snapshot 返回当前生效配置快照。
func (c *EffectiveConfig) Snapshot() config.DynamicRuntimeConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Current
}

// CurrentVersion 返回当前进程内生效配置的版本号。
func (c *EffectiveConfig) CurrentVersion() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Version
}
