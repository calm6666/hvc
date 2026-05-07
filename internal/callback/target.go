package callback

import (
	"net/url"
	"strings"
)

// TaskCallbackTarget 表示任务级 callback_url 解析结果。
type TaskCallbackTarget struct {
	Protocol     string
	TargetURL    string
	RPCEndpoint  string
	RPCMethod    string
	MQExchange   string
	MQRoutingKey string
}

// ParseTaskCallbackTarget 解析任务级 callback_url。
func ParseTaskCallbackTarget(raw string) (TaskCallbackTarget, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return TaskCallbackTarget{}, false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return TaskCallbackTarget{}, false
	}

	switch strings.ToLower(parsed.Scheme) {
	case "", "http", "https":
		return TaskCallbackTarget{
			Protocol:  "http",
			TargetURL: raw,
		}, true
	case "grpc":
		method := strings.TrimSpace(parsed.Query().Get("method"))
		if method == "" {
			method = strings.TrimSpace(parsed.EscapedPath())
			if method == "" {
				method = strings.TrimSpace(parsed.Path)
			}
		}
		if parsed.Host == "" || method == "" {
			return TaskCallbackTarget{}, false
		}
		return TaskCallbackTarget{
			Protocol:    "grpc",
			RPCEndpoint: parsed.Host,
			RPCMethod:   method,
		}, true
	case "mq":
		exchange := strings.TrimSpace(parsed.Query().Get("exchange"))
		routingKey := strings.TrimSpace(parsed.Query().Get("routing_key"))
		if exchange == "" {
			exchange = strings.TrimSpace(parsed.Host)
		}
		if routingKey == "" {
			routingKey = strings.Trim(strings.TrimSpace(parsed.Path), "/")
		}
		if routingKey == "" {
			return TaskCallbackTarget{}, false
		}
		return TaskCallbackTarget{
			Protocol:     "mq",
			MQExchange:   exchange,
			MQRoutingKey: routingKey,
		}, true
	default:
		return TaskCallbackTarget{}, false
	}
}
