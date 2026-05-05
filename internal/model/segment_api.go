package model

// SegmentUploadedRequest 表示分片上传成功回传请求。
type SegmentUploadedRequest struct {
	SegmentID       uint64 `json:"segment_id"`
	ObjectETag      string `json:"object_etag"`
	ObjectSizeBytes uint64 `json:"object_size_bytes"`
}
