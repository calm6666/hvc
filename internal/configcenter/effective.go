package configcenter

import "hvc/internal/config"

// EffectiveConfig 表示当前生效配置。
type EffectiveConfig struct {
	Current config.RuntimeConfig
}

// NewEffectiveConfig 创建生效配置容器。
func NewEffectiveConfig(cfg config.RuntimeConfig) *EffectiveConfig {
	return &EffectiveConfig{Current: cfg}
}

// Replace 替换生效配置。
func (c *EffectiveConfig) Replace(cfg config.RuntimeConfig) {
	c.Current = cfg
}
