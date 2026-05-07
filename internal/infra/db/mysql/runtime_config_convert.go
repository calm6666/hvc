package mysql

import (
	"context"
	"time"

	"hvc/internal/config"
)

// ToDynamicRuntimeConfig 把数据库运行配置记录转换为当前进程消费的动态运行配置。
//
// 第一阶段后台能力的关键点不是字段抽象有多优雅，而是管理员改完配置后，
// 系统真的能把配置记录转成 DynamicRuntimeConfig 并替换到生效容器里。
func ToDynamicRuntimeConfig(record RuntimeConfigRecord) config.DynamicRuntimeConfig {
	return config.DynamicRuntimeConfig{
		Mode: config.ModeConfig{
			EnableHTTPServer: true,
			EnableGRPCServer: false,
			EnableMQConsumer: false,
			EnableScheduler:  true,
			EnableWorker:     true,
			EnableCallback:   true,
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
			MaxNodeTranscodeSessions:     0,
			MaxNodeUploadConcurrency:     record.SingleJobUploadConcurrencyLimit,
			DynamicConcurrencyControl:    record.DynamicConcurrencyControlEnabled,
		},
		Worker: config.WorkerConfig{
			LoopInterval:               time.Duration(record.WorkerLoopIntervalMS) * time.Millisecond,
			SingleJobUploadConcurrency: record.SingleJobUploadConcurrencyLimit,
			UploadMaxRetryCount:        3,
		},
		Callback: config.CallbackConfig{
			HTTPURL:     record.CallbackHTTPURL,
			HTTPTimeout:  3 * time.Second,
			GRPCTimeout:  3 * time.Second,
			MQTimeout:    3 * time.Second,
			RetryBackoff: 2 * time.Second,
		},
		Storage: config.StorageConfig{
			BasePrefix: record.WorkerObjectPrefix,
		},
	}
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
