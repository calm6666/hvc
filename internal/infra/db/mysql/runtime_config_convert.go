package mysql

import (
	"context"
	"fmt"
	"net"
	"time"

	"hvc/internal/config"
	"hvc/pkg/idgen"
)

// ToDynamicRuntimeConfig 把数据库运行配置记录转换为当前进程消费的动态运行配置。
//
// 第一阶段后台能力的关键点不是字段抽象有多优雅，而是管理员改完配置后，
// 系统真的能把配置记录转成 DynamicRuntimeConfig 并替换到生效容器里。
func ToDynamicRuntimeConfig(record RuntimeConfigRecord) config.DynamicRuntimeConfig {
	return config.NormalizeDynamicRuntimeConfig(config.DynamicRuntimeConfig{
		Mode: config.ModeConfig{
			EnableHTTPServer: record.EnableHTTPServer,
			// public gRPC 已独立建模；这里保留对旧字段的兜底兼容，
			// 目的是让已落库但尚未执行增量 SQL 的历史版本依旧可被正确加载。
			EnableGRPCServer: publicGRPCEnabled(record),
			EnableMQConsumer: record.EnableMQConsumer,
			EnableScheduler:  true,
			EnableWorker:     true,
			EnableCallback:   record.EnableCallback,
		},
		Scheduler: config.SchedulerConfig{
			LoopInterval:                 time.Duration(record.SchedulerLoopIntervalMS) * time.Millisecond,
			JobLeaseTTL:                  time.Duration(record.JobLeaseTTLSeconds) * time.Second,
			WorkerHeartbeatTimeout:       time.Duration(record.WorkerHeartbeatTimeoutSec) * time.Second,
			RequireHardwareEncode:        record.RequireHardwareEncode,
			AllowSoftwareDecodeFallback:  record.AllowSoftwareDecodeFallback,
			RequireHardwareWatermark:     record.RequireHardwareWatermark,
			SoftDecodeCPULimitPercent:    record.SoftDecodeCPULimitPercent,
			NodeCPUSafetyLimitPercent:    record.NodeCPUSafetyLimitPercent,
			NodeMemorySafetyLimitPercent: record.NodeMemorySafetyLimitPercent,
			NodeGPUSafetyLimitPercent:    record.NodeGPUMemorySafetyLimitPercent,
			MaxGlobalTranscodeSessions:   record.MaxGlobalTranscodeSessions,
			MaxNodeTranscodeSessions:     max(1, record.MaxGlobalTranscodeSessions),
			MaxNodeUploadConcurrency:     record.SingleJobUploadConcurrencyLimit,
			DynamicConcurrencyControl:    record.DynamicConcurrencyControlEnabled,
		},
		Worker: config.WorkerConfig{
			LoopInterval:               time.Duration(record.WorkerLoopIntervalMS) * time.Millisecond,
			SingleJobUploadConcurrency: record.SingleJobUploadConcurrencyLimit,
			SegmentTemplate:            "{job_id}-{rendition_key}-{media_type}-{number}.m4s",
			UploadMaxRetryCount:        3,
		},
		Callback: config.CallbackConfig{
			HTTPURL:      record.CallbackHTTPURL,
			HTTPTimeout:  3 * time.Second,
			GRPCTimeout:  3 * time.Second,
			MQTimeout:    3 * time.Second,
			RetryBackoff: 2 * time.Second,
		},
		Storage: config.StorageConfig{
			Endpoint:         record.StorageEndpoint,
			Bucket:           record.StorageBucket,
			AccessKeyID:      record.StorageAccessKeyID,
			SecretAccessKey:  record.StorageSecretAccessKey,
			UseSSL:           record.StorageUseSSL,
			BasePrefix:       record.WorkerObjectPrefix,
			PlayDomain:       record.StoragePlayDomain,
			FLVDomain:        record.StorageFLVDomain,
			StorageType:      fallbackStorageType(record.StorageType),
			LocalBasePath:    record.StorageLocalBasePath,
			DefaultStorageID: record.DefaultStorageID,
		},
		GRPC: config.GRPCConfig{
			ListenAddress:     publicGRPCListenAddress(record),
			MaxRecvMsgSizeMB:  64,
			MaxSendMsgSizeMB:  64,
			ConnectionTimeout: 5 * time.Second,
		},
		MQ: config.MQRuntimeConfig{
			QueueName:     record.MQQueueName,
			Host:          record.MQHost,
			Port:          record.MQPort,
			Username:      record.MQUsername,
			Password:      record.MQPassword,
			VHost:         record.MQVHost,
			ConsumerTag:   record.MQConsumerTag,
			PrefetchCount: record.MQPrefetchCount,
			LoopInterval:  time.Duration(record.MQLoopIntervalMS) * time.Millisecond,
			CallbackTopic: record.CallbackMQTopic,
		},
	})
}

func publicGRPCEnabled(record RuntimeConfigRecord) bool {
	if shouldFallbackLegacyPublicGRPC(record) {
		return record.RPCCallbackReceiverEnabled
	}
	return record.PublicGRPCEnabled
}

func publicGRPCListenAddress(record RuntimeConfigRecord) string {
	host := record.PublicGRPCHost
	port := record.PublicGRPCPort
	if shouldFallbackLegacyPublicGRPC(record) || (host == "" && port <= 0) {
		host = record.RPCCallbackReceiverHost
		port = record.RPCCallbackReceiverPort
	}
	if host == "" {
		host = "0.0.0.0"
	}
	if port <= 0 {
		port = 9090
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func shouldFallbackLegacyPublicGRPC(record RuntimeConfigRecord) bool {
	if record.PublicGRPCEnabled {
		return false
	}
	if record.PublicGRPCHost == "" && record.PublicGRPCPort <= 0 {
		return true
	}
	// 兼容“只完成自动加列、尚未做历史回填”的过渡状态：
	// 此时新列往往仍是默认值，而真实配置仍保存在 legacy receiver 字段里。
	if record.RPCCallbackReceiverEnabled &&
		(record.PublicGRPCHost == "" || record.PublicGRPCHost == "0.0.0.0") &&
		(record.PublicGRPCPort == 0 || record.PublicGRPCPort == 9090) {
		return true
	}
	return false
}

// NewBootstrapRuntimeConfigRecord 生成首启时落库的已发布 runtime config。
func NewBootstrapRuntimeConfigRecord(cfg config.DynamicRuntimeConfig, source string) RuntimeConfigRecord {
	now := time.Now()
	host, port := splitBootstrapListenAddress(cfg.GRPC.ListenAddress)
	queueName := cfg.MQ.QueueName
	return RuntimeConfigRecord{
		ConfigVersion:                    idgen.Next(),
		EnableHTTPServer:                 cfg.Mode.EnableHTTPServer,
		EnableCallback:                   cfg.Mode.EnableCallback,
		DefaultProfileID:                 0,
		MaxGlobalTranscodeSessions:       cfg.Scheduler.MaxGlobalTranscodeSessions,
		JobLeaseTTLSeconds:               int(cfg.Scheduler.JobLeaseTTL / time.Second),
		WorkerHeartbeatTimeoutSec:        int(cfg.Scheduler.WorkerHeartbeatTimeout / time.Second),
		AllowRequestOverrideProfile:      true,
		AllowRequestOverrideSegmentDur:   true,
		AllowRequestOverrideHWAccel:      true,
		Published:                        true,
		SchedulerLoopIntervalMS:          int(cfg.Scheduler.LoopInterval / time.Millisecond),
		WorkerAssignedStatus:             3,
		WorkerProbeFailProgressPermille:  0,
		WorkerUploadFailProgressPermille: 800,
		WorkerSuccessProgressPermille:    1000,
		WorkerLoopIntervalMS:             int(cfg.Worker.LoopInterval / time.Millisecond),
		RPCLoopIntervalMS:                int(cfg.GRPC.ConnectionTimeout / time.Millisecond),
		MQLoopIntervalMS:                 int(cfg.MQ.LoopInterval / time.Millisecond),
		RequireHardwareEncode:            cfg.Scheduler.RequireHardwareEncode,
		AllowSoftwareDecodeFallback:      cfg.Scheduler.AllowSoftwareDecodeFallback,
		SoftDecodeCPULimitPercent:        cfg.Scheduler.SoftDecodeCPULimitPercent,
		NodeCPUSafetyLimitPercent:        cfg.Scheduler.NodeCPUSafetyLimitPercent,
		NodeMemorySafetyLimitPercent:     cfg.Scheduler.NodeMemorySafetyLimitPercent,
		NodeGPUMemorySafetyLimitPercent:  cfg.Scheduler.NodeGPUSafetyLimitPercent,
		SingleJobUploadConcurrencyLimit:  cfg.Worker.SingleJobUploadConcurrency,
		DynamicConcurrencyControlEnabled: cfg.Scheduler.DynamicConcurrencyControl,
		RequireHardwareWatermark:         cfg.Scheduler.RequireHardwareWatermark,
		WorkerProbeFailMessage:           "FFprobe probing failed",
		WorkerUploadFailMessage:          "object storage upload failed",
		WorkerSuccessMessage:             "job execution completed",
		CallbackHTTPURL:                  cfg.Callback.HTTPURL,
		CallbackRPCEndpoint:              "",
		CallbackMQTopic:                  cfg.MQ.CallbackTopic,
		PublicGRPCEnabled:                cfg.Mode.EnableGRPCServer,
		PublicGRPCHost:                   host,
		PublicGRPCPort:                   port,
		// legacy callback receiver 字段已不再承载 public gRPC 运行期开关。
		// 这里保持零值，避免新版本继续把两套语义混写到一起。
		RPCCallbackReceiverEnabled:    false,
		RPCCallbackReceiverHost:       "0.0.0.0",
		RPCCallbackReceiverPort:       0,
		RPCCallbackReceiverRegistryID: 0,
		EnableMQConsumer:              cfg.Mode.EnableMQConsumer,
		MQQueueName:                   queueName,
		MQHost:                        cfg.MQ.Host,
		MQPort:                        cfg.MQ.Port,
		MQUsername:                    cfg.MQ.Username,
		MQPassword:                    cfg.MQ.Password,
		MQVHost:                       cfg.MQ.VHost,
		MQConsumerTag:                 cfg.MQ.ConsumerTag,
		MQPrefetchCount:               cfg.MQ.PrefetchCount,
		StorageType:                   cfg.Storage.StorageType,
		StorageEndpoint:               cfg.Storage.Endpoint,
		StorageBucket:                 cfg.Storage.Bucket,
		StorageAccessKeyID:            cfg.Storage.AccessKeyID,
		StorageSecretAccessKey:        cfg.Storage.SecretAccessKey,
		StorageUseSSL:                 cfg.Storage.UseSSL,
		StoragePlayDomain:             cfg.Storage.PlayDomain,
		StorageFLVDomain:              cfg.Storage.FLVDomain,
		StorageLocalBasePath:          cfg.Storage.LocalBasePath,
		DefaultStorageID:              cfg.Storage.DefaultStorageID,
		SchedulerWorkerID:             "worker-default",
		WorkerOutputPath:              "",
		WorkerObjectPrefix:            cfg.Storage.BasePrefix,
		ChangeSummary:                 "bootstrap initial runtime config",
		ConfigSource:                  source,
		SourceRevision:                "",
		PublishedBy:                   "bootstrap",
		PublishedAt:                   now,
		EffectiveConfigHash:           "",
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}
}

func splitBootstrapListenAddress(address string) (string, int) {
	if address == "" {
		return "0.0.0.0", 9090
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "0.0.0.0", 9090
	}
	port := 9090
	if parsed, parseErr := net.LookupPort("tcp", portText); parseErr == nil {
		port = parsed
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host, port
}

func fallbackStorageType(value string) string {
	if value != "" {
		return value
	}
	return "local"
}

// ListVersions 返回运行配置版本列表。
func (r *RuntimeConfigRepository) ListVersions(ctx context.Context) []RuntimeConfigRecord {
	var records []RuntimeConfigRecord
	if err := r.db.WithContext(ctx).Order("config_version desc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// FindByVersion 根据版本号查询运行配置。
func (r *RuntimeConfigRepository) FindByVersion(ctx context.Context, configVersion uint64) (RuntimeConfigRecord, bool) {
	var record RuntimeConfigRecord
	if err := r.db.WithContext(ctx).Where("config_version = ?", configVersion).Take(&record).Error; err != nil {
		return RuntimeConfigRecord{}, false
	}
	return record, true
}
