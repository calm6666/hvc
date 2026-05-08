package model

import "time"

// LiveChannel 表示直播频道。
type LiveChannel struct {
	ChannelID             uint64    `json:"channel_id"`
	ChannelKey            string    `json:"channel_key"`
	ChannelName           string    `json:"channel_name"`
	ProfileID             uint64    `json:"profile_id"`
	Status                string    `json:"status"`
	EnableSourceRendition bool      `json:"enable_source_rendition"`
	EnableWatermark       bool      `json:"enable_watermark"`
	PlayDomain            string    `json:"play_domain,omitempty"`
	PushDomain            string    `json:"push_domain,omitempty"`
	AssignedNodeID        uint64    `json:"assigned_node_id,omitempty"`
	AssignedWorkerID      string    `json:"assigned_worker_id,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// 直播频道状态常量。
const (
	LiveChannelStatusIdle     = "IDLE"
	LiveChannelStatusStarting = "STARTING"
	LiveChannelStatusLive     = "LIVE"
	LiveChannelStatusStopped  = "STOPPED"
	LiveChannelStatusError    = "ERROR"
)

// LiveSession 表示推流会话。
type LiveSession struct {
	SessionID        uint64     `json:"session_id"`
	ChannelID        uint64     `json:"channel_id"`
	ChannelKey       string     `json:"channel_key"`
	SessionKey       string     `json:"session_key,omitempty"`
	Status           string     `json:"status"`
	IngestURL        string     `json:"ingest_url,omitempty"`
	PlaybackHLSURL   string     `json:"playback_hls_url,omitempty"`
	PushProtocol     string     `json:"push_protocol"`
	AssignedNodeID   uint64     `json:"assigned_node_id,omitempty"`
	AssignedWorkerID string     `json:"assigned_worker_id,omitempty"`
	StartedAt        time.Time  `json:"started_at"`
	StoppedAt        *time.Time `json:"stopped_at,omitempty"`
	ResumeCount      int        `json:"resume_count"`
}

// 推流会话状态常量。
const (
	LiveSessionStatusConnecting          = "CONNECTING"
	LiveSessionStatusPublishing          = "PUBLISHING"
	LiveSessionStatusInterruptWaitResume = "INTERRUPT_WAIT_RESUME"
	LiveSessionStatusResumed             = "RESUMED"
	LiveSessionStatusStopped             = "STOPPED"
	LiveSessionStatusRejected            = "REJECTED"
)

// LivePlaybackInfo 表示直播播放信息。
type LivePlaybackInfo struct {
	ChannelKey     string                  `json:"channel_key"`
	Status         string                  `json:"status"`
	PlayToken      string                  `json:"play_token,omitempty"`
	ExpireAt       int64                   `json:"expire_at,omitempty"`
	MasterHLSURL   string                  `json:"master_hls_url,omitempty"`
	HTTPFLVURL     string                  `json:"http_flv_url,omitempty"`
	RenditionNames []string                `json:"rendition_names,omitempty"`
	Renditions     []LivePlaybackRendition `json:"renditions,omitempty"`
}

// LivePlaybackRendition 表示单个直播清晰度的播放地址集合。
type LivePlaybackRendition struct {
	RenditionName string `json:"rendition_name"`
	IsSource      bool   `json:"is_source"`
	HLSURL        string `json:"hls_url,omitempty"`
	HTTPFLVURL    string `json:"http_flv_url,omitempty"`
}
