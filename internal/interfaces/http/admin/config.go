package admin

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// ConfigHandler 负责后台运行时配置的版本创建与发布。
//
// 这里明确维护三份状态：
// 1. MySQL 中的待发布/已发布版本；
// 2. Redis 中的已发布运行时快照；
// 3. 当前进程内的生效配置快照。
type ConfigHandler struct {
	runtimeConfigRepository *mysql.RuntimeConfigRepository
	runtimeConfigCache      *rediscache.RuntimeConfigCache
	effectiveConfig         *configcenter.EffectiveConfig
}

// NewConfigHandler 创建运行时配置处理器。
func NewConfigHandler(runtimeConfigRepository *mysql.RuntimeConfigRepository, runtimeConfigCache *rediscache.RuntimeConfigCache, effectiveConfig *configcenter.EffectiveConfig) *ConfigHandler {
	return &ConfigHandler{
		runtimeConfigRepository: runtimeConfigRepository,
		runtimeConfigCache:      runtimeConfigCache,
		effectiveConfig:         effectiveConfig,
	}
}

// ListRuntimeVersions 返回全部运行时配置版本。
func (h *ConfigHandler) ListRuntimeVersions(w http.ResponseWriter, r *http.Request) {
	rawItems := h.runtimeConfigRepository.ListVersions(r.Context())
	items := make([]mysql.RuntimeConfigRecord, 0, len(rawItems))
	for _, item := range rawItems {
		items = append(items, sanitizeRuntimeConfigRecord(item))
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// UpdateRuntime 基于当前已发布版本生成一个新的待发布版本。
//
// 设计约束：
// - public gRPC 的运行期开关与监听地址已经独立建模；
// - legacy callback receiver 字段只保留历史兼容，不再写入新版本；
// - MQ 运行期开关与队列参数独立建模，但启用消费者时仍必须提供可消费的队列信息。
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
		StorageType                    *string `json:"storage_type"`
		StorageEndpoint                *string `json:"storage_endpoint"`
		StorageBucket                  *string `json:"storage_bucket"`
		StorageAccessKeyID             *string `json:"storage_access_key_id"`
		StorageSecretAccessKey         *string `json:"storage_secret_access_key"`
		StorageUseSSL                  *bool   `json:"storage_use_ssl"`
		StoragePlayDomain              *string `json:"storage_play_domain"`
		StorageFLVDomain               *string `json:"storage_flv_domain"`
		StorageLocalBasePath           *string `json:"storage_local_base_path"`
		DefaultStorageID               *uint64 `json:"default_storage_id"`
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

	// 兼容旧版本仍把 public gRPC 信息写在 legacy receiver 字段里的历史记录。
	// 一旦生成新版本，就统一迁移到 public_grpc_* 三个字段，避免后续语义继续串线。
	normalizeLegacyPublicGRPCFields(&record)

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
		record.PublicGRPCEnabled = *req.EnableGRPCServer
	}
	if req.GRPCListenAddress != nil {
		host, port, err := splitListenAddress(*req.GRPCListenAddress)
		if err != nil {
			logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
			return
		}
		record.PublicGRPCHost = host
		record.PublicGRPCPort = port
	}
	if req.EnableMQConsumer != nil {
		record.EnableMQConsumer = *req.EnableMQConsumer
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
	if req.StorageType != nil {
		record.StorageType = *req.StorageType
	}
	if req.StorageEndpoint != nil {
		record.StorageEndpoint = *req.StorageEndpoint
	}
	if req.StorageBucket != nil {
		record.StorageBucket = *req.StorageBucket
	}
	if req.StorageAccessKeyID != nil {
		record.StorageAccessKeyID = *req.StorageAccessKeyID
	}
	if req.StorageSecretAccessKey != nil {
		record.StorageSecretAccessKey = *req.StorageSecretAccessKey
	}
	if req.StorageUseSSL != nil {
		record.StorageUseSSL = *req.StorageUseSSL
	}
	if req.StoragePlayDomain != nil {
		record.StoragePlayDomain = *req.StoragePlayDomain
	}
	if req.StorageFLVDomain != nil {
		record.StorageFLVDomain = *req.StorageFLVDomain
	}
	if req.StorageLocalBasePath != nil {
		record.StorageLocalBasePath = *req.StorageLocalBasePath
	}
	if req.DefaultStorageID != nil {
		record.DefaultStorageID = *req.DefaultStorageID
	}
	if req.WorkerObjectPrefix != nil {
		record.WorkerObjectPrefix = *req.WorkerObjectPrefix
	}

	if record.EnableMQConsumer {
		if strings.TrimSpace(record.MQQueueName) == "" {
			logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "mq_queue_name is required when enable_mq_consumer=true"})
			return
		}
		if strings.TrimSpace(record.MQHost) == "" {
			logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "mq_host is required when enable_mq_consumer=true"})
			return
		}
	}

	clearLegacyCallbackReceiverFields(&record)

	record.ChangeSummary = req.ChangeSummary
	record.Published = false
	record.CreatedAt = now
	record.UpdatedAt = now

	if err := h.runtimeConfigRepository.Save(r.Context(), record); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save runtime config failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"pending_config_version": record.ConfigVersion,
		"published":              false,
	}})
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

// normalizeLegacyPublicGRPCFields 把旧字段中的 public gRPC 值提升到新字段。
//
// 这个步骤只在“基于已发布版本创建新版本”时执行一次，
// 目的是让历史版本完成平滑迁移，随后新版本只再写 public_grpc_*。
func normalizeLegacyPublicGRPCFields(record *mysql.RuntimeConfigRecord) {
	if record == nil {
		return
	}
	if record.PublicGRPCHost == "" && record.RPCCallbackReceiverHost != "" {
		record.PublicGRPCHost = record.RPCCallbackReceiverHost
	}
	if record.PublicGRPCPort <= 0 && record.RPCCallbackReceiverPort > 0 {
		record.PublicGRPCPort = record.RPCCallbackReceiverPort
	}
	if !record.PublicGRPCEnabled && record.RPCCallbackReceiverEnabled {
		record.PublicGRPCEnabled = true
	}
}

// clearLegacyCallbackReceiverFields 清空已经废弃的 callback receiver 运行时字段。
func clearLegacyCallbackReceiverFields(record *mysql.RuntimeConfigRecord) {
	if record == nil {
		return
	}
	record.RPCCallbackReceiverEnabled = false
	record.RPCCallbackReceiverHost = "0.0.0.0"
	record.RPCCallbackReceiverPort = 0
	record.RPCCallbackReceiverRegistryID = 0
}

// PublishRuntime 发布指定版本，并同步刷新 Redis 与当前进程内快照。
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

	latestConfig := mysql.ToDynamicRuntimeConfig(record)
	if h.runtimeConfigCache != nil {
		// 发布顺序固定为：先删缓存，再回填最新版本，最后替换进程内快照。
		// 这样其它实例即使在切换窗口命中 Redis，也不会继续读到旧版本。
		if err := h.runtimeConfigCache.InvalidatePublished(r.Context()); err != nil {
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "invalidate runtime config cache failed"})
			return
		}
		if err := h.runtimeConfigCache.SavePublished(r.Context(), rediscache.RuntimeConfigSnapshot{
			ConfigVersion: record.ConfigVersion,
			Config:        latestConfig,
		}); err != nil {
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save runtime config cache failed"})
			return
		}
	}

	h.effectiveConfig.ReplaceWithVersion(latestConfig, record.ConfigVersion)
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
