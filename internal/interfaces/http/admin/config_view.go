package admin

import (
	"fmt"
	"time"

	"hvc/internal/infra/db/mysql"
)

type callbackConfigView struct {
	CallbackConfigID uint64    `json:"callback_config_id"`
	CallbackName     string    `json:"callback_name"`
	CallbackType     int       `json:"callback_type"`
	TargetURL        string    `json:"target_url"`
	RPCEndpoint      string    `json:"rpc_endpoint"`
	RPCServiceName   string    `json:"rpc_service_name"`
	MQExchange       string    `json:"mq_exchange"`
	MQRoutingKey     string    `json:"mq_routing_key"`
	TimeoutMS        int       `json:"timeout_ms"`
	RetryTimes       int       `json:"retry_times"`
	Enabled          bool      `json:"enabled"`
	Priority         int       `json:"priority"`
	RegistryID       uint64    `json:"registry_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type configCenterBindingView struct {
	BindingID       uint64     `json:"binding_id"`
	BindingName     string     `json:"binding_name"`
	ProviderType    string     `json:"provider_type"`
	Endpoint        string     `json:"endpoint"`
	Namespace       string     `json:"namespace"`
	AuthMode        string     `json:"auth_mode"`
	AccessKey       string     `json:"access_key"`
	SecretKey       string     `json:"secret_key"`
	Token           string     `json:"token"`
	Enabled         bool       `json:"enabled"`
	Priority        int        `json:"priority"`
	LastSyncStatus  string     `json:"last_sync_status"`
	LastSyncMessage string     `json:"last_sync_message"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ConfigScope     string     `json:"config_scope"`
	AffectsRuntime  bool       `json:"affects_runtime"`
	BindingUsage    []string   `json:"binding_usage"`
}

type registryEtcdConfigView struct {
	RegistryID       uint64    `json:"registry_id"`
	RegistryName     string    `json:"registry_name"`
	Endpoints        string    `json:"endpoints"`
	ServiceNamespace string    `json:"service_namespace"`
	LeaseTTLSec      int       `json:"lease_ttl_sec"`
	DialTimeoutMS    int       `json:"dial_timeout_ms"`
	Enabled          bool      `json:"enabled"`
	Priority         int       `json:"priority"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type runtimeConfigVersionView struct {
	ConfigVersion                  uint64     `json:"config_version"`
	EnableHTTPServer               bool       `json:"enable_http_server"`
	EnableCallback                 bool       `json:"enable_callback"`
	DefaultProfileID               uint64     `json:"default_profile_id"`
	MaxGlobalTranscodeSessions     int        `json:"max_global_transcode_sessions"`
	JobLeaseTTLSec                 int        `json:"job_lease_ttl_sec"`
	WorkerHeartbeatTimeoutSec      int        `json:"worker_heartbeat_timeout_sec"`
	AllowRequestOverrideProfile    bool       `json:"allow_request_override_profile"`
	AllowRequestOverrideSegmentDur bool       `json:"allow_request_override_segment_duration"`
	AllowRequestOverrideHWAccel    bool       `json:"allow_request_override_hwaccel"`
	Published                      bool       `json:"published"`
	SchedulerLoopIntervalMS        int        `json:"scheduler_loop_interval_ms"`
	WorkerLoopIntervalMS           int        `json:"worker_loop_interval_ms"`
	RequireHardwareEncode          bool       `json:"require_hardware_encode"`
	AllowSoftwareDecodeFallback    bool       `json:"allow_software_decode_fallback"`
	RequireHardwareWatermark       bool       `json:"require_hardware_watermark"`
	NodeCPUSafetyLimitPercent      int        `json:"node_cpu_safety_limit_percent"`
	NodeMemorySafetyLimitPercent   int        `json:"node_memory_safety_limit_percent"`
	NodeGPUSafetyLimitPercent      int        `json:"node_gpu_safety_limit_percent"`
	SingleJobUploadConcurrency     int        `json:"single_job_upload_concurrency"`
	CallbackHTTPURL                string     `json:"callback_http_url"`
	CallbackRPCEndpoint            string     `json:"callback_rpc_endpoint"`
	CallbackMQTopic                string     `json:"callback_mq_topic"`
	EnableGRPCServer               bool       `json:"enable_grpc_server"`
	GRPCListenAddress              string     `json:"grpc_listen_address"`
	PublicGRPCRegistryID           uint64     `json:"public_grpc_registry_id"`
	EnableMQConsumer               bool       `json:"enable_mq_consumer"`
	MQQueueName                    string     `json:"mq_queue_name"`
	MQHost                         string     `json:"mq_host"`
	MQPort                         int        `json:"mq_port"`
	MQUsername                     string     `json:"mq_username"`
	MQPassword                     string     `json:"mq_password"`
	MQVHost                        string     `json:"mq_vhost"`
	MQConsumerTag                  string     `json:"mq_consumer_tag"`
	MQPrefetchCount                int        `json:"mq_prefetch_count"`
	StorageType                    string     `json:"storage_type"`
	StorageEndpoint                string     `json:"storage_endpoint"`
	StorageBucket                  string     `json:"storage_bucket"`
	StorageAccessKeyID             string     `json:"storage_access_key_id"`
	StorageSecretAccessKey         string     `json:"storage_secret_access_key"`
	StorageUseSSL                  bool       `json:"storage_use_ssl"`
	StoragePlayDomain              string     `json:"storage_play_domain"`
	StorageFLVDomain               string     `json:"storage_flv_domain"`
	StorageLocalBasePath           string     `json:"storage_local_base_path"`
	DefaultStorageID               uint64     `json:"default_storage_id"`
	WorkerObjectPrefix             string     `json:"worker_object_prefix"`
	ChangeSummary                  string     `json:"change_summary"`
	ConfigSource                   string     `json:"config_source"`
	SourceRevision                 string     `json:"source_revision"`
	PublishedBy                    string     `json:"published_by"`
	PublishedAt                    *time.Time `json:"published_at,omitempty"`
	EffectiveConfigHash            string     `json:"effective_config_hash"`
	CreatedAt                      time.Time  `json:"created_at"`
	UpdatedAt                      time.Time  `json:"updated_at"`
}

func toCallbackConfigView(record mysql.CallbackConfigRecord) callbackConfigView {
	return callbackConfigView{
		CallbackConfigID: record.CallbackConfigID,
		CallbackName:     record.CallbackName,
		CallbackType:     record.CallbackType,
		TargetURL:        record.TargetURL,
		RPCEndpoint:      record.RPCEndpoint,
		RPCServiceName:   record.RPCServiceName,
		MQExchange:       record.MQExchange,
		MQRoutingKey:     record.MQRoutingKey,
		TimeoutMS:        record.TimeoutMS,
		RetryTimes:       record.RetryTimes,
		Enabled:          record.Enabled,
		Priority:         record.Priority,
		RegistryID:       record.RegistryID,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func buildConfigCenterBindingView(record mysql.ConfigCenterBindingRecord) configCenterBindingView {
	view := configCenterBindingView{
		BindingID:       record.BindingID,
		BindingName:     record.BindingName,
		ProviderType:    record.ProviderType,
		Endpoint:        record.Endpoint,
		Namespace:       record.Namespace,
		AuthMode:        record.AuthMode,
		AccessKey:       record.AccessKey,
		SecretKey:       record.SecretKey,
		Token:           record.Token,
		Enabled:         record.Enabled,
		Priority:        record.Priority,
		LastSyncStatus:  record.LastSyncStatus,
		LastSyncMessage: record.LastSyncMessage,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
		ConfigScope:     configCenterBindingScopeBootstrap,
		AffectsRuntime:  false,
		BindingUsage: []string{
			"mysql",
			"redis",
			"cluster_registry",
			"node_identity",
		},
	}
	if record.LastSyncAt != nil && !record.LastSyncAt.IsZero() {
		lastSyncAt := *record.LastSyncAt
		view.LastSyncAt = &lastSyncAt
	}
	return view
}

func toRuntimeConfigVersionView(record mysql.RuntimeConfigRecord) runtimeConfigVersionView {
	view := runtimeConfigVersionView{
		ConfigVersion:                  record.ConfigVersion,
		EnableHTTPServer:               record.EnableHTTPServer,
		EnableCallback:                 record.EnableCallback,
		DefaultProfileID:               record.DefaultProfileID,
		MaxGlobalTranscodeSessions:     record.MaxGlobalTranscodeSessions,
		JobLeaseTTLSec:                 record.JobLeaseTTLSeconds,
		WorkerHeartbeatTimeoutSec:      record.WorkerHeartbeatTimeoutSec,
		AllowRequestOverrideProfile:    record.AllowRequestOverrideProfile,
		AllowRequestOverrideSegmentDur: record.AllowRequestOverrideSegmentDur,
		AllowRequestOverrideHWAccel:    record.AllowRequestOverrideHWAccel,
		Published:                      record.Published,
		SchedulerLoopIntervalMS:        record.SchedulerLoopIntervalMS,
		WorkerLoopIntervalMS:           record.WorkerLoopIntervalMS,
		RequireHardwareEncode:          record.RequireHardwareEncode,
		AllowSoftwareDecodeFallback:    record.AllowSoftwareDecodeFallback,
		RequireHardwareWatermark:       record.RequireHardwareWatermark,
		NodeCPUSafetyLimitPercent:      record.NodeCPUSafetyLimitPercent,
		NodeMemorySafetyLimitPercent:   record.NodeMemorySafetyLimitPercent,
		NodeGPUSafetyLimitPercent:      record.NodeGPUMemorySafetyLimitPercent,
		SingleJobUploadConcurrency:     record.SingleJobUploadConcurrencyLimit,
		CallbackHTTPURL:                record.CallbackHTTPURL,
		CallbackRPCEndpoint:            record.CallbackRPCEndpoint,
		CallbackMQTopic:                record.CallbackMQTopic,
		EnableGRPCServer:               record.PublicGRPCEnabled,
		GRPCListenAddress:              buildListenAddress(record.PublicGRPCHost, record.PublicGRPCPort),
		PublicGRPCRegistryID:           record.PublicGRPCRegistryID,
		EnableMQConsumer:               record.EnableMQConsumer,
		MQQueueName:                    record.MQQueueName,
		MQHost:                         record.MQHost,
		MQPort:                         record.MQPort,
		MQUsername:                     record.MQUsername,
		MQPassword:                     record.MQPassword,
		MQVHost:                        record.MQVHost,
		MQConsumerTag:                  record.MQConsumerTag,
		MQPrefetchCount:                record.MQPrefetchCount,
		StorageType:                    record.StorageType,
		StorageEndpoint:                record.StorageEndpoint,
		StorageBucket:                  record.StorageBucket,
		StorageAccessKeyID:             record.StorageAccessKeyID,
		StorageSecretAccessKey:         record.StorageSecretAccessKey,
		StorageUseSSL:                  record.StorageUseSSL,
		StoragePlayDomain:              record.StoragePlayDomain,
		StorageFLVDomain:               record.StorageFLVDomain,
		StorageLocalBasePath:           record.StorageLocalBasePath,
		DefaultStorageID:               record.DefaultStorageID,
		WorkerObjectPrefix:             record.WorkerObjectPrefix,
		ChangeSummary:                  record.ChangeSummary,
		ConfigSource:                   record.ConfigSource,
		SourceRevision:                 record.SourceRevision,
		PublishedBy:                    record.PublishedBy,
		EffectiveConfigHash:            record.EffectiveConfigHash,
		CreatedAt:                      record.CreatedAt,
		UpdatedAt:                      record.UpdatedAt,
	}
	if record.PublishedAt != nil && !record.PublishedAt.IsZero() {
		publishedAt := *record.PublishedAt
		view.PublishedAt = &publishedAt
	}
	return view
}

func toRegistryEtcdConfigView(record mysql.RegistryEtcdConfigRecord) registryEtcdConfigView {
	return registryEtcdConfigView{
		RegistryID:       record.RegistryID,
		RegistryName:     record.RegistryName,
		Endpoints:        record.Endpoints,
		ServiceNamespace: record.ServiceNamespace,
		LeaseTTLSec:      record.LeaseTTLSec,
		DialTimeoutMS:    record.DialTimeoutMS,
		Enabled:          record.Enabled,
		Priority:         record.Priority,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func buildListenAddress(host string, port int) string {
	if host == "" {
		host = "0.0.0.0"
	}
	if port <= 0 {
		port = 9090
	}
	return fmt.Sprintf("%s:%d", host, port)
}
