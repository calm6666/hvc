package mysql

import (
	"context"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GPUDeviceRepository 表示节点 GPU 设备仓储。
//
// 这层的职责很单一：
// 1. 根据 node_id + gpu_uuid 或 node_id + gpu_index 为 GPU 建立稳定设备身份；
// 2. 让调度器和 Worker 后续都尽量消费 gpu_device_id，而不是继续用 index 当永久标识；
// 3. 只处理设备主档，不承担 codec 能力快照职责。
type GPUDeviceRepository struct {
	db *DB
}

// NewGPUDeviceRepository 创建节点 GPU 设备仓储。
func NewGPUDeviceRepository(db *DB) *GPUDeviceRepository {
	return &GPUDeviceRepository{db: db}
}

// SaveOrUpdateByCapability 根据能力信息创建或更新稳定 GPU 设备记录。
//
// 这里优先使用 gpu_uuid 作为稳定匹配键；如果当前探测还拿不到真实 UUID，
// 则退化为 node_id + gpu_index 的匹配方式，以保证同一节点上的同一索引设备至少能形成稳定记录。
func (r *GPUDeviceRepository) SaveOrUpdateByCapability(ctx context.Context, nodeID uint64, capability model.GPUCapability) (uint64, error) {
	now := time.Now()
	query := r.db.WithContext(ctx).Model(&NodeGPUDeviceRecord{})
	var existing NodeGPUDeviceRecord
	if capability.GPUUUID != "" {
		if err := query.Where("node_id = ? AND gpu_uuid = ?", nodeID, capability.GPUUUID).Take(&existing).Error; err == nil {
			return existing.GPUDeviceID, r.db.WithContext(ctx).Model(&NodeGPUDeviceRecord{}).
				Where("gpu_device_id = ?", existing.GPUDeviceID).
				Updates(map[string]any{
					"gpu_index":              capability.GPUIndex,
					"gpu_uuid":               capability.GPUUUID,
					"max_transcode_sessions": capability.MaxSessions,
					"last_seen_at":           now,
					"updated_at":             now,
				}).Error
		}
	}
	if err := query.Where("node_id = ? AND gpu_index = ?", nodeID, capability.GPUIndex).Take(&existing).Error; err == nil {
		return existing.GPUDeviceID, r.db.WithContext(ctx).Model(&NodeGPUDeviceRecord{}).
			Where("gpu_device_id = ?", existing.GPUDeviceID).
			Updates(map[string]any{
				"gpu_uuid":               capability.GPUUUID,
				"gpu_index":              capability.GPUIndex,
				"max_transcode_sessions": capability.MaxSessions,
				"last_seen_at":           now,
				"updated_at":             now,
			}).Error
	}
	record := NodeGPUDeviceRecord{
		GPUDeviceID:          idgen.Next(),
		NodeID:               nodeID,
		GPUIndex:             capability.GPUIndex,
		GPUUUID:              capability.GPUUUID,
		MemoryTotalMB:        0,
		MaxTranscodeSessions: capability.MaxSessions,
		Healthy:              true,
		Schedulable:          true,
		LastSeenAt:           now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return 0, err
	}
	return record.GPUDeviceID, nil
}

// FindByNodeAndIndex 根据节点和当前 GPU 索引查询设备记录。
func (r *GPUDeviceRepository) FindByNodeAndIndex(ctx context.Context, nodeID uint64, gpuIndex int) (NodeGPUDeviceRecord, bool) {
	var record NodeGPUDeviceRecord
	if err := r.db.WithContext(ctx).Where("node_id = ? AND gpu_index = ?", nodeID, gpuIndex).Take(&record).Error; err != nil {
		return NodeGPUDeviceRecord{}, false
	}
	return record, true
}

// WorkerCodecCapabilityRepository 表示 Worker 编解码能力快照仓储。
//
// 这里保存的是“某次启动、某一代 probe 的最新能力快照”。
// 当前实现采用“同一节点当前代次先整体标旧，再写新快照”的最小策略，先保证：
// 1. 数据有稳定落点；
// 2. 最新快照可追踪；
// 3. 调度层未来可以切到数据库消费而不是只有 Redis 热路径。
type WorkerCodecCapabilityRepository struct {
	db *DB
}

// NewWorkerCodecCapabilityRepository 创建 Worker 能力快照仓储。
func NewWorkerCodecCapabilityRepository(db *DB) *WorkerCodecCapabilityRepository {
	return &WorkerCodecCapabilityRepository{db: db}
}

// ReplaceLatestSnapshot 用当前能力集合替换该节点的一批最新能力快照。
func (r *WorkerCodecCapabilityRepository) ReplaceLatestSnapshot(ctx context.Context, nodeID uint64, startupInstanceID string, probeGeneration uint64, capabilities []model.GPUCapability) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&WorkerCodecCapabilityRecord{}).
			Where("node_id = ? AND is_latest = ?", nodeID, true).
			Updates(map[string]any{"is_latest": false}).Error; err != nil {
			return err
		}
		for _, capability := range capabilities {
			for _, codec := range capability.EncodeCodecs {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&WorkerCodecCapabilityRecord{
					ID:                idgen.Next(),
					NodeID:            nodeID,
					GPUDeviceID:       capability.GPUDeviceID,
					GPUIndex:          capability.GPUIndex,
					GPUUUID:           capability.GPUUUID,
					StartupInstanceID: startupInstanceID,
					ProbeGeneration:   probeGeneration,
					CodecName:         codec,
					CapType:           2,
					HWType:            mapHWType(capability.ExecutionHWTypes),
					MaxSessions:       capability.MaxSessions,
					Enabled:           true,
					IsLatest:          true,
					CollectedAt:       now,
					CreatedAt:         now,
				}).Error; err != nil {
					return err
				}
			}
			for _, codec := range capability.DecodeCodecs {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&WorkerCodecCapabilityRecord{
					ID:                idgen.Next(),
					NodeID:            nodeID,
					GPUDeviceID:       capability.GPUDeviceID,
					GPUIndex:          capability.GPUIndex,
					GPUUUID:           capability.GPUUUID,
					StartupInstanceID: startupInstanceID,
					ProbeGeneration:   probeGeneration,
					CodecName:         codec,
					CapType:           1,
					HWType:            mapHWType(capability.ExecutionHWTypes),
					MaxSessions:       capability.MaxSessions,
					Enabled:           true,
					IsLatest:          true,
					CollectedAt:       now,
					CreatedAt:         now,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func mapHWType(hwTypes []string) int {
	for _, hwType := range hwTypes {
		switch hwType {
		case model.ExecutionHWNVIDIA:
			return 1
		case model.ExecutionHWIntelQSV:
			return 2
		case model.ExecutionHWAMDAMF:
			return 3
		case model.ExecutionHWVAAPI:
			return 4
		case model.ExecutionHWAppleVideoToolbox:
			return 5
		}
	}
	return 0
}
