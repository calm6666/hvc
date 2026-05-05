package callback

import (
	"context"

	"hvc/internal/model"
)

// BuildPayload 构造回调负载。
func BuildPayload(ctx context.Context, event model.OutboxEvent) map[string]any {
	_ = ctx
	return map[string]any{
		"event_id":    event.EventID,
		"event_type":  event.EventType,
		"job_id":      event.JobID,
		"request_id":  event.RequestID,
		"payload_json": event.PayloadJSON,
	}
}
