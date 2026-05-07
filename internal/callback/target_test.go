package callback

import "testing"

func TestParseTaskCallbackTarget(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantOK    bool
		protocol  string
		targetURL string
		endpoint  string
		method    string
		exchange  string
		routing   string
	}{
		{
			name:      "http",
			raw:       "https://callback.example.com/job/1",
			wantOK:    true,
			protocol:  "http",
			targetURL: "https://callback.example.com/job/1",
		},
		{
			name:     "grpc path",
			raw:      "grpc://127.0.0.1:9000/transcode.callback.Service/Notify",
			wantOK:   true,
			protocol: "grpc",
			endpoint: "127.0.0.1:9000",
			method:   "/transcode.callback.Service/Notify",
		},
		{
			name:     "grpc query",
			raw:      "grpc://127.0.0.1:9000?method=/transcode.callback.Service/Notify",
			wantOK:   true,
			protocol: "grpc",
			endpoint: "127.0.0.1:9000",
			method:   "/transcode.callback.Service/Notify",
		},
		{
			name:     "mq host path",
			raw:      "mq://callback.exchange/transcode.job.completed",
			wantOK:   true,
			protocol: "mq",
			exchange: "callback.exchange",
			routing:  "transcode.job.completed",
		},
		{
			name:     "mq query",
			raw:      "mq:///ignored?routing_key=transcode.job.completed&exchange=callback.exchange",
			wantOK:   true,
			protocol: "mq",
			exchange: "callback.exchange",
			routing:  "transcode.job.completed",
		},
		{
			name:   "invalid scheme",
			raw:    "redis://127.0.0.1:6379/topic",
			wantOK: false,
		},
		{
			name:   "invalid grpc",
			raw:    "grpc://127.0.0.1:9000",
			wantOK: false,
		},
		{
			name:   "invalid mq",
			raw:    "mq://callback.exchange",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		got, ok := ParseTaskCallbackTarget(tt.raw)
		if ok != tt.wantOK {
			t.Fatalf("%s: expected ok=%v, got %v", tt.name, tt.wantOK, ok)
		}
		if !ok {
			continue
		}
		if got.Protocol != tt.protocol {
			t.Fatalf("%s: unexpected protocol %s", tt.name, got.Protocol)
		}
		if got.TargetURL != tt.targetURL {
			t.Fatalf("%s: unexpected target url %s", tt.name, got.TargetURL)
		}
		if got.RPCEndpoint != tt.endpoint {
			t.Fatalf("%s: unexpected endpoint %s", tt.name, got.RPCEndpoint)
		}
		if got.RPCMethod != tt.method {
			t.Fatalf("%s: unexpected method %s", tt.name, got.RPCMethod)
		}
		if got.MQExchange != tt.exchange {
			t.Fatalf("%s: unexpected exchange %s", tt.name, got.MQExchange)
		}
		if got.MQRoutingKey != tt.routing {
			t.Fatalf("%s: unexpected routing key %s", tt.name, got.MQRoutingKey)
		}
	}
}
