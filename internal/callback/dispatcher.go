package callback

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/pkg/logx"
)

// Dispatcher 表示回调投递模块。
type Dispatcher struct {
	cfg              config.RuntimeConfig
	outboxRepository *mysql.OutboxRepository
	httpClient       *http.Client
}

// NewDispatcher 创建回调投递模块。
func NewDispatcher(cfg config.RuntimeConfig, outboxRepository *mysql.OutboxRepository) *Dispatcher {
	return &Dispatcher{
		cfg:              cfg,
		outboxRepository: outboxRepository,
		httpClient: &http.Client{
			Timeout: cfg.Callback.HTTPTimeout,
		},
	}
}

// Start 启动回调投递模块。
func (d *Dispatcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pending := d.outboxRepository.ListPending(ctx)
			for _, event := range pending {
				logx.Info("callback.dispatch.start", logx.Fields{
					"event_id":    event.EventID,
					"event_type":  event.EventType,
					"job_id":      event.JobID,
					"request_id":  event.RequestID,
					"retry_count": event.RetryCount,
				})
				if err := d.dispatchHTTP(ctx, event.PayloadJSON); err != nil {
					_ = d.outboxRepository.MarkFailed(ctx, event.EventID, err.Error())
					logx.Error("callback.dispatch.failed", err, logx.Fields{
						"event_id":   event.EventID,
						"event_type": event.EventType,
					})
					continue
				}
				_ = d.outboxRepository.MarkDelivered(ctx, event.EventID)
				logx.Info("callback.dispatch.success", logx.Fields{
					"event_id":   event.EventID,
					"event_type": event.EventType,
					"job_id":     event.JobID,
				})
			}
		}
	}
}

func (d *Dispatcher) dispatchHTTP(ctx context.Context, payload string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:18080/callback", bytes.NewBufferString(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return http.ErrHandlerTimeout
	}
	return nil
}
