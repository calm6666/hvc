package config

// IsStandalone 返回是否启用单机模式。
func (cfg DynamicRuntimeConfig) IsStandalone() bool {
	return cfg.Mode.EnableHTTPServer && cfg.Mode.EnableScheduler && cfg.Mode.EnableWorker && cfg.Mode.EnableCallback
}

// IsClusterControl 返回是否启用集群控制节点模式。
func (cfg DynamicRuntimeConfig) IsClusterControl() bool {
	return cfg.Mode.EnableHTTPServer && cfg.Mode.EnableScheduler && !cfg.Mode.EnableWorker
}

// IsClusterWorker 返回是否启用集群执行节点模式。
func (cfg DynamicRuntimeConfig) IsClusterWorker() bool {
	return !cfg.Mode.EnableHTTPServer && !cfg.Mode.EnableScheduler && cfg.Mode.EnableWorker
}

// IsClusterAllInOne 返回是否启用集群一体化模式。
func (cfg DynamicRuntimeConfig) IsClusterAllInOne() bool {
	return cfg.Mode.EnableHTTPServer && cfg.Mode.EnableScheduler && cfg.Mode.EnableWorker
}
