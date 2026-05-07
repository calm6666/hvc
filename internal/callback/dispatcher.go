package callback

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/pkg/logx"
)

// Dispatcher 表示回调投递模块。
type Dispatcher struct {
	cfg                    config.DynamicRuntimeConfig
	outboxRepository       *mysql.OutboxRepository
	callbackConfigRepo     *mysql.CallbackConfigRepository
	httpClient             *http.Client
}

// NewDispatcher 创建回调投递模块。
func NewDispatcher(cfg config.DynamicRuntimeConfig, outboxRepository *mysql.OutboxRepository, callbackConfigRepo *mysql.CallbackConfigRepository) *Dispatcher {
	return &Dispatcher{
		cfg:                cfg,
		outboxRepository:   outboxRepository,
		callbackConfigRepo: callbackConfigRepo,
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
	targetURL := d.cfg.Callback.HTTPURL
	if d.callbackConfigRepo != nil {
		configs := d.callbackConfigRepo.ListEnabled(ctx)
		for _, item := range configs {
			if item.CallbackType == 1 && item.TargetURL != "" {
				targetURL = item.TargetURL
				break
			}
		}
	}
	if targetURL == "" {
		logx.Error("callback.dispatch.no_url_configured", nil, logx.Fields{
			"event_payload": payload[:min(len(payload), 200)],
		})
		return fmt.Errorf("回调 URL 未配置，请在后台配置回调地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBufferString(payload))
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
