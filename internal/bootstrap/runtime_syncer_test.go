package bootstrap

import (
	"testing"

	"hvc/internal/config"
)

func TestLoadPublishedDynamicRuntimeConfigAppliesNodeModeConstraintsInSnapshotPath(t *testing.T) {
	cfg := config.DefaultDynamicRuntimeConfig()
	cfg.Mode.EnableHTTPServer = true
	cfg.Mode.EnableGRPCServer = true
	cfg.Mode.EnableMQConsumer = true
	cfg.Mode.EnableCallback = true
	cfg.Mode.EnableScheduler = false
	cfg.Mode.EnableWorker = false

	control := config.ApplyNodeModeRuntimeConstraints(config.NodeModeClusterControl, cfg)
	if !control.Mode.EnableScheduler || control.Mode.EnableWorker {
		t.Fatalf("unexpected control node runtime snapshot: %+v", control.Mode)
	}
	if !control.Mode.EnableHTTPServer || !control.Mode.EnableGRPCServer || !control.Mode.EnableMQConsumer || !control.Mode.EnableCallback {
		t.Fatalf("control node should preserve runtime-managed northbound modules: %+v", control.Mode)
	}

	worker := config.ApplyNodeModeRuntimeConstraints(config.NodeModeClusterWorker, cfg)
	if worker.Mode.EnableScheduler || !worker.Mode.EnableWorker {
		t.Fatalf("unexpected worker node runtime snapshot: %+v", worker.Mode)
	}
	if worker.Mode.EnableHTTPServer || worker.Mode.EnableGRPCServer || worker.Mode.EnableMQConsumer || worker.Mode.EnableCallback {
		t.Fatalf("worker node should hard-disable control-plane modules: %+v", worker.Mode)
	}
}

func TestShouldRunHTTPServer(t *testing.T) {
	cfg := config.DefaultDynamicRuntimeConfig()
	cfg.Mode.EnableHTTPServer = false

	if shouldRunHTTPServer(config.NodeModeStandalone, cfg) {
		t.Fatal("standalone node should respect runtime http switch")
	}
	if !shouldRunHTTPServer(config.NodeModeClusterWorker, cfg) {
		t.Fatal("cluster-worker should still run internal-only http server")
	}

	cfg.Mode.EnableHTTPServer = true
	if !shouldRunHTTPServer(config.NodeModeClusterControl, cfg) {
		t.Fatal("control node should run http server when runtime switch is enabled")
	}
}
