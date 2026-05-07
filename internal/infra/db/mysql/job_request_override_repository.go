package mysql

import (
	"context"

	"hvc/internal/model"
)

func toJobRequestOverrideRecord(item model.TranscodeJobRequestOverride) TranscodeJobRequestOverrideRecord {
	return TranscodeJobRequestOverrideRecord{
		ID:                               item.ID,
		JobID:                            item.JobID,
		OverrideProfileID:                item.OverrideProfileID,
		OverrideSegmentDurationSec:       item.OverrideSegmentDurationSec,
		OverridePreferredHWAccel:         item.OverridePreferredHWAccel,
		OverrideEnableWatermark:          item.OverrideEnableWatermark,
		OverrideWatermarkImageURL:        item.OverrideWatermarkImageURL,
		OverrideWatermarkAnchor:          item.OverrideWatermarkAnchor,
		OverrideWatermarkXRatio:          item.OverrideWatermarkXRatio,
		OverrideWatermarkYRatio:          item.OverrideWatermarkYRatio,
		OverrideWatermarkWidthRatio:      item.OverrideWatermarkWidthRatio,
		OverrideWatermarkOpacity:         item.OverrideWatermarkOpacity,
		OverrideEnableThumbnailSprite:    item.OverrideEnableThumbnailSprite,
		OverrideThumbRows:                item.OverrideThumbRows,
		OverrideThumbCols:                item.OverrideThumbCols,
		OverrideThumbIntervalSec:         item.OverrideThumbIntervalSec,
		OverrideThumbWidth:               item.OverrideThumbWidth,
		OverrideThumbHeight:              item.OverrideThumbHeight,
		OverrideThumbImageFormat:         item.OverrideThumbImageFormat,
		OverrideThumbStoragePrefix:       item.OverrideThumbStoragePrefix,
		OverrideEnableThumbnailBinaryIdx: item.OverrideEnableThumbnailBinaryIdx,
		OverrideThumbBinaryStoragePrefix: item.OverrideThumbBinaryStoragePrefix,
		OverrideThumbBinaryMaxSizeBytes:  item.OverrideThumbBinaryMaxSizeBytes,
		OverrideBucketPrefix:             item.OverrideBucketPrefix,
		OverrideSegmentPrefix:            item.OverrideSegmentPrefix,
		CreatedAt:                        item.CreatedAt,
	}
}

// JobRequestOverrideRepository 表示请求覆盖参数仓储。
//
// 这层负责把 t_transcode_job_request_override 真正落到数据库，
// 让创建任务时的覆盖参数和后续调度/执行产生的结果分层保存。
type JobRequestOverrideRepository struct {
	db *DB
}

func toJobRequestOverrideModel(record TranscodeJobRequestOverrideRecord) model.TranscodeJobRequestOverride {
	return model.TranscodeJobRequestOverride{
		ID:                               record.ID,
		JobID:                            record.JobID,
		OverrideProfileID:                record.OverrideProfileID,
		OverrideSegmentDurationSec:       record.OverrideSegmentDurationSec,
		OverridePreferredHWAccel:         record.OverridePreferredHWAccel,
		OverrideEnableWatermark:          record.OverrideEnableWatermark,
		OverrideWatermarkImageURL:        record.OverrideWatermarkImageURL,
		OverrideWatermarkAnchor:          record.OverrideWatermarkAnchor,
		OverrideWatermarkXRatio:          record.OverrideWatermarkXRatio,
		OverrideWatermarkYRatio:          record.OverrideWatermarkYRatio,
		OverrideWatermarkWidthRatio:      record.OverrideWatermarkWidthRatio,
		OverrideWatermarkOpacity:         record.OverrideWatermarkOpacity,
		OverrideEnableThumbnailSprite:    record.OverrideEnableThumbnailSprite,
		OverrideThumbRows:                record.OverrideThumbRows,
		OverrideThumbCols:                record.OverrideThumbCols,
		OverrideThumbIntervalSec:         record.OverrideThumbIntervalSec,
		OverrideThumbWidth:               record.OverrideThumbWidth,
		OverrideThumbHeight:              record.OverrideThumbHeight,
		OverrideThumbImageFormat:         record.OverrideThumbImageFormat,
		OverrideThumbStoragePrefix:       record.OverrideThumbStoragePrefix,
		OverrideEnableThumbnailBinaryIdx: record.OverrideEnableThumbnailBinaryIdx,
		OverrideThumbBinaryStoragePrefix: record.OverrideThumbBinaryStoragePrefix,
		OverrideThumbBinaryMaxSizeBytes:  record.OverrideThumbBinaryMaxSizeBytes,
		OverrideBucketPrefix:             record.OverrideBucketPrefix,
		OverrideSegmentPrefix:            record.OverrideSegmentPrefix,
		CreatedAt:                        record.CreatedAt,
	}
}

// NewJobRequestOverrideRepository 创建请求覆盖参数仓储。
func NewJobRequestOverrideRepository(db *DB) *JobRequestOverrideRepository {
	return &JobRequestOverrideRepository{db: db}
}

// Save 保存单任务覆盖参数。
func (r *JobRequestOverrideRepository) Save(ctx context.Context, item model.TranscodeJobRequestOverride) error {
	record := toJobRequestOverrideRecord(item)
	return r.db.WithContext(ctx).Create(&record).Error
}

// FindByJobID 根据 job_id 查询单任务覆盖参数。
func (r *JobRequestOverrideRepository) FindByJobID(ctx context.Context, jobID uint64) (model.TranscodeJobRequestOverride, bool) {
	var record TranscodeJobRequestOverrideRecord
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Take(&record).Error; err != nil {
		return model.TranscodeJobRequestOverride{}, false
	}
	return toJobRequestOverrideModel(record), true
}
