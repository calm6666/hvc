package model

import "time"

// LiveChannel 表示直播频道。
type LiveChannel struct {
	ChannelID             uint64    `json:"channel_id"`
	ChannelKey            string    `json:"channel_key"`
	ChannelName           string    `json:"channel_name"`
	ProfileID             uint64    `json:"profile_id"`
	Status                int       `json:"status"`
	EnableSourceRendition bool      `json:"enable_source_rendition"`
	EnableWatermark       bool      `json:"enable_watermark"`
	PlayDomain            string    `json:"play_domain,omitempty"`
	PushDomain            string    `json:"push_domain,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// LivePlaybackInfo 表示直播播放信息。
type LivePlaybackInfo struct {
	ChannelKey     string   `json:"channel_key"`
	Status         string   `json:"status"`
	MasterHLSURL   string   `json:"master_hls_url,omitempty"`
	HTTPFLVURL     string   `json:"http_flv_url,omitempty"`
	RenditionNames []string `json:"rendition_names,omitempty"`
}
