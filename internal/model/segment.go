package model

import "time"

// 分片上传状态常量，表示分片从创建到上传完成的完整状态。
const (
	SegmentUploadPending = 1 // 待上传
	SegmentUploading     = 2 // 上传中
	SegmentUploaded      = 3 // 已上传
	SegmentUploadFailed  = 4 // 上传失败
)

// Segment 表示分片元数据。
type Segment struct {
	SegmentID          uint64
	JobID              uint64
	RenditionID        uint64
	RenditionName      string
	RenditionKey       string
	SegmentType        string
	MediaType          int
	IsInitSegment      bool
	SequenceNo         int
	DurationMS         int
	Width              int
	Height             int
	VideoBitrateKbps   int
	AudioBitrateKbps   int
	VideoCodec         string
	AudioCodec         string
	SupportDash        bool
	SupportHLS         bool
	CodecName          string
	ObjectKey          string
	ObjectSizeBytes    uint64
	ObjectETag         string
	SHA256             string
	StartPTSMS         int64
	EndPTSMS           int64
	UploadStatus       int
	UploadRetryCount   int
	UploadErrorMessage string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
