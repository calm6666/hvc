package mysql

import (
	"context"
	"hvc/internal/model"
	"time"
)

func toSegmentRecord(segment model.Segment) SegmentRecord {
	return SegmentRecord{
		SegmentID:          segment.SegmentID,
		JobID:              segment.JobID,
		RenditionID:        segment.RenditionID,
		MediaType:          segment.MediaType,
		IsInitSegment:      segment.IsInitSegment,
		SequenceNo:         segment.SequenceNo,
		DurationMS:         segment.DurationMS,
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
		MediaType:          record.MediaType,
		IsInitSegment:      record.IsInitSegment,
		SequenceNo:         record.SequenceNo,
		DurationMS:         record.DurationMS,
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
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
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
func (r *SegmentRepository) ListPendingUpload(ctx context.Context, limit int) []model.Segment {
	var records []SegmentRecord
	query := r.db.WithContext(ctx).Where("upload_status IN ?", []int{model.SegmentUploadPending, model.SegmentUploadFailed}).Order("updated_at asc")
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

// MarkUploading 标记分片上传中。
func (r *SegmentRepository) MarkUploading(ctx context.Context, segmentID uint64) error {
	return r.db.WithContext(ctx).Model(&SegmentRecord{}).Where("segment_id = ?", segmentID).Updates(map[string]any{
		"upload_status": model.SegmentUploading,
		"updated_at":    time.Now(),
	}).Error
}

// MarkUploaded 标记分片上传成功。
func (r *SegmentRepository) MarkUploaded(ctx context.Context, segmentID uint64, objectETag string, objectSizeBytes uint64) error {
	return r.db.WithContext(ctx).Model(&SegmentRecord{}).Where("segment_id = ?", segmentID).Updates(map[string]any{
		"upload_status":       model.SegmentUploaded,
		"object_etag":         objectETag,
		"object_size_bytes":   objectSizeBytes,
		"upload_error_message": "",
		"updated_at":          time.Now(),
	}).Error
}

// MarkUploadFailed 标记分片上传失败。
func (r *SegmentRepository) MarkUploadFailed(ctx context.Context, segmentID uint64, message string) error {
	return r.db.WithContext(ctx).Model(&SegmentRecord{}).Where("segment_id = ?", segmentID).Updates(map[string]any{
		"upload_status":        model.SegmentUploadFailed,
		"upload_retry_count":   gormExprInc(),
		"upload_error_message": message,
		"updated_at":           time.Now(),
	}).Error
}

func gormExprInc() any {
	return gormExpr("upload_retry_count + 1")
}
