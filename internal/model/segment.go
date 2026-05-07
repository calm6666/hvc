package model

import "time"

const (
	SegmentUploadPending   = 1
	SegmentUploading       = 2
	SegmentUploaded        = 3
	SegmentUploadFailed    = 4
)

// Segment 表示分片元数据。
type Segment struct {
	SegmentID          uint64
	JobID              uint64
	RenditionID        uint64
	RenditionName      string
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
