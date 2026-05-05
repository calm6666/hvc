package bootstrap

import "hvc/internal/config"

func testRuntimeConfig() (config.RuntimeConfig, error) {
	cfg := config.DefaultRuntimeConfig()
	cfg.ConfigCenter.Enabled = false
	return cfg, nil
}
