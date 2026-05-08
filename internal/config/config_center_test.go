package config

import (
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
