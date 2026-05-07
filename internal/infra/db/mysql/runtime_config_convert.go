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
	return config.DynamicRuntimeConfig{
		Mode: config.ModeConfig{
			EnableHTTPServer: record.EnableHTTPServer,
			EnableGRPCServer: record.RPCCallbackReceiverEnabled,
			EnableMQConsumer: record.MQQueueName != "",
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
			SegmentTemplate:            "{job_id}-{media_type}-{number}.m4s",
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
			BasePrefix:  record.WorkerObjectPrefix,
			StorageType: "s3",
		},
		GRPC: config.GRPCConfig{
			ListenAddress:     grpcListenAddress(record),
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
	}
}

func grpcListenAddress(record RuntimeConfigRecord) string {
	host := record.RPCCallbackReceiverHost
	if host == "" {
		host = "0.0.0.0"
	}
	port := record.RPCCallbackReceiverPort
	if port <= 0 {
		port = 9090
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// NewBootstrapRuntimeConfigRecord 生成首启时落库的已发布 runtime config。
func NewBootstrapRuntimeConfigRecord(cfg config.DynamicRuntimeConfig, source string) RuntimeConfigRecord {
	now := time.Now()
	host, port := splitBootstrapListenAddress(cfg.GRPC.ListenAddress)
	queueName := cfg.MQ.QueueName
	if !cfg.Mode.EnableMQConsumer {
		queueName = ""
	}

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
		RPCCallbackReceiverEnabled:       cfg.Mode.EnableGRPCServer,
		RPCCallbackReceiverHost:          host,
		RPCCallbackReceiverPort:          port,
		RPCCallbackReceiverRegistryID:    0,
		MQQueueName:                      queueName,
		MQHost:                           cfg.MQ.Host,
		MQPort:                           cfg.MQ.Port,
		MQUsername:                       cfg.MQ.Username,
		MQPassword:                       cfg.MQ.Password,
		MQVHost:                          cfg.MQ.VHost,
		MQConsumerTag:                    cfg.MQ.ConsumerTag,
		MQPrefetchCount:                  cfg.MQ.PrefetchCount,
		SchedulerWorkerID:                "worker-default",
		WorkerOutputPath:                 "",
		WorkerObjectPrefix:               cfg.Storage.BasePrefix,
		ChangeSummary:                    "bootstrap initial runtime config",
		ConfigSource:                     source,
		SourceRevision:                   "",
		PublishedBy:                      "bootstrap",
		PublishedAt:                      now,
		EffectiveConfigHash:              "",
		CreatedAt:                        now,
		UpdatedAt:                        now,
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
