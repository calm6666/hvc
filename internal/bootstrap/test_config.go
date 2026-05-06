package bootstrap

import (
	"path/filepath"
	"time"

	"hvc/internal/config"
	"hvc/pkg/idgen"
)

func testRuntimeConfig() (config.RuntimeConfig, error) {
	cfg, err := config.LoadRuntimeConfigFromYAML(filepath.Clean("../../configs/config.yaml"))
	if err != nil {
		return config.RuntimeConfig{}, err
	}
	cfg.ConfigCenter.Enabled = false
	startTime, err := time.Parse(time.RFC3339, cfg.ID.StartTime)
	if err != nil {
		return config.RuntimeConfig{}, err
	}
	idgen.Configure(startTime, cfg.Server.NodeID, cfg.ID.NodeBits, cfg.ID.SequenceBits)
	return cfg, nil
}
