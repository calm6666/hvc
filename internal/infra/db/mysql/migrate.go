package mysql

import "context"

// AutoMigrate 执行数据库模型迁移。
//
// 这里统一注册当前代码已经补齐的主要 GORM record，目的是保证：
// 1. 主初始化 SQL 中的核心表都有明确的 Go 结构体落点；
// 2. 增量 schema 中新增的配置中心/能力快照表也能被 GORM 感知；
// 3. 后续继续补仓储实现时，不会出现“结构体已存在但迁移链路完全没纳入”的失配。
func (db *DB) AutoMigrate(ctx context.Context) error {
	return db.WithContext(ctx).AutoMigrate(
		&AdminUserRecord{},
		&AdminRoleRecord{},
		&AdminPermissionRecord{},
		&AdminUserRoleRecord{},
		&AdminRolePermissionRecord{},
		&AdminSessionRecord{},
		&AdminAuditLogRecord{},
		&TranscodeJobRequestOverrideRecord{},
		&TranscodeProfileRecord{},
		&TranscodeProfileRenditionRecord{},
		&StorageConfigRecord{},
		&TranscodeRenditionRecord{},
		&JobRecord{},
		&SegmentRecord{},
		&TranscodeThumbnailSpriteRecord{},
		&TranscodeThumbnailBinRecord{},
		&TranscodeThumbnailItemRecord{},
		&CallbackConfigRecord{},
		&RegistryEtcdConfigRecord{},
		&OutboxRecord{},
		&DeliveryFailureQueueRecord{},
		&RuntimeConfigRecord{},
		&ConfigCenterBindingRecord{},
		&EffectiveRuntimeConfigSnapshotRecord{},
		&ClusterNodeRecord{},
		&NodeGPUDeviceRecord{},
		&WorkerInstanceRecord{},
		&WorkerCodecCapabilityRecord{},
		&JobExecutionRecord{},
		&LiveChannelRecord{},
		&LiveProfileRenditionRecord{},
		&LiveSessionRecord{},
		&LiveSessionEventRecord{},
		&LivePlaybackTokenRecord{},
		&LivePublishSessionRecord{},
		&LivePublishAuthLogRecord{},
	)
}
