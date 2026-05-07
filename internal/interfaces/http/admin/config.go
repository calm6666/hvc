package admin

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// ConfigHandler 处理后台动态配置接口。
type ConfigHandler struct {
	runtimeConfigRepository *mysql.RuntimeConfigRepository
	effectiveConfig         *configcenter.EffectiveConfig
}

// NewConfigHandler 创建后台动态配置处理器。
func NewConfigHandler(runtimeConfigRepository *mysql.RuntimeConfigRepository, effectiveConfig *configcenter.EffectiveConfig) *ConfigHandler {
	return &ConfigHandler{runtimeConfigRepository: runtimeConfigRepository, effectiveConfig: effectiveConfig}
}

// ListRuntimeVersions 返回运行配置版本列表。
func (h *ConfigHandler) ListRuntimeVersions(w http.ResponseWriter, r *http.Request) {
	items := h.runtimeConfigRepository.ListVersions(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// UpdateRuntime 创建新的运行配置版本。
func (h *ConfigHandler) UpdateRuntime(w http.ResponseWriter, r *http.Request) {
	base, ok := h.runtimeConfigRepository.LatestPublished(r.Context())
	if !ok {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "no base runtime config"})
		return
	}
	var req struct {
		EnableHTTPServer               *bool   `json:"enable_http_server"`
		EnableCallback                 *bool   `json:"enable_callback"`
		DefaultProfileID               *uint64 `json:"default_profile_id"`
		MaxGlobalTranscodeSessions     *int    `json:"max_global_transcode_sessions"`
		JobLeaseTTLSec                 *int    `json:"job_lease_ttl_sec"`
		WorkerHeartbeatTimeoutSec      *int    `json:"worker_heartbeat_timeout_sec"`
		AllowRequestOverrideProfile    *bool   `json:"allow_request_override_profile"`
		AllowRequestOverrideSegmentDur *bool   `json:"allow_request_override_segment_duration"`
		AllowRequestOverrideHWAccel    *bool   `json:"allow_request_override_hwaccel"`
		SchedulerLoopIntervalMS        *int    `json:"scheduler_loop_interval_ms"`
		WorkerLoopIntervalMS           *int    `json:"worker_loop_interval_ms"`
		SingleJobUploadConcurrency     *int    `json:"single_job_upload_concurrency"`
		RequireHardwareEncode          *bool   `json:"require_hardware_encode"`
		AllowSoftwareDecodeFallback    *bool   `json:"allow_software_decode_fallback"`
		RequireHardwareWatermark       *bool   `json:"require_hardware_watermark"`
		NodeCPUSafetyLimitPercent      *int    `json:"node_cpu_safety_limit_percent"`
		NodeMemorySafetyLimitPercent   *int    `json:"node_memory_safety_limit_percent"`
		NodeGPUSafetyLimitPercent      *int    `json:"node_gpu_safety_limit_percent"`
		CallbackHTTPURL                *string `json:"callback_http_url"`
		CallbackMQTopic                *string `json:"callback_mq_topic"`
		EnableGRPCServer               *bool   `json:"enable_grpc_server"`
		GRPCListenAddress              *string `json:"grpc_listen_address"`
		MQQueueName                    *string `json:"mq_queue_name"`
		EnableMQConsumer               *bool   `json:"enable_mq_consumer"`
		MQHost                         *string `json:"mq_host"`
		MQPort                         *int    `json:"mq_port"`
		MQUsername                     *string `json:"mq_username"`
		MQPassword                     *string `json:"mq_password"`
		MQVHost                        *string `json:"mq_vhost"`
		MQConsumerTag                  *string `json:"mq_consumer_tag"`
		MQPrefetchCount                *int    `json:"mq_prefetch_count"`
		MQLoopIntervalMS               *int    `json:"mq_loop_interval_ms"`
		WorkerObjectPrefix             *string `json:"worker_object_prefix"`
		ChangeSummary                  string  `json:"change_summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	now := time.Now()
	record := base
	record.ConfigVersion = idgen.Next()
	if req.EnableHTTPServer != nil {
		record.EnableHTTPServer = *req.EnableHTTPServer
	}
	if req.EnableCallback != nil {
		record.EnableCallback = *req.EnableCallback
	}
	if req.DefaultProfileID != nil {
		record.DefaultProfileID = *req.DefaultProfileID
	}
	if req.MaxGlobalTranscodeSessions != nil {
		record.MaxGlobalTranscodeSessions = *req.MaxGlobalTranscodeSessions
	}
	if req.JobLeaseTTLSec != nil {
		record.JobLeaseTTLSeconds = *req.JobLeaseTTLSec
	}
	if req.WorkerHeartbeatTimeoutSec != nil {
		record.WorkerHeartbeatTimeoutSec = *req.WorkerHeartbeatTimeoutSec
	}
	if req.AllowRequestOverrideProfile != nil {
		record.AllowRequestOverrideProfile = *req.AllowRequestOverrideProfile
	}
	if req.AllowRequestOverrideSegmentDur != nil {
		record.AllowRequestOverrideSegmentDur = *req.AllowRequestOverrideSegmentDur
	}
	if req.AllowRequestOverrideHWAccel != nil {
		record.AllowRequestOverrideHWAccel = *req.AllowRequestOverrideHWAccel
	}
	if req.SchedulerLoopIntervalMS != nil {
		record.SchedulerLoopIntervalMS = *req.SchedulerLoopIntervalMS
	}
	if req.WorkerLoopIntervalMS != nil {
		record.WorkerLoopIntervalMS = *req.WorkerLoopIntervalMS
	}
	if req.SingleJobUploadConcurrency != nil {
		record.SingleJobUploadConcurrencyLimit = *req.SingleJobUploadConcurrency
	}
	if req.RequireHardwareEncode != nil {
		record.RequireHardwareEncode = *req.RequireHardwareEncode
	}
	if req.AllowSoftwareDecodeFallback != nil {
		record.AllowSoftwareDecodeFallback = *req.AllowSoftwareDecodeFallback
	}
	if req.RequireHardwareWatermark != nil {
		record.RequireHardwareWatermark = *req.RequireHardwareWatermark
	}
	if req.NodeCPUSafetyLimitPercent != nil {
		record.NodeCPUSafetyLimitPercent = *req.NodeCPUSafetyLimitPercent
	}
	if req.NodeMemorySafetyLimitPercent != nil {
		record.NodeMemorySafetyLimitPercent = *req.NodeMemorySafetyLimitPercent
	}
	if req.NodeGPUSafetyLimitPercent != nil {
		record.NodeGPUMemorySafetyLimitPercent = *req.NodeGPUSafetyLimitPercent
	}
	if req.CallbackHTTPURL != nil {
		record.CallbackHTTPURL = *req.CallbackHTTPURL
	}
	if req.CallbackMQTopic != nil {
		record.CallbackMQTopic = *req.CallbackMQTopic
	}
	if req.EnableGRPCServer != nil {
		record.RPCCallbackReceiverEnabled = *req.EnableGRPCServer
	}
	if req.GRPCListenAddress != nil {
		host, port, err := splitListenAddress(*req.GRPCListenAddress)
		if err != nil {
			logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
			return
		}
		record.RPCCallbackReceiverHost = host
		record.RPCCallbackReceiverPort = port
	}
	if req.EnableMQConsumer != nil && !*req.EnableMQConsumer {
		record.MQQueueName = ""
	}
	if req.MQQueueName != nil {
		record.MQQueueName = *req.MQQueueName
	}
	if req.MQHost != nil {
		record.MQHost = *req.MQHost
	}
	if req.MQPort != nil {
		record.MQPort = *req.MQPort
	}
	if req.MQUsername != nil {
		record.MQUsername = *req.MQUsername
	}
	if req.MQPassword != nil {
		record.MQPassword = *req.MQPassword
	}
	if req.MQVHost != nil {
		record.MQVHost = *req.MQVHost
	}
	if req.MQConsumerTag != nil {
		record.MQConsumerTag = *req.MQConsumerTag
	}
	if req.MQPrefetchCount != nil {
		record.MQPrefetchCount = *req.MQPrefetchCount
	}
	if req.MQLoopIntervalMS != nil {
		record.MQLoopIntervalMS = *req.MQLoopIntervalMS
	}
	if req.WorkerObjectPrefix != nil {
		record.WorkerObjectPrefix = *req.WorkerObjectPrefix
	}
	record.ChangeSummary = req.ChangeSummary
	record.Published = false
	record.CreatedAt = now
	record.UpdatedAt = now
	if err := h.runtimeConfigRepository.Save(r.Context(), record); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save runtime config failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"pending_config_version": record.ConfigVersion, "published": false}})
}

func splitListenAddress(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host, port, nil
}

// PublishRuntime 发布指定运行配置版本并替换当前生效配置。
func (h *ConfigHandler) PublishRuntime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigVersion uint64 `json:"config_version"`
		PublishReason string `json:"publish_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	record, ok := h.runtimeConfigRepository.FindByVersion(r.Context(), req.ConfigVersion)
	if !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "config version not found"})
		return
	}
	if err := h.runtimeConfigRepository.MarkPublished(r.Context(), req.ConfigVersion, "admin"); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "publish config failed"})
		return
	}
	h.effectiveConfig.Replace(mysql.ToDynamicRuntimeConfig(record))
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"config_version": req.ConfigVersion,
		"published":      true,
		"effective_scope": map[string]any{
			"new_jobs":     true,
			"queued_jobs":  true,
			"running_jobs": false,
		},
	}})
}
