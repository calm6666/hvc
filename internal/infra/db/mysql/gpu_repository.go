package mysql

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"hvc/internal/model"
	"hvc/pkg/idgen"
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

// GPUNodeSummary 表示单节点 GPU 汇总结果。
type GPUNodeSummary struct {
	NodeID               uint64
	Total                int
	HealthyTotal         int
	SchedulableTotal     int
	MaxTranscodeSessions int
}

// NewGPUDeviceRepository 创建节点 GPU 设备仓储。
func NewGPUDeviceRepository(db *DB) *GPUDeviceRepository {
	return &GPUDeviceRepository{db: db}
}

// ListAll 返回全部 GPU 设备记录，用于后台概览聚合。
func (r *GPUDeviceRepository) ListAll(ctx context.Context) []NodeGPUDeviceRecord {
	var records []NodeGPUDeviceRecord
	if err := r.db.WithContext(ctx).Order("node_id asc, gpu_index asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListByNodeID 返回指定节点下的全部 GPU 设备记录。
func (r *GPUDeviceRepository) ListByNodeID(ctx context.Context, nodeID uint64) []NodeGPUDeviceRecord {
	var records []NodeGPUDeviceRecord
	if err := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("gpu_index asc").
		Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListByNodeIDs 按节点集合批量返回 GPU 设备记录。
//
// 后台分页节点列表只需要当前页节点的 GPU 主档，
// 没必要每次都把整张 t_node_gpu_device 全表拉回来。
func (r *GPUDeviceRepository) ListByNodeIDs(ctx context.Context, nodeIDs []uint64) []NodeGPUDeviceRecord {
	if len(nodeIDs) == 0 {
		return nil
	}
	uniqueNodeIDs := make([]uint64, 0, len(nodeIDs))
	seen := make(map[uint64]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if nodeID == 0 {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		uniqueNodeIDs = append(uniqueNodeIDs, nodeID)
	}
	if len(uniqueNodeIDs) == 0 {
		return nil
	}

	var records []NodeGPUDeviceRecord
	if err := r.db.WithContext(ctx).
		Where("node_id IN ?", uniqueNodeIDs).
		Order("node_id asc, gpu_index asc").
		Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// SummaryByNodeIDs 返回节点集合的 GPU 汇总信息。
//
// overview / realtime 只需要聚合数，不需要每张 GPU 的完整明细。
// 这里直接下推到数据库做 group by，避免在控制面高频轮询时构造整批 GPU 明细对象。
func (r *GPUDeviceRepository) SummaryByNodeIDs(ctx context.Context, nodeIDs []uint64) map[uint64]GPUNodeSummary {
	result := make(map[uint64]GPUNodeSummary)
	if len(nodeIDs) == 0 {
		return result
	}

	uniqueNodeIDs := make([]uint64, 0, len(nodeIDs))
	seen := make(map[uint64]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if nodeID == 0 {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		uniqueNodeIDs = append(uniqueNodeIDs, nodeID)
	}
	if len(uniqueNodeIDs) == 0 {
		return result
	}

	type summaryRow struct {
		NodeID               uint64 `gorm:"column:node_id"`
		Total                int64  `gorm:"column:total"`
		HealthyTotal         int64  `gorm:"column:healthy_total"`
		SchedulableTotal     int64  `gorm:"column:schedulable_total"`
		MaxTranscodeSessions int64  `gorm:"column:max_transcode_sessions"`
	}

	var rows []summaryRow
	if err := r.db.WithContext(ctx).
		Model(&NodeGPUDeviceRecord{}).
		Select("node_id, COUNT(*) AS total, SUM(CASE WHEN healthy THEN 1 ELSE 0 END) AS healthy_total, SUM(CASE WHEN schedulable THEN 1 ELSE 0 END) AS schedulable_total, SUM(max_transcode_sessions) AS max_transcode_sessions").
		Where("node_id IN ?", uniqueNodeIDs).
		Group("node_id").
		Find(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.NodeID] = GPUNodeSummary{
			NodeID:               row.NodeID,
			Total:                int(row.Total),
			HealthyTotal:         int(row.HealthyTotal),
			SchedulableTotal:     int(row.SchedulableTotal),
			MaxTranscodeSessions: int(row.MaxTranscodeSessions),
		}
	}
	return result
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
		result := query.Where("node_id = ? AND gpu_uuid = ?", nodeID, capability.GPUUUID).Limit(1).Find(&existing)
		if result.Error != nil {
			return 0, result.Error
		}
		if result.RowsAffected > 0 {
			return existing.GPUDeviceID, r.db.WithContext(ctx).Model(&NodeGPUDeviceRecord{}).
				Where("gpu_device_id = ?", existing.GPUDeviceID).
				Updates(map[string]any{
					"gpu_index":              capability.GPUIndex,
					"gpu_uuid":               capability.GPUUUID,
					"vendor":                 capability.Vendor,
					"model":                  capability.Model,
					"driver_version":         capability.DriverVersion,
					"memory_total_mb":        capability.MemoryTotalMB,
					"max_transcode_sessions": capability.MaxSessions,
					"last_seen_at":           now,
					"updated_at":             now,
				}).Error
		}
	}
	result := query.Where("node_id = ? AND gpu_index = ?", nodeID, capability.GPUIndex).Limit(1).Find(&existing)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		return existing.GPUDeviceID, r.db.WithContext(ctx).Model(&NodeGPUDeviceRecord{}).
			Where("gpu_device_id = ?", existing.GPUDeviceID).
			Updates(map[string]any{
				"gpu_uuid":               capability.GPUUUID,
				"gpu_index":              capability.GPUIndex,
				"vendor":                 capability.Vendor,
				"model":                  capability.Model,
				"driver_version":         capability.DriverVersion,
				"memory_total_mb":        capability.MemoryTotalMB,
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
		Vendor:               capability.Vendor,
		Model:                capability.Model,
		DriverVersion:        capability.DriverVersion,
		MemoryTotalMB:        capability.MemoryTotalMB,
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
	result := r.db.WithContext(ctx).Where("node_id = ? AND gpu_index = ?", nodeID, gpuIndex).Limit(1).Find(&record)
	if result.Error != nil || result.RowsAffected == 0 {
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
func (r *WorkerCodecCapabilityRepository) ReplaceLatestSnapshot(ctx context.Context, nodeID uint64, workerInstanceID uint64, startupInstanceID string, machineFingerprint string, probeGeneration uint64, capabilities []model.GPUCapability) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&WorkerCodecCapabilityRecord{}).
			Where("node_id = ? AND is_latest = ?", nodeID, true).
			Updates(map[string]any{"is_latest": false}).Error; err != nil {
			return err
		}
		for _, capability := range capabilities {
			payloadJSON := marshalCapabilityPayload(capability)
			record := WorkerCodecCapabilityRecord{
				NodeID:                nodeID,
				WorkerInstanceID:      workerInstanceID,
				GPUDeviceID:           capability.GPUDeviceID,
				GPUIndex:              capability.GPUIndex,
				GPUUUID:               capability.GPUUUID,
				StartupInstanceID:     startupInstanceID,
				ProbeGeneration:       probeGeneration,
				MachineFingerprint:    machineFingerprint,
				HWType:                mapHWType(capability.ExecutionHWTypes),
				MaxSessions:           capability.MaxSessions,
				Enabled:               true,
				IsLatest:              true,
				CapabilityPayloadJSON: payloadJSON,
				CollectedAt:           now,
				CreatedAt:             now,
			}
			for _, codec := range capability.EncodeCodecs {
				item := record
				item.ID = idgen.Next()
				item.CodecName = codec
				item.CapType = 2
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error; err != nil {
					return err
				}
			}
			for _, codec := range capability.DecodeCodecs {
				item := record
				item.ID = idgen.Next()
				item.CodecName = codec
				item.CapType = 1
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func marshalCapabilityPayload(capability model.GPUCapability) *string {
	payload, err := json.Marshal(capability)
	if err != nil {
		return nil
	}
	result := string(payload)
	return &result
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
