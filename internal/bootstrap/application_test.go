package bootstrap

import (
	"context"
	"os/exec"
	"testing"
)

func TestRuntimeConfigLoad(t *testing.T) {
	cfg, err := testRuntimeConfig()
	if err != nil {
		t.Fatalf("load test config failed: %v", err)
	}
	if cfg.Server.ListenAddress == "" {
		t.Fatalf("listen address should not be empty")
	}
}

func TestFFprobeExists(t *testing.T) {
	_, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not found")
	}
}

func TestApplicationRunCancelledContext(t *testing.T) {
	cfg, err := testRuntimeConfig()
	if err != nil {
		t.Fatalf("load test config failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app, err := NewApplication(cfg)
	if err != nil {
		t.Skipf("skip bootstrap because external dependency is unavailable: %v", err)
	}
	if err := app.Run(ctx); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}
