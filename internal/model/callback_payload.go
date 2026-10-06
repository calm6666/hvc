package model

import "encoding/json"

// TranscodeCompletedPayload 转码完成回调载荷。
//
// Worker 完成转码后写入 Outbox 的 PayloadJSON 字段，
// 回调投递时作为标准信封格式的 payload 字段发送给回调目标。
type TranscodeCompletedPayload struct {
	JobID             uint64               `json:"job_id"`
	RequestID         string               `json:"request_id"`
	BizKey            string               `json:"biz_key"`
	SourceURL         string               `json:"source_url"`
	Status            int                  `json:"status"`
	StatusName        string               `json:"status_name"`
	DurationMS        int64                `json:"duration_ms"`
	SegmentDuration   int                  `json:"segment_duration_sec"`
	SupportDash       bool                 `json:"support_dash"`
	SupportHLS        bool                 `json:"support_hls"`
	SegmentTemplate   string               `json:"segment_template"`
	StorageType       string               `json:"storage_type"`
	StorageBucket     string               `json:"storage_bucket"`
	PlayDomain        string               `json:"play_domain"`
	SourceInfo        SourceInfo           `json:"source_info"`
	Renditions        []CompletedRendition `json:"renditions"`
	TotalSegmentCount int                  `json:"total_segment_count"`
	TotalSizeBytes    uint64               `json:"total_size_bytes"`
}

// SourceInfo 源视频探测信息。
type SourceInfo struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	VideoCodec       string  `json:"video_codec"`
	VideoBitrateKbps int     `json:"video_bitrate_kbps"`
	AudioCodec       string  `json:"audio_codec"`
	AudioBitrateKbps int     `json:"audio_bitrate_kbps"`
	FPS              float64 `json:"fps"`
	DurationMS       int64   `json:"duration_ms"`
}

// CompletedSegmentDigest 单个分片的内容凭据（B3 清单物化）。
//
// 用途：A1 的判据是"回调送达 ⇒ 清单引用的每个分片都能 GET 到且 sha256 通过"。
// 回调载荷里带上每片的摘要，下游（与门禁）就能在拿到回调后逐片复算校验，而不必信任我们的自述。
// 摘要由切片产出时算好、随分片行落库（见 worker/segment_digest.go 与 verifyJobSegments）。
type CompletedSegmentDigest struct {
	ObjectKey string `json:"object_key"`
	SHA256    string `json:"sha256"`
	SizeBytes uint64 `json:"size_bytes"`
}

// CompletedRendition 转码完成的单个清晰度信息。
type CompletedRendition struct {
	RenditionName         string `json:"rendition_name"`
	RenditionKey          string `json:"rendition_key"`
	QualityLabel          string `json:"quality_label"`
	Width                 int    `json:"width"`
	Height                int    `json:"height"`
	VideoCodec            string `json:"video_codec"`
	VideoBitrateKbps      int    `json:"video_bitrate_kbps"`
	AudioBitrateKbps      int    `json:"audio_bitrate_kbps"`
	SegmentCount          int    `json:"segment_count"`
	InitSegmentObjectKey  string `json:"init_segment_object_key"`
	ManifestDashURL       string `json:"manifest_dash_url"`
	ManifestHLSURL        string `json:"manifest_hls_url"`
	ManifestHLSVariantURL string `json:"manifest_hls_variant_url"`
	// Segments 该清晰度下每个媒体分片的摘要（不含 init 段）；顺序与清单里出现的顺序一致。
	Segments []CompletedSegmentDigest `json:"segments,omitempty"`
}

// TranscodeFailedPayload 转码失败回调载荷。
type TranscodeFailedPayload struct {
	JobID        uint64 `json:"job_id"`
	RequestID    string `json:"request_id"`
	BizKey       string `json:"biz_key"`
	SourceURL    string `json:"source_url"`
	Status       int    `json:"status"`
	StatusName   string `json:"status_name"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	FailedStage  string `json:"failed_stage"`
	RetryCount   int    `json:"retry_count"`
}

// ToJSON 序列化为 JSON 字符串。
func (p TranscodeCompletedPayload) ToJSON() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// ToJSON 序列化为 JSON 字符串。
func (p TranscodeFailedPayload) ToJSON() string {
	b, _ := json.Marshal(p)
	return string(b)
}
