package mysql

import (
	"context"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func toTranscodeRenditionRecord(item model.TranscodeRendition) TranscodeRenditionRecord {
	return TranscodeRenditionRecord{
		RenditionID:       item.RenditionID,
		JobID:             item.JobID,
		RenditionName:     item.RenditionName,
		RenditionKey:      item.RenditionKey,
		Status:            item.Status,
		OutWidth:          item.OutWidth,
		OutHeight:         item.OutHeight,
		VideoCodec:        item.VideoCodec,
		AudioCodec:        item.AudioCodec,
		VideoBitrateKbps:  item.VideoBitrateKbps,
		AudioBitrateKbps:  item.AudioBitrateKbps,
		SegmentCountVideo: item.SegmentCountVideo,
		SegmentCountAudio: item.SegmentCountAudio,
		ProgressPermille:  item.ProgressPermille,
		ErrorCode:         item.ErrorCode,
		ErrorMessage:      item.ErrorMessage,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toTranscodeRenditionModel(record TranscodeRenditionRecord) model.TranscodeRendition {
	return model.TranscodeRendition{
		RenditionID:       record.RenditionID,
		JobID:             record.JobID,
		RenditionName:     record.RenditionName,
		RenditionKey:      record.RenditionKey,
		Status:            record.Status,
		OutWidth:          record.OutWidth,
		OutHeight:         record.OutHeight,
		VideoCodec:        record.VideoCodec,
		AudioCodec:        record.AudioCodec,
		VideoBitrateKbps:  record.VideoBitrateKbps,
		AudioBitrateKbps:  record.AudioBitrateKbps,
		SegmentCountVideo: record.SegmentCountVideo,
		SegmentCountAudio: record.SegmentCountAudio,
		ProgressPermille:  record.ProgressPermille,
		ErrorCode:         record.ErrorCode,
		ErrorMessage:      record.ErrorMessage,
		CreatedAt:         record.CreatedAt,
		UpdatedAt:         record.UpdatedAt,
	}
}

// TranscodeRenditionRepository 管理任务下各清晰度子任务的持久化。
type TranscodeRenditionRepository struct {
	db *DB
}

func NewTranscodeRenditionRepository(db *DB) *TranscodeRenditionRepository {
	return &TranscodeRenditionRepository{db: db}
}

func (r *TranscodeRenditionRepository) ListByJobID(ctx context.Context, jobID uint64) []model.TranscodeRendition {
	var records []TranscodeRenditionRecord
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Order("created_at asc").Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.TranscodeRendition, 0, len(records))
	for _, record := range records {
		items = append(items, toTranscodeRenditionModel(record))
	}
	return items
}

// EnsureForJob 按任务和清晰度名称复用或创建子任务记录。
//
// 同一 job 下同名清晰度重试时不会重新生成 rendition_id / rendition_key，
// 从而保证分片路径和后续清单构建都保持稳定。
func (r *TranscodeRenditionRepository) EnsureForJob(ctx context.Context, desired []model.TranscodeRendition) ([]model.TranscodeRendition, error) {
	if len(desired) == 0 {
		return nil, nil
	}

	result := make([]model.TranscodeRendition, 0, len(desired))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		jobID := desired[0].JobID
		var existingRecords []TranscodeRenditionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("job_id = ?", jobID).
			Find(&existingRecords).Error; err != nil {
			return err
		}

		existingByName := make(map[string]TranscodeRenditionRecord, len(existingRecords))
		for _, record := range existingRecords {
			existingByName[record.RenditionName] = record
		}

		now := time.Now()
		for _, item := range desired {
			if current, ok := existingByName[item.RenditionName]; ok {
				updates := map[string]any{
					"status":             item.Status,
					"out_width":          item.OutWidth,
					"out_height":         item.OutHeight,
					"video_codec":        item.VideoCodec,
					"audio_codec":        item.AudioCodec,
					"video_bitrate_kbps": item.VideoBitrateKbps,
					"audio_bitrate_kbps": item.AudioBitrateKbps,
					"updated_at":         now,
				}
				if current.RenditionKey == "" && item.RenditionKey != "" {
					updates["rendition_key"] = item.RenditionKey
					current.RenditionKey = item.RenditionKey
				}
				if err := tx.Model(&TranscodeRenditionRecord{}).
					Where("rendition_id = ?", current.RenditionID).
					Updates(updates).Error; err != nil {
					return err
				}

				current.Status = item.Status
				current.OutWidth = item.OutWidth
				current.OutHeight = item.OutHeight
				current.VideoCodec = item.VideoCodec
				current.AudioCodec = item.AudioCodec
				current.VideoBitrateKbps = item.VideoBitrateKbps
				current.AudioBitrateKbps = item.AudioBitrateKbps
				current.UpdatedAt = now
				result = append(result, toTranscodeRenditionModel(current))
				continue
			}

			if item.RenditionID == 0 {
				item.RenditionID = idgen.Next()
			}
			if item.CreatedAt.IsZero() {
				item.CreatedAt = now
			}
			item.UpdatedAt = now

			record := toTranscodeRenditionRecord(item)
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			result = append(result, item)
		}
		return nil
	})
	return result, err
}
