package mysql

import "time"

// LiveChannelRecord 表示直播频道表映射。
type LiveChannelRecord struct {
	ChannelID             uint64    `gorm:"column:channel_id;primaryKey"`
	ChannelKey            string    `gorm:"column:channel_key"`
	ChannelName           string    `gorm:"column:channel_name"`
	ProfileID             uint64    `gorm:"column:profile_id"`
	Status                int       `gorm:"column:status"`
	PlayDomain            string    `gorm:"column:play_domain"`
	PushDomain            string    `gorm:"column:push_domain"`
	EnableSourceRendition bool      `gorm:"column:enable_source_rendition"`
	EnableWatermark       bool      `gorm:"column:enable_watermark"`
	AssignedNodeID        uint64    `gorm:"column:assigned_node_id"`
	AssignedWorkerID      string    `gorm:"column:assigned_worker_id"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (LiveChannelRecord) TableName() string { return "t_live_channel" }

// LiveProfileRenditionRecord 表示直播模板清晰度配置表映射。
type LiveProfileRenditionRecord struct {
	ID               uint64    `gorm:"column:id;primaryKey"`
	ProfileID        uint64    `gorm:"column:profile_id"`
	RenditionName    string    `gorm:"column:rendition_name"`
	IsSource         bool      `gorm:"column:is_source"`
	Enabled          bool      `gorm:"column:enabled"`
	OutWidth         int       `gorm:"column:out_width"`
	OutHeight        int       `gorm:"column:out_height"`
	VideoCodec       string    `gorm:"column:video_codec"`
	VideoBitrateKbps int       `gorm:"column:video_bitrate_kbps"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (LiveProfileRenditionRecord) TableName() string { return "t_live_profile_rendition" }

// LiveSessionRecord 表示直播会话表映射。
type LiveSessionRecord struct {
	SessionID        uint64     `gorm:"column:session_id;primaryKey"`
	ChannelID        uint64     `gorm:"column:channel_id"`
	SessionKey       string     `gorm:"column:session_key"`
	Status           int        `gorm:"column:status"`
	IngestURL        string     `gorm:"column:ingest_url"`
	PlaybackHLSURL   string     `gorm:"column:playback_hls_url"`
	PushProtocol     string     `gorm:"column:push_protocol"`
	AssignedNodeID   uint64     `gorm:"column:assigned_node_id"`
	AssignedWorkerID string     `gorm:"column:assigned_worker_id"`
	ResumeCount      int        `gorm:"column:resume_count"`
	StartedAt        time.Time  `gorm:"column:started_at"`
	EndedAt          *time.Time `gorm:"column:ended_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (LiveSessionRecord) TableName() string { return "t_live_session" }

// LiveSessionEventRecord 表示直播会话事件表映射。
type LiveSessionEventRecord struct {
	EventID          uint64    `gorm:"column:event_id;primaryKey"`
	SessionID        *uint64   `gorm:"column:session_id"`
	ChannelID        uint64    `gorm:"column:channel_id"`
	EventType        string    `gorm:"column:event_type"`
	EventPayloadJSON string    `gorm:"column:event_payload_json"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (LiveSessionEventRecord) TableName() string { return "t_live_session_event" }

// LivePlaybackTokenRecord 表示直播播放令牌表映射。
type LivePlaybackTokenRecord struct {
	TokenID   uint64    `gorm:"column:token_id;primaryKey"`
	ChannelID uint64    `gorm:"column:channel_id"`
	UserToken string    `gorm:"column:user_token"`
	ViewerID  string    `gorm:"column:viewer_id"`
	AllowPlay bool      `gorm:"column:allow_play"`
	ExpireAt  time.Time `gorm:"column:expire_at"`
	IssuedAt  time.Time `gorm:"column:issued_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (LivePlaybackTokenRecord) TableName() string { return "t_live_playback_token" }

// LivePublishSessionRecord 表示直播推流会话表映射。
type LivePublishSessionRecord struct {
	PublishSessionID uint64     `gorm:"column:publish_session_id;primaryKey"`
	ChannelID        uint64     `gorm:"column:channel_id"`
	SessionID        *uint64    `gorm:"column:session_id"`
	StreamKey        string     `gorm:"column:stream_key"`
	PublishIP        string     `gorm:"column:publish_ip"`
	PublishStatus    int        `gorm:"column:publish_status"`
	ConnectedAt      *time.Time `gorm:"column:connected_at"`
	DisconnectedAt   *time.Time `gorm:"column:disconnected_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (LivePublishSessionRecord) TableName() string { return "t_live_publish_session" }

// LivePublishAuthLogRecord 表示直播推流鉴权日志表映射。
type LivePublishAuthLogRecord struct {
	AuthLogID   uint64    `gorm:"column:auth_log_id;primaryKey"`
	ChannelID   uint64    `gorm:"column:channel_id"`
	StreamKey   string    `gorm:"column:stream_key"`
	RequestIP   string    `gorm:"column:request_ip"`
	AuthResult  int       `gorm:"column:auth_result"`
	AuthMessage string    `gorm:"column:auth_message"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (LivePublishAuthLogRecord) TableName() string { return "t_live_publish_auth_log" }
