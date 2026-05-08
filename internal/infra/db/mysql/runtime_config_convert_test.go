package mysql

import (
	"testing"
	"time"

	"hvc/internal/config"
	"hvc/pkg/idgen"
)

func TestToDynamicRuntimeConfigUsesPublicGRPCFields(t *testing.T) {
	record := RuntimeConfigRecord{
		EnableHTTPServer: true,
		EnableCallback:   true,
		PublicGRPCEnabled: true,
		PublicGRPCHost:   "127.0.0.1",
		PublicGRPCPort:   19090,
		EnableMQConsumer: true,
		MQQueueName:      "hvc.transcode.create",
		MQHost:           "127.0.0.1",
		MQPort:           5672,
		MQConsumerTag:    "runtime",
		MQVHost:          "/",
		MQLoopIntervalMS: 15000,
		StorageType:      "s3",
		StorageEndpoint:  "127.0.0.1:9000",
		StorageBucket:    "hvc-media",
		WorkerObjectPrefix: "runtime-prefix",
	}

	cfg := ToDynamicRuntimeConfig(record)
	if !cfg.Mode.EnableGRPCServer {
		t.Fatal("expected public grpc to be enabled")
	}
	if cfg.GRPC.ListenAddress != "127.0.0.1:19090" {
		t.Fatalf("unexpected public grpc listen address: %s", cfg.GRPC.ListenAddress)
	}
	if !cfg.Mode.EnableMQConsumer {
		t.Fatal("expected mq consumer mode to come from explicit runtime field")
	}
	if cfg.Storage.Endpoint != "127.0.0.1:9000" || cfg.Storage.BasePrefix != "runtime-prefix" {
		t.Fatalf("unexpected storage config snapshot: %+v", cfg.Storage)
	}
}

func TestToDynamicRuntimeConfigFallsBackToLegacyReceiverFields(t *testing.T) {
	record := RuntimeConfigRecord{
		EnableHTTPServer:           true,
		EnableCallback:             true,
		RPCCallbackReceiverEnabled: true,
		RPCCallbackReceiverHost:    "0.0.0.0",
		RPCCallbackReceiverPort:    29090,
		MQLoopIntervalMS:           int((15 * time.Second) / time.Millisecond),
	}

	cfg := ToDynamicRuntimeConfig(record)
	if !cfg.Mode.EnableGRPCServer {
		t.Fatal("expected legacy grpc receiver fields to keep compatibility")
	}
	if cfg.GRPC.ListenAddress != "0.0.0.0:29090" {
		t.Fatalf("unexpected legacy fallback listen address: %s", cfg.GRPC.ListenAddress)
	}
}

func TestNewBootstrapRuntimeConfigRecordPersistsMQAndStorage(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)

	cfg := config.DefaultDynamicRuntimeConfig()
	cfg.Mode.EnableMQConsumer = true
	cfg.MQ.QueueName = "hvc.transcode.create"
	cfg.Storage.StorageType = "s3"
	cfg.Storage.Endpoint = "127.0.0.1:9000"
	cfg.Storage.Bucket = "hvc-media"
	cfg.Storage.BasePrefix = "runtime-prefix"

	record := NewBootstrapRuntimeConfigRecord(cfg, "bootstrap_local")
	if !record.EnableMQConsumer {
		t.Fatal("expected bootstrap record to persist mq enable flag")
	}
	if record.StorageEndpoint != "127.0.0.1:9000" || record.StorageBucket != "hvc-media" {
		t.Fatalf("unexpected storage persistence: %+v", record)
	}
	if record.WorkerObjectPrefix != "runtime-prefix" {
		t.Fatalf("unexpected storage base prefix persistence: %+v", record)
	}
}
