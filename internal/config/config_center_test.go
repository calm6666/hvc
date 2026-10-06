package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDynamicRuntimeConfig(t *testing.T) {
	cfg := DefaultDynamicRuntimeConfig()
	if !cfg.Mode.EnableHTTPServer {
		t.Fatal("expected built-in runtime defaults to enable http")
	}
	if cfg.Mode.EnableGRPCServer {
		t.Fatal("expected built-in runtime defaults to keep public grpc disabled")
	}
	if cfg.Mode.EnableMQConsumer {
		t.Fatal("expected mq consumer to stay disabled by default")
	}
	if cfg.Worker.SegmentTemplate == "" || cfg.Callback.HTTPTimeout <= 0 {
		t.Fatalf("unexpected built-in defaults: %+v", cfg)
	}
}

func TestLoadBootstrapDynamicRuntimeConfigIgnoresDeprecatedBootstrapDynamicSections(t *testing.T) {
	cfg := LoadBootstrapDynamicRuntimeConfig(RuntimeConfig{
		MQ: MQConfig{
			RabbitMQHost:  "127.0.0.1",
			RabbitMQPort:  5672,
			RabbitMQUser:  "guest",
			QueueName:     "hvc.transcode.create",
			ConsumerTag:   "runtime-consumer",
			PrefetchCount: 8,
		},
		GRPC: GRPCConfig{
			ListenAddress: ":29090",
		},
		Storage: StorageConfig{
			StorageType:   "s3",
			Endpoint:      "127.0.0.1:9000",
			Bucket:        "hvc-media",
			BasePrefix:    "legacy-prefix",
			LocalBasePath: "/tmp/hvc",
		},
		Worker: WorkerConfig{
			LoopInterval:         5 * time.Second,
			UploadMaxRetryCount:  9,
			SegmentTemplate:      "{job_id}-{number}.m4s",
			SourceReadTimeout:    45 * time.Minute,
			UploadRetryBaseDelay: 3 * time.Second,
		},
	})

	if cfg.Mode.EnableMQConsumer {
		t.Fatal("bootstrap runtime config should no longer enable mq consumer from YAML legacy fields")
	}
	if cfg.MQ.Host != "127.0.0.1" || cfg.MQ.QueueName != "" {
		t.Fatalf("unexpected built-in mq defaults: %+v", cfg.MQ)
	}
	if cfg.GRPC.ListenAddress != ":9090" {
		t.Fatalf("unexpected built-in grpc defaults: %+v", cfg.GRPC)
	}
	if cfg.Storage.BasePrefix != "hvc" || cfg.Storage.StorageType != "local" {
		t.Fatalf("unexpected built-in storage defaults: %+v", cfg.Storage)
	}
	if cfg.Worker.UploadMaxRetryCount != 3 {
		t.Fatalf("unexpected built-in worker defaults: %+v", cfg.Worker)
	}
}

func TestApplyNodeModeRuntimeConstraints(t *testing.T) {
	base := DefaultDynamicRuntimeConfig()
	base.Mode.EnableHTTPServer = true
	base.Mode.EnableGRPCServer = true
	base.Mode.EnableMQConsumer = true
	base.Mode.EnableCallback = true
	base.Mode.EnableScheduler = false
	base.Mode.EnableWorker = false

	control := ApplyNodeModeRuntimeConstraints(NodeModeClusterControl, base)
	if !control.Mode.EnableScheduler || control.Mode.EnableWorker {
		t.Fatalf("unexpected control-plane constraints: %+v", control.Mode)
	}
	if !control.Mode.EnableHTTPServer || !control.Mode.EnableGRPCServer || !control.Mode.EnableMQConsumer || !control.Mode.EnableCallback {
		t.Fatalf("control-plane should preserve runtime-managed northbound modules: %+v", control.Mode)
	}

	worker := ApplyNodeModeRuntimeConstraints(NodeModeClusterWorker, base)
	if worker.Mode.EnableScheduler || !worker.Mode.EnableWorker {
		t.Fatalf("unexpected worker-plane constraints: %+v", worker.Mode)
	}
	if worker.Mode.EnableHTTPServer || worker.Mode.EnableGRPCServer || worker.Mode.EnableMQConsumer || worker.Mode.EnableCallback {
		t.Fatalf("worker-plane should hard-disable control-plane modules: %+v", worker.Mode)
	}

	allInOne := ApplyNodeModeRuntimeConstraints(NodeModeClusterAllInOne, base)
	if !allInOne.Mode.EnableScheduler || !allInOne.Mode.EnableWorker {
		t.Fatalf("unexpected all-in-one constraints: %+v", allInOne.Mode)
	}
	if !allInOne.Mode.EnableHTTPServer || !allInOne.Mode.EnableGRPCServer || !allInOne.Mode.EnableMQConsumer || !allInOne.Mode.EnableCallback {
		t.Fatalf("all-in-one should preserve runtime-managed northbound modules: %+v", allInOne.Mode)
	}
}

func TestValidateRuntimeConfigRejectsDeprecatedDynamicSections(t *testing.T) {
	cfg := RuntimeConfig{
		Server: ServerConfig{
			ListenAddress: ":8080",
		},
		MySQL: MySQLConfig{
			DSN: "root:pass@tcp(127.0.0.1:3306)/hvc",
		},
		Redis: RedisConfig{
			Addrs: []string{"127.0.0.1:6379"},
		},
		Worker: WorkerConfig{
			LoopInterval: 5 * time.Second,
		},
	}

	if err := ValidateRuntimeConfig(cfg); err == nil {
		t.Fatal("expected deprecated dynamic bootstrap section to be rejected")
	}
}

func TestResolveRuntimeConfigPath(t *testing.T) {
	t.Setenv("HVC_CONFIG_PATH", filepath.Join("testdata", "env-config.yaml"))

	if got := ResolveRuntimeConfigPath([]string{"--config", "custom.yaml"}); got != "custom.yaml" {
		t.Fatalf("expected cli path, got %q", got)
	}
	if got := ResolveRuntimeConfigPath([]string{"--config=inline.yaml"}); got != "inline.yaml" {
		t.Fatalf("expected inline cli path, got %q", got)
	}
	if got := ResolveRuntimeConfigPath(nil); got != filepath.Join("testdata", "env-config.yaml") {
		t.Fatalf("expected env path, got %q", got)
	}

	_ = os.Unsetenv("HVC_CONFIG_PATH")
	if got := ResolveRuntimeConfigPath(nil); got != defaultRuntimeConfigPath {
		t.Fatalf("expected default path, got %q", got)
	}
}
