package mysql

import (
	"context"
	"errors"
	"hvc/internal/model"
	"time"

	"hvc/pkg/idgen"

	"gorm.io/gorm/clause"
)

var ErrSegmentNotFound = errors.New("segment not found")

func toSegmentRecord(segment model.Segment) SegmentRecord {
	if segment.SegmentID == 0 {
		segment.SegmentID = idgen.Next()
	}
	return SegmentRecord{
		SegmentID:          segment.SegmentID,
		JobID:              segment.JobID,
		RenditionID:        segment.RenditionID,
		RenditionName:      segment.RenditionName,
		RenditionKey:       segment.RenditionKey,
		MediaType:          segment.MediaType,
		IsInitSegment:      segment.IsInitSegment,
		SequenceNo:         segment.SequenceNo,
		DurationMS:         segment.DurationMS,
		Width:              segment.Width,
		Height:             segment.Height,
		VideoBitrateKbps:   segment.VideoBitrateKbps,
		AudioBitrateKbps:   segment.AudioBitrateKbps,
		VideoCodec:         segment.VideoCodec,
		AudioCodec:         segment.AudioCodec,
		SupportDash:        segment.SupportDash,
		SupportHLS:         segment.SupportHLS,
		CodecName:          segment.CodecName,
		ObjectKey:          segment.ObjectKey,
		ObjectSizeBytes:    segment.ObjectSizeBytes,
		ObjectETag:         segment.ObjectETag,
		SHA256:             segment.SHA256,
		StartPTSMS:         segment.StartPTSMS,
		EndPTSMS:           segment.EndPTSMS,
		UploadStatus:       segment.UploadStatus,
		UploadRetryCount:   segment.UploadRetryCount,
		UploadErrorMessage: segment.UploadErrorMessage,
		CreatedAt:          segment.CreatedAt,
		UpdatedAt:          segment.UpdatedAt,
	}
}

func toSegmentModel(record SegmentRecord) model.Segment {
	return model.Segment{
		SegmentID:          record.SegmentID,
		JobID:              record.JobID,
		RenditionID:        record.RenditionID,
		RenditionName:      record.RenditionName,
		RenditionKey:       record.RenditionKey,
		MediaType:          record.MediaType,
		IsInitSegment:      record.IsInitSegment,
		SequenceNo:         record.SequenceNo,
		DurationMS:         record.DurationMS,
		Width:              record.Width,
		Height:             record.Height,
		VideoBitrateKbps:   record.VideoBitrateKbps,
		AudioBitrateKbps:   record.AudioBitrateKbps,
		VideoCodec:         record.VideoCodec,
		AudioCodec:         record.AudioCodec,
		SupportDash:        record.SupportDash,
		SupportHLS:         record.SupportHLS,
		CodecName:          record.CodecName,
		ObjectKey:          record.ObjectKey,
		ObjectSizeBytes:    record.ObjectSizeBytes,
		ObjectETag:         record.ObjectETag,
		SHA256:             record.SHA256,
		StartPTSMS:         record.StartPTSMS,
		EndPTSMS:           record.EndPTSMS,
		UploadStatus:       record.UploadStatus,
		UploadRetryCount:   record.UploadRetryCount,
		UploadErrorMessage: record.UploadErrorMessage,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

// SegmentRepository 表示分片仓储。
type SegmentRepository struct {
	db *DB
}

// NewSegmentRepository 创建分片仓储。
func NewSegmentRepository(db *DB) *SegmentRepository {
	return &SegmentRepository{db: db}
}

// Save 保存分片元数据。
func (r *SegmentRepository) Save(ctx context.Context, segment model.Segment) (model.Segment, error) {
	record := toSegmentRecord(segment)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "rendition_id"},
			{Name: "media_type"},
			{Name: "sequence_no"},
			{Name: "is_init_segment"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"job_id":               record.JobID,
			"rendition_name":       record.RenditionName,
			"rendition_key":        record.RenditionKey,
			"duration_ms":          record.DurationMS,
			"width":                record.Width,
			"height":               record.Height,
			"video_bitrate_kbps":   record.VideoBitrateKbps,
			"audio_bitrate_kbps":   record.AudioBitrateKbps,
			"video_codec":          record.VideoCodec,
			"audio_codec":          record.AudioCodec,
			"support_dash":         record.SupportDash,
			"support_hls":          record.SupportHLS,
			"codec_name":           record.CodecName,
			"object_key":           record.ObjectKey,
			"object_size_bytes":    record.ObjectSizeBytes,
			"object_etag":          record.ObjectETag,
			"sha256":               record.SHA256,
			"start_pts_ms":         record.StartPTSMS,
			"end_pts_ms":           record.EndPTSMS,
			"upload_status":        record.UploadStatus,
			"upload_error_message": record.UploadErrorMessage,
			"updated_at":           record.UpdatedAt,
		}),
	}).Create(&record).Error; err != nil {
		return model.Segment{}, err
	}
	segment.SegmentID = record.SegmentID
	return segment, nil
}

// ListByJobID 返回任务分片列表。
func (r *SegmentRepository) ListByJobID(ctx context.Context, jobID uint64) []model.Segment {
	var records []SegmentRecord
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.Segment, 0, len(records))
	for _, record := range records {
		items = append(items, toSegmentModel(record))
	}
	return items
}

// ListPendingUpload 返回待上传分片列表。
func (r *SegmentRepository) ListPendingUpload(ctx context.Context, limit int, maxRetry int) []model.Segment {
	var records []SegmentRecord
	if maxRetry <= 0 {
		maxRetry = 1
	}
	query := r.db.WithContext(ctx).
		Where("upload_status = ? OR (upload_status = ? AND upload_retry_count < ?)", model.SegmentUploadPending, model.SegmentUploadFailed, maxRetry).
		Order("updated_at asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.Segment, 0, len(records))
	for _, record := range records {
		items = append(items, toSegmentModel(record))
	}
	return items
}

// CountPendingUpload 返回待上传分片数量。
//
// Worker 周期性上报 metrics 时只关心“当前上传队列深度”，
// 不需要把一批分片记录拉回进程再 len()。这里直接走数据库计数。
// CountPendingUploadByJob 返回**某个任务**还没传完的分片数（A1）。
//
// 为什么不能用上面的 CountPendingUpload：那个是**全局**队列深度，用来判断"这个任务传完没有"
// 一定是错的 —— 队列里还躺着别的任务的分片，它会永远 > 0，任务也就永远发布不出去。
func (r *SegmentRepository) CountPendingUploadByJob(ctx context.Context, jobID uint64, maxRetry int) int {
	if maxRetry <= 0 {
		maxRetry = 1
	}
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&SegmentRecord{}).
		Where("job_id = ?", jobID).
		Where("upload_status = ? OR (upload_status = ? AND upload_retry_count < ?)",
			model.SegmentUploadPending, model.SegmentUploadFailed, maxRetry).
		Count(&total).Error; err != nil {
		// 查不出来时**当成"还没传完"**：宁可暂时不发布，也不能在不确定的情况下发回调。
		return 1
	}
	if total < 0 {
		return 1
	}
	return int(total)
}
func (r *SegmentRepository) CountPendingUpload(ctx context.Context, maxRetry int) int {
	if maxRetry <= 0 {
		maxRetry = 1
	}
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&SegmentRecord{}).
		Where("upload_status = ? OR (upload_status = ? AND upload_retry_count < ?)", model.SegmentUploadPending, model.SegmentUploadFailed, maxRetry).
		Count(&total).Error; err != nil {
		return 0
	}
	if total < 0 {
		return 0
	}
	return int(total)
}

// MarkUploading 标记分片上传中。
func (r *SegmentRepository) MarkUploading(ctx context.Context, segmentID uint64) error {
	result := r.db.WithContext(ctx).Model(&SegmentRecord{}).Where("segment_id = ?", segmentID).Updates(map[string]any{
		"upload_status": model.SegmentUploading,
		"updated_at":    time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSegmentNotFound
	}
	return nil
}

// MarkUploaded 标记分片上传成功。
func (r *SegmentRepository) MarkUploaded(ctx context.Context, segmentID uint64, objectETag string, objectSizeBytes uint64) error {
	result := r.db.WithContext(ctx).Model(&SegmentRecord{}).Where("segment_id = ?", segmentID).Updates(map[string]any{
		"upload_status":        model.SegmentUploaded,
		"object_etag":          objectETag,
		"object_size_bytes":    objectSizeBytes,
		"upload_error_message": "",
		"updated_at":           time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSegmentNotFound
	}
	return nil
}

// MarkUploadFailed 标记分片上传失败。
func (r *SegmentRepository) MarkUploadFailed(ctx context.Context, segmentID uint64, message string) error {
	result := r.db.WithContext(ctx).Model(&SegmentRecord{}).Where("segment_id = ?", segmentID).Updates(map[string]any{
		"upload_status":        model.SegmentUploadFailed,
		"upload_retry_count":   gormExprInc(),
		"upload_error_message": message,
		"updated_at":           time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSegmentNotFound
	}
	return nil
}

func gormExprInc() any {
	return gormExpr("upload_retry_count + 1")
}
