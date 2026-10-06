package callback

import (
	"context"
	"testing"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
)

func TestResolveTargetsUsesHTTPJobOverrideCallbackURL(t *testing.T) {
	dispatcher := &Dispatcher{}
	override := &model.TranscodeJobRequestOverride{
		OverrideCallbackURL: "https://callback.example.com/job/1",
	}

	targets := dispatcher.resolveTargetsWithOverride(context.Background(), config.DynamicRuntimeConfig{}, model.OutboxEvent{JobID: 1}, override)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].CallbackType != callbackTypeHTTP {
		t.Fatalf("expected callback type %d, got %d", callbackTypeHTTP, targets[0].CallbackType)
	}
	if targets[0].TargetURL != "https://callback.example.com/job/1" {
		t.Fatalf("unexpected callback target: %s", targets[0].TargetURL)
	}
}

func TestResolveTargetsUsesGRPCJobOverrideCallbackURL(t *testing.T) {
	dispatcher := &Dispatcher{}
	override := &model.TranscodeJobRequestOverride{
		OverrideCallbackURL: "grpc://127.0.0.1:9000/transcode.callback.Service/Notify",
	}

	targets := dispatcher.resolveTargetsWithOverride(context.Background(), config.DynamicRuntimeConfig{}, model.OutboxEvent{JobID: 1}, override)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].CallbackType != callbackTypeGRPC {
		t.Fatalf("expected callback type %d, got %d", callbackTypeGRPC, targets[0].CallbackType)
	}
	if targets[0].RPCEndpoint != "127.0.0.1:9000" {
		t.Fatalf("unexpected grpc endpoint: %s", targets[0].RPCEndpoint)
	}
	if targets[0].RPCServiceName != "/transcode.callback.Service/Notify" {
		t.Fatalf("unexpected grpc method: %s", targets[0].RPCServiceName)
	}
}

func TestResolveTargetsUsesMQJobOverrideCallbackURL(t *testing.T) {
	dispatcher := &Dispatcher{}
	override := &model.TranscodeJobRequestOverride{
		OverrideCallbackURL: "mq://callback.exchange/transcode.job.completed",
	}

	targets := dispatcher.resolveTargetsWithOverride(context.Background(), config.DynamicRuntimeConfig{}, model.OutboxEvent{JobID: 1}, override)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].CallbackType != callbackTypeMQ {
		t.Fatalf("expected callback type %d, got %d", callbackTypeMQ, targets[0].CallbackType)
	}
	if targets[0].MQExchange != "callback.exchange" {
		t.Fatalf("unexpected mq exchange: %s", targets[0].MQExchange)
	}
	if targets[0].MQRoutingKey != "transcode.job.completed" {
		t.Fatalf("unexpected mq routing key: %s", targets[0].MQRoutingKey)
	}
}

func TestResolveTargetsFallsBackToRuntimeHTTPURL(t *testing.T) {
	dispatcher := &Dispatcher{}
	cfg := config.DynamicRuntimeConfig{
		Callback: config.CallbackConfig{
			HTTPURL: "https://callback.example.com/runtime",
		},
	}

	targets := dispatcher.resolveTargetsWithOverride(context.Background(), cfg, model.OutboxEvent{JobID: 2}, nil)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].TargetURL != "https://callback.example.com/runtime" {
		t.Fatalf("unexpected fallback callback target: %s", targets[0].TargetURL)
	}
}

func TestResolveTargetsFallsBackToRuntimeMQTopic(t *testing.T) {
	dispatcher := &Dispatcher{}
	cfg := config.DynamicRuntimeConfig{
		MQ: config.MQRuntimeConfig{
			CallbackTopic: "transcode.job.completed",
		},
	}

	targets := dispatcher.resolveTargetsWithOverride(context.Background(), cfg, model.OutboxEvent{JobID: 3}, nil)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].CallbackType != callbackTypeMQ {
		t.Fatalf("expected callback type %d, got %d", callbackTypeMQ, targets[0].CallbackType)
	}
	if targets[0].MQRoutingKey != "transcode.job.completed" {
		t.Fatalf("unexpected fallback mq routing key: %s", targets[0].MQRoutingKey)
	}
}

func TestDefaultTimeoutMS(t *testing.T) {
	cfg := config.CallbackConfig{
		HTTPTimeout:  4 * time.Millisecond,
		GRPCTimeout:  5 * time.Millisecond,
		MQTimeout:    6 * time.Millisecond,
		RetryBackoff: 7 * time.Millisecond,
	}

	if got := defaultTimeoutMS(callbackTypeHTTP, cfg); got != 4 {
		t.Fatalf("unexpected http timeout: %d", got)
	}
	if got := defaultTimeoutMS(callbackTypeGRPC, cfg); got != 5 {
		t.Fatalf("unexpected grpc timeout: %d", got)
	}
	if got := defaultTimeoutMS(callbackTypeMQ, cfg); got != 6 {
		t.Fatalf("unexpected mq timeout: %d", got)
	}
}

func TestSendingLeaseTimeout(t *testing.T) {
	cfg := config.DynamicRuntimeConfig{
		Callback: config.CallbackConfig{
			HTTPTimeout: 8 * time.Second,
			GRPCTimeout: 3 * time.Second,
			MQTimeout:   5 * time.Second,
		},
	}
	if got := sendingLeaseTimeout(cfg); got != 30*time.Second {
		t.Fatalf("expected minimum sending lease timeout 30s, got %s", got)
	}

	cfg.Callback.HTTPTimeout = 20 * time.Second
	if got := sendingLeaseTimeout(cfg); got != 40*time.Second {
		t.Fatalf("expected doubled sending lease timeout, got %s", got)
	}
}

func TestCallbackTargetString(t *testing.T) {
	tests := []struct {
		name   string
		target mysql.CallbackConfigRecord
		want   string
	}{
		{
			name: "http",
			target: mysql.CallbackConfigRecord{
				CallbackType: callbackTypeHTTP,
				TargetURL:    "https://callback.example.com/job/1",
			},
			want: "https://callback.example.com/job/1",
		},
		{
			name: "grpc",
			target: mysql.CallbackConfigRecord{
				CallbackType:   callbackTypeGRPC,
				RPCEndpoint:    "127.0.0.1:9000",
				RPCServiceName: "/transcode.callback.Service/Notify",
			},
			want: "grpc://127.0.0.1:9000/transcode.callback.Service/Notify",
		},
		{
			name: "mq",
			target: mysql.CallbackConfigRecord{
				CallbackType: callbackTypeMQ,
				MQExchange:   "callback.exchange",
				MQRoutingKey: "transcode.job.completed",
			},
			want: "mq://callback.exchange/transcode.job.completed",
		},
	}

	for _, tt := range tests {
		if got := callbackTargetString(tt.target); got != tt.want {
			t.Fatalf("%s: expected %s, got %s", tt.name, tt.want, got)
		}
	}
}

func TestGRPCServiceNameFromMethod(t *testing.T) {
	if got := grpcServiceNameFromMethod("/transcode.callback.Service/Notify"); got != "transcode.callback.Service" {
		t.Fatalf("unexpected service name: %s", got)
	}
	if got := grpcServiceNameFromMethod("transcode.callback.Service/Notify"); got != "transcode.callback.Service" {
		t.Fatalf("unexpected service name without leading slash: %s", got)
	}
}

func TestRegistryServicePrefix(t *testing.T) {
	if got := registryServicePrefix("hvc/public", "transcode.callback.Service"); got != "/hvc/public/transcode.callback.Service/" {
		t.Fatalf("unexpected registry prefix: %s", got)
	}
}
