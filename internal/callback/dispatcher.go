package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/infra/mq/rabbitmq"
	mqproducer "hvc/internal/interfaces/mq/producer"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
	"hvc/pkg/retryx"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	callbackTypeHTTP = 1
	callbackTypeGRPC = 2
	callbackTypeMQ   = 3
)

type dispatchFailure struct {
	target mysql.CallbackConfigRecord
	stage  string
	err    error
}

func (e *dispatchFailure) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Dispatcher 表示回调投递模块。
type Dispatcher struct {
	effectiveConfig    *configcenter.EffectiveConfig
	outboxRepository   *mysql.OutboxRepository
	callbackConfigRepo *mysql.CallbackConfigRepository
	jobOverrideRepo    *mysql.JobRequestOverrideRepository
	failureQueueRepo   *mysql.DeliveryFailureQueueRepository
}

// NewDispatcher 创建回调投递模块。
func NewDispatcher(
	effectiveConfig *configcenter.EffectiveConfig,
	outboxRepository *mysql.OutboxRepository,
	callbackConfigRepo *mysql.CallbackConfigRepository,
	jobOverrideRepo *mysql.JobRequestOverrideRepository,
	failureQueueRepo *mysql.DeliveryFailureQueueRepository,
) *Dispatcher {
	return &Dispatcher{
		effectiveConfig:    effectiveConfig,
		outboxRepository:   outboxRepository,
		callbackConfigRepo: callbackConfigRepo,
		jobOverrideRepo:    jobOverrideRepo,
		failureQueueRepo:   failureQueueRepo,
	}
}

// Start 启动回调投递模块。
func (d *Dispatcher) Start(ctx context.Context) error {
	for {
		cfg := d.currentConfig()
		if cfg.Mode.EnableCallback {
			d.dispatchOnce(ctx, cfg)
		}

		wait := cfg.Callback.RetryBackoff
		if wait <= 0 {
			wait = 2 * time.Second
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (d *Dispatcher) dispatchOnce(ctx context.Context, cfg config.DynamicRuntimeConfig) {
	events := append(d.outboxRepository.ListPending(ctx), d.outboxRepository.ListRetryable(ctx)...)
	for _, event := range events {
		logx.Info("callback.dispatch.start", logx.Fields{
			"event_id":    event.EventID,
			"event_type":  event.EventType,
			"job_id":      event.JobID,
			"request_id":  event.RequestID,
			"retry_count": event.RetryCount,
		})
		if err := d.dispatchEvent(ctx, cfg, event); err != nil {
			d.handleFailure(ctx, cfg, event, err)
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

func (d *Dispatcher) dispatchEvent(ctx context.Context, cfg config.DynamicRuntimeConfig, event model.OutboxEvent) error {
	targets := d.resolveTargets(ctx, cfg, event)
	if len(targets) == 0 {
		return fmt.Errorf("no callback target configured")
	}

	var lastErr error
	for _, target := range targets {
		if err := d.dispatchToTarget(ctx, cfg, target, event.PayloadJSON); err != nil {
			lastErr = &dispatchFailure{
				target: target,
				stage:  dispatchStage(target.CallbackType),
				err:    err,
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (d *Dispatcher) dispatchToTarget(ctx context.Context, cfg config.DynamicRuntimeConfig, target mysql.CallbackConfigRecord, payload string) error {
	timeoutMS := target.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = defaultTimeoutMS(target.CallbackType, cfg.Callback)
	}

	switch target.CallbackType {
	case callbackTypeHTTP:
		targetURL := target.TargetURL
		if targetURL == "" {
			targetURL = cfg.Callback.HTTPURL
		}
		return d.dispatchHTTP(ctx, targetURL, timeoutMS, payload)
	case callbackTypeGRPC:
		return d.dispatchGRPC(ctx, target.RPCEndpoint, target.RPCServiceName, timeoutMS, payload)
	case callbackTypeMQ:
		return d.dispatchMQ(ctx, cfg.MQ, target.MQExchange, target.MQRoutingKey, timeoutMS, payload)
	default:
		return fmt.Errorf("unsupported callback type %d", target.CallbackType)
	}
}

func (d *Dispatcher) dispatchHTTP(ctx context.Context, targetURL string, timeoutMS int, payload string) error {
	if targetURL == "" {
		return fmt.Errorf("callback http target is empty")
	}

	timeout := 3 * time.Second
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBufferString(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("callback http status %d", resp.StatusCode)
	}
	return nil
}

func (d *Dispatcher) dispatchGRPC(ctx context.Context, endpoint string, method string, timeoutMS int, payload string) error {
	if endpoint == "" {
		return fmt.Errorf("callback grpc endpoint is empty")
	}
	if method == "" {
		return fmt.Errorf("callback grpc method is empty")
	}

	callCtx := ctx
	if timeoutMS > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
		defer cancel()
	}

	conn, err := grpc.DialContext(callCtx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()

	requestBody := map[string]any{}
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &requestBody); err != nil {
			return err
		}
	}
	req, err := structpb.NewStruct(requestBody)
	if err != nil {
		return err
	}
	if method[0] != '/' {
		method = "/" + method
	}
	return conn.Invoke(callCtx, method, req, &emptypb.Empty{})
}

func (d *Dispatcher) dispatchMQ(ctx context.Context, mqCfg config.MQRuntimeConfig, exchange string, routingKey string, timeoutMS int, payload string) error {
	if routingKey == "" {
		routingKey = mqCfg.CallbackTopic
	}
	if routingKey == "" {
		return fmt.Errorf("callback mq routing key is empty")
	}

	publishCtx := ctx
	if timeoutMS > 0 {
		var cancel context.CancelFunc
		publishCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
		defer cancel()
	}

	conn, err := rabbitmq.OpenRuntime(mqCfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	publisher := mqproducer.NewPublisher(channel, exchange, routingKey)
	return publisher.PublishPayload(publishCtx, []byte(payload))
}

func (d *Dispatcher) handleFailure(ctx context.Context, cfg config.DynamicRuntimeConfig, event model.OutboxEvent, err error) {
	errMsg := err.Error()
	var nextRetryAt *time.Time
	finalFailed := event.RetryCount+1 >= event.MaxRetryCount
	if finalFailed {
		_ = d.outboxRepository.MarkFinalFailed(ctx, event.EventID, errMsg)
	} else {
		backoff := retryx.Backoff{
			BaseDelay: cfg.Callback.RetryBackoff,
			MaxDelay:  max(cfg.Callback.RetryBackoff*16, 2*time.Second),
		}.Duration(event.RetryCount)
		if backoff <= 0 {
			backoff = 2 * time.Second
		}
		value := time.Now().Add(backoff)
		nextRetryAt = &value
		_ = d.outboxRepository.MarkRetryable(ctx, event.EventID, errMsg, value)
	}

	if dispatchErr, ok := err.(*dispatchFailure); ok {
		d.recordFailure(ctx, event, dispatchErr, nextRetryAt)
	}

	if finalFailed {
		logx.Error("callback.dispatch.final_failed", err, logx.Fields{
			"event_id":   event.EventID,
			"event_type": event.EventType,
		})
		return
	}

	logx.Error("callback.dispatch.failed", err, logx.Fields{
		"event_id":      event.EventID,
		"event_type":    event.EventType,
		"next_retry_at": nextRetryAt,
	})
}

func (d *Dispatcher) recordFailure(ctx context.Context, event model.OutboxEvent, failed *dispatchFailure, nextRetryAt *time.Time) {
	if d.failureQueueRepo == nil || failed == nil {
		return
	}

	record := mysql.DeliveryFailureQueueRecord{
		FailureID:        idgen.Next(),
		EventID:          event.EventID,
		FailureStage:     failed.stage,
		FailureCode:      failureCode(failed.err),
		FailureMessage:   failed.Error(),
		CallbackConfigID: failed.target.CallbackConfigID,
		CallbackTarget:   callbackTargetString(failed.target),
		RetryCount:       event.RetryCount + 1,
		NextRetryAt:      nextRetryAt,
		Resolved:         false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	_ = d.failureQueueRepo.Save(ctx, record)
}

func (d *Dispatcher) resolveTargets(ctx context.Context, cfg config.DynamicRuntimeConfig, event model.OutboxEvent) []mysql.CallbackConfigRecord {
	return d.resolveTargetsWithOverride(ctx, cfg, event, d.findJobOverride(ctx, event.JobID))
}

func (d *Dispatcher) resolveTargetsWithOverride(ctx context.Context, cfg config.DynamicRuntimeConfig, event model.OutboxEvent, override *model.TranscodeJobRequestOverride) []mysql.CallbackConfigRecord {
	_ = event

	if override != nil {
		if target := strings.TrimSpace(override.OverrideCallbackURL); target != "" {
			if parsed, ok := parseTaskCallbackTarget(target); ok {
				return []mysql.CallbackConfigRecord{parsed}
			}
		}
	}

	if d.callbackConfigRepo != nil {
		configs := d.callbackConfigRepo.ListEnabled(ctx)
		if len(configs) > 0 {
			return configs
		}
	}

	// 兼容“未配置 callback config 表，但运行时默认配置已经下发”的兜底链路。
	// 优先走 HTTP 单地址；若未配置 HTTP，则允许直接回退到默认 MQ routing key。
	if cfg.Callback.HTTPURL != "" {
		return []mysql.CallbackConfigRecord{{
			CallbackType: callbackTypeHTTP,
			TargetURL:    cfg.Callback.HTTPURL,
		}}
	}
	if cfg.MQ.CallbackTopic != "" {
		return []mysql.CallbackConfigRecord{{
			CallbackType: callbackTypeMQ,
			MQRoutingKey: cfg.MQ.CallbackTopic,
		}}
	}
	return nil
}

func parseTaskCallbackTarget(raw string) (mysql.CallbackConfigRecord, bool) {
	target, ok := ParseTaskCallbackTarget(raw)
	if !ok {
		return mysql.CallbackConfigRecord{}, false
	}

	switch target.Protocol {
	case "http":
		return mysql.CallbackConfigRecord{
			CallbackType: callbackTypeHTTP,
			TargetURL:    target.TargetURL,
		}, true
	case "grpc":
		return mysql.CallbackConfigRecord{
			CallbackType:   callbackTypeGRPC,
			RPCEndpoint:    target.RPCEndpoint,
			RPCServiceName: target.RPCMethod,
		}, true
	case "mq":
		return mysql.CallbackConfigRecord{
			CallbackType: callbackTypeMQ,
			MQExchange:   target.MQExchange,
			MQRoutingKey: target.MQRoutingKey,
		}, true
	default:
		return mysql.CallbackConfigRecord{}, false
	}
}

func defaultTimeoutMS(callbackType int, cfg config.CallbackConfig) int {
	switch callbackType {
	case callbackTypeHTTP:
		return int(cfg.HTTPTimeout / time.Millisecond)
	case callbackTypeGRPC:
		return int(cfg.GRPCTimeout / time.Millisecond)
	case callbackTypeMQ:
		return int(cfg.MQTimeout / time.Millisecond)
	default:
		return 0
	}
}

func dispatchStage(callbackType int) string {
	switch callbackType {
	case callbackTypeHTTP:
		return "http.callback"
	case callbackTypeGRPC:
		return "grpc.callback"
	case callbackTypeMQ:
		return "mq.callback"
	default:
		return "callback.dispatch"
	}
}

func callbackTargetString(target mysql.CallbackConfigRecord) string {
	switch target.CallbackType {
	case callbackTypeHTTP:
		return target.TargetURL
	case callbackTypeGRPC:
		if target.RPCServiceName == "" {
			return "grpc://" + target.RPCEndpoint
		}
		return "grpc://" + target.RPCEndpoint + ensureLeadingSlash(target.RPCServiceName)
	case callbackTypeMQ:
		if target.MQExchange == "" {
			return "mq:///" + target.MQRoutingKey
		}
		return "mq://" + target.MQExchange + "/" + target.MQRoutingKey
	default:
		return ""
	}
}

func ensureLeadingSlash(path string) string {
	if path == "" || path[0] == '/' {
		return path
	}
	return "/" + path
}

func failureCode(err error) string {
	if err == nil {
		return ""
	}
	return "dispatch_failed"
}

func (d *Dispatcher) findJobOverride(ctx context.Context, jobID uint64) *model.TranscodeJobRequestOverride {
	if d.jobOverrideRepo == nil || jobID == 0 {
		return nil
	}

	override, ok := d.jobOverrideRepo.FindByJobID(ctx, jobID)
	if !ok {
		return nil
	}
	return &override
}

func (d *Dispatcher) currentConfig() config.DynamicRuntimeConfig {
	if d.effectiveConfig == nil {
		return config.DynamicRuntimeConfig{}
	}
	return d.effectiveConfig.Snapshot()
}
