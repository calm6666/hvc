package config

import "time"

// DefaultDynamicRuntimeConfig 返回系统内置的运行时配置默认值。
//
// 这些值用于：
// 1. 空库首启时生成第一版 runtime config；
// 2. 历史配置文件中已经删除的动态配置段的默认兜底；
// 3. 后台尚未发布新版本时，为各模块提供稳定的初始行为。
func DefaultDynamicRuntimeConfig() DynamicRuntimeConfig {
	cfg := DynamicRuntimeConfig{
		Mode: ModeConfig{
			EnableHTTPServer: true,
			// public gRPC 属于运行期动态暴露能力。
			// 首启默认保持关闭，避免空库首次启动时自动把对外 RPC 端口暴露出去。
			EnableGRPCServer: false,
			EnableMQConsumer: false,
			EnableScheduler:  true,
			EnableWorker:     true,
			EnableCallback:   true,
		},
		Scheduler: SchedulerConfig{
			LoopInterval:                 3 * time.Second,
			JobLeaseTTL:                  5 * time.Minute,
			WorkerHeartbeatTimeout:       30 * time.Second,
			RequireHardwareEncode:        false,
			AllowSoftwareDecodeFallback:  true,
			RequireHardwareWatermark:     false,
			SoftDecodeCPULimitPercent:    80,
			NodeCPUSafetyLimitPercent:    90,
			NodeMemorySafetyLimitPercent: 90,
			NodeGPUSafetyLimitPercent:    90,
			MaxGlobalTranscodeSessions:   100,
			MaxNodeTranscodeSessions:     10,
			MaxNodeUploadConcurrency:     20,
			DynamicConcurrencyControl:    true,
		},
		Worker: WorkerConfig{
			LoopInterval:               2 * time.Second,
			SingleJobUploadConcurrency: 4,
			SourceReadTimeout:          30 * time.Minute,
			UploadRetryBaseDelay:       time.Second,
			UploadRetryMaxDelay:        30 * time.Second,
			UploadMaxRetryCount:        3,
			SegmentTemplate:            "{job_id}-{rendition_key}-{media_type}-{number}.m4s",
		},
		Callback: CallbackConfig{
			HTTPURL:      "",
			HTTPTimeout:  5 * time.Second,
			GRPCTimeout:  3 * time.Second,
			MQTimeout:    3 * time.Second,
			RetryBackoff: 2 * time.Second,
		},
		Storage: StorageConfig{
			StorageType:      "local",
			BasePrefix:       "hvc",
			LocalBasePath:    "/data/hvc/media",
			DefaultStorageID: 0,
		},
		GRPC: GRPCConfig{
			ListenAddress:     ":9090",
			MaxRecvMsgSizeMB:  64,
			MaxSendMsgSizeMB:  64,
			ConnectionTimeout: 5 * time.Second,
		},
		MQ: MQRuntimeConfig{
			Host:          "127.0.0.1",
			Port:          5672,
			Username:      "guest",
			Password:      "guest",
			VHost:         "/",
			ConsumerTag:   "vod-mq-consumer",
			PrefetchCount: 10,
			LoopInterval:  15 * time.Second,
			CallbackTopic: "",
		},
	}
	applyDynamicDefaults(&cfg)
	return cfg
}

// LoadBootstrapDynamicRuntimeConfig 构造首启时使用的运行时配置默认值。
//
// 约束如下：
// 1. 运行期业务配置的唯一生效来源是后台发布到数据库的 runtime config；
// 2. 本地 YAML 不再承载 scheduler/worker/callback/storage/public grpc/mq 这些业务动态段；
// 3. 外部配置中心不再直接下发业务运行时配置，只负责启动所需的基础设施配置。
func LoadBootstrapDynamicRuntimeConfig(base RuntimeConfig) DynamicRuntimeConfig {
	cfg := DefaultDynamicRuntimeConfig()
	switch base.EffectiveNodeMode() {
	case NodeModeStandalone:
		cfg.Mode.EnableHTTPServer = true
		cfg.Mode.EnableScheduler = true
		cfg.Mode.EnableWorker = true
		cfg.Mode.EnableCallback = true
	case NodeModeClusterControl:
		cfg.Mode.EnableHTTPServer = true
		cfg.Mode.EnableScheduler = true
		cfg.Mode.EnableWorker = false
		cfg.Mode.EnableCallback = true
	case NodeModeClusterWorker:
		cfg.Mode.EnableHTTPServer = false
		cfg.Mode.EnableScheduler = false
		cfg.Mode.EnableWorker = true
		cfg.Mode.EnableCallback = false
		cfg.Mode.EnableGRPCServer = false
		cfg.Mode.EnableMQConsumer = false
	case NodeModeClusterAllInOne:
		cfg.Mode.EnableHTTPServer = true
		cfg.Mode.EnableScheduler = true
		cfg.Mode.EnableWorker = true
		cfg.Mode.EnableCallback = true
	}
	applyDynamicDefaults(&cfg)
	return cfg
}

// NormalizeDynamicRuntimeConfig 对动态运行配置做统一默认值回填。
//
// 数据库存量历史配置、后台草稿配置、配置中心合并结果都应通过这个入口做一次归一化，
// 避免各模块分别补默认值，导致行为漂移。
func NormalizeDynamicRuntimeConfig(cfg DynamicRuntimeConfig) DynamicRuntimeConfig {
	applyDynamicDefaults(&cfg)
	return cfg
}

func applyDynamicDefaults(cfg *DynamicRuntimeConfig) {
	if cfg == nil {
		return
	}

	if cfg.Scheduler.LoopInterval <= 0 {
		cfg.Scheduler.LoopInterval = 5 * time.Second
	}
	if cfg.Scheduler.JobLeaseTTL <= 0 {
		cfg.Scheduler.JobLeaseTTL = time.Minute
	}
	if cfg.Scheduler.WorkerHeartbeatTimeout <= 0 {
		cfg.Scheduler.WorkerHeartbeatTimeout = 20 * time.Second
	}
	if cfg.Scheduler.SoftDecodeCPULimitPercent <= 0 {
		cfg.Scheduler.SoftDecodeCPULimitPercent = 50
	}
	if cfg.Scheduler.NodeCPUSafetyLimitPercent <= 0 {
		cfg.Scheduler.NodeCPUSafetyLimitPercent = 85
	}
	if cfg.Scheduler.NodeMemorySafetyLimitPercent <= 0 {
		cfg.Scheduler.NodeMemorySafetyLimitPercent = 85
	}
	if cfg.Scheduler.NodeGPUSafetyLimitPercent <= 0 {
		cfg.Scheduler.NodeGPUSafetyLimitPercent = 90
	}
	if cfg.Scheduler.MaxGlobalTranscodeSessions <= 0 {
		cfg.Scheduler.MaxGlobalTranscodeSessions = 10
	}
	if cfg.Scheduler.MaxNodeTranscodeSessions <= 0 {
		cfg.Scheduler.MaxNodeTranscodeSessions = cfg.Scheduler.MaxGlobalTranscodeSessions
	}
	if cfg.Scheduler.MaxNodeUploadConcurrency <= 0 {
		cfg.Scheduler.MaxNodeUploadConcurrency = 4
	}

	if cfg.Worker.LoopInterval <= 0 {
		cfg.Worker.LoopInterval = 15 * time.Second
	}
	if cfg.Worker.SingleJobUploadConcurrency <= 0 {
		cfg.Worker.SingleJobUploadConcurrency = 4
	}
	if cfg.Worker.UploadRetryBaseDelay <= 0 {
		cfg.Worker.UploadRetryBaseDelay = time.Second
	}
	if cfg.Worker.UploadRetryMaxDelay <= 0 {
		cfg.Worker.UploadRetryMaxDelay = 30 * time.Second
	}
	if cfg.Worker.UploadMaxRetryCount <= 0 {
		cfg.Worker.UploadMaxRetryCount = 3
	}
	if cfg.Worker.SegmentTemplate == "" {
		cfg.Worker.SegmentTemplate = "{job_id}-{rendition_key}-{media_type}-{number}.m4s"
	}

	if cfg.Callback.HTTPTimeout <= 0 {
		cfg.Callback.HTTPTimeout = 5 * time.Second
	}
	if cfg.Callback.GRPCTimeout <= 0 {
		cfg.Callback.GRPCTimeout = 3 * time.Second
	}
	if cfg.Callback.MQTimeout <= 0 {
		cfg.Callback.MQTimeout = 3 * time.Second
	}
	if cfg.Callback.RetryBackoff <= 0 {
		cfg.Callback.RetryBackoff = 2 * time.Second
	}

	if cfg.Storage.StorageType == "" {
		cfg.Storage.StorageType = "local"
	}
	if cfg.Storage.BasePrefix == "" {
		cfg.Storage.BasePrefix = "hvc"
	}
	if cfg.Storage.LocalBasePath == "" {
		cfg.Storage.LocalBasePath = "/data/hvc/media"
	}

	if cfg.GRPC.MaxRecvMsgSizeMB <= 0 {
		cfg.GRPC.MaxRecvMsgSizeMB = 64
	}
	if cfg.GRPC.MaxSendMsgSizeMB <= 0 {
		cfg.GRPC.MaxSendMsgSizeMB = 64
	}
	if cfg.GRPC.ConnectionTimeout <= 0 {
		cfg.GRPC.ConnectionTimeout = 5 * time.Second
	}

	if cfg.MQ.Port <= 0 {
		cfg.MQ.Port = 5672
	}
	if cfg.MQ.VHost == "" {
		cfg.MQ.VHost = "/"
	}
	if cfg.MQ.ConsumerTag == "" {
		cfg.MQ.ConsumerTag = "vod-mq-consumer"
	}
	if cfg.MQ.PrefetchCount <= 0 {
		cfg.MQ.PrefetchCount = 10
	}
	if cfg.MQ.LoopInterval <= 0 {
		cfg.MQ.LoopInterval = 15 * time.Second
	}
}
