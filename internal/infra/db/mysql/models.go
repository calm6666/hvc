package mysql

import "time"

// AdminUserRecord 表示管理员用户表映射。
type AdminUserRecord struct {
	AdminUserID    uint64    `gorm:"column:admin_user_id;primaryKey"`
	Username       string    `gorm:"column:username"`
	PasswordHash   string    `gorm:"column:password_hash"`
	DisplayName    string    `gorm:"column:display_name"`
	Status         int       `gorm:"column:status"`
	LastLoginAt    time.Time `gorm:"column:last_login_at"`
	LastLoginIP    string    `gorm:"column:last_login_ip"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (AdminUserRecord) TableName() string { return "t_admin_user" }

// AdminSessionRecord 表示管理员会话表映射。
type AdminSessionRecord struct {
	SessionID      uint64    `gorm:"column:session_id;primaryKey"`
	AdminUserID    uint64    `gorm:"column:admin_user_id"`
	SessionToken   string    `gorm:"column:session_token"`
	SessionStatus  int       `gorm:"column:session_status"`
	ExpireAt       time.Time `gorm:"column:expire_at"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (AdminSessionRecord) TableName() string { return "t_admin_session" }

// AdminAuditLogRecord 表示管理员审计表映射。
type AdminAuditLogRecord struct {
	AuditLogID        uint64    `gorm:"column:audit_log_id;primaryKey"`
	AdminUserID       uint64    `gorm:"column:admin_user_id"`
	Username          string    `gorm:"column:username"`
	ActionName        string    `gorm:"column:action_name"`
	TargetType        string    `gorm:"column:target_type"`
	TargetID          string    `gorm:"column:target_id"`
	RequestID         string    `gorm:"column:request_id"`
	RequestIP         string    `gorm:"column:request_ip"`
	RequestUserAgent  string    `gorm:"column:request_user_agent"`
	ResultCode        int       `gorm:"column:result_code"`
	ResultMessage     string    `gorm:"column:result_message"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (AdminAuditLogRecord) TableName() string { return "t_admin_audit_log" }

// RuntimeConfigRecord 表示运行配置版本表映射。
type RuntimeConfigRecord struct {
	ConfigVersion              uint64    `gorm:"column:config_version;primaryKey"`
	DefaultProfileID           uint64    `gorm:"column:default_profile_id"`
	MaxGlobalTranscodeSessions int       `gorm:"column:max_global_transcode_sessions"`
	JobLeaseTTLSeconds         int       `gorm:"column:job_lease_ttl_sec"`
	WorkerHeartbeatTimeoutSec  int       `gorm:"column:worker_heartbeat_timeout_sec"`
	Published                  bool      `gorm:"column:published"`
	ChangeSummary              string    `gorm:"column:change_summary"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
	UpdatedAt                  time.Time `gorm:"column:updated_at"`
}

func (RuntimeConfigRecord) TableName() string { return "t_runtime_config" }

// ConfigCenterBindingRecord 表示配置中心绑定表映射。
type ConfigCenterBindingRecord struct {
	BindingID        uint64    `gorm:"column:binding_id;primaryKey"`
	BindingName      string    `gorm:"column:binding_name"`
	ProviderType     string    `gorm:"column:provider_type"`
	Endpoint         string    `gorm:"column:endpoint"`
	Namespace        string    `gorm:"column:namespace"`
	AuthMode         string    `gorm:"column:auth_mode"`
	AccessKey        string    `gorm:"column:access_key"`
	SecretKey        string    `gorm:"column:secret_key"`
	Token            string    `gorm:"column:token"`
	Enabled          bool      `gorm:"column:enabled"`
	Priority         int       `gorm:"column:priority"`
	LastSyncStatus   string    `gorm:"column:last_sync_status"`
	LastSyncMessage  string    `gorm:"column:last_sync_message"`
	LastSyncAt       time.Time `gorm:"column:last_sync_at"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (ConfigCenterBindingRecord) TableName() string { return "t_config_center_binding" }

// EffectiveRuntimeConfigSnapshotRecord 表示生效配置快照表映射。
type EffectiveRuntimeConfigSnapshotRecord struct {
	SnapshotID       uint64    `gorm:"column:snapshot_id;primaryKey"`
	ConfigVersion    uint64    `gorm:"column:config_version"`
	ConfigSource     string    `gorm:"column:config_source"`
	SourceRevision   string    `gorm:"column:source_revision"`
	MergedPayloadJSON string   `gorm:"column:merged_payload_json"`
	ConfigHash       string    `gorm:"column:config_hash"`
	CreatedBy        string    `gorm:"column:created_by"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (EffectiveRuntimeConfigSnapshotRecord) TableName() string { return "t_effective_runtime_config_snapshot" }

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
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (LiveChannelRecord) TableName() string { return "t_live_channel" }

// LiveSessionRecord 表示直播会话表映射。
type LiveSessionRecord struct {
	SessionID        uint64    `gorm:"column:session_id;primaryKey"`
	ChannelID        uint64    `gorm:"column:channel_id"`
	SessionKey       string    `gorm:"column:session_key"`
	Status           int       `gorm:"column:status"`
	IngestURL        string    `gorm:"column:ingest_url"`
	PlaybackHLSURL   string    `gorm:"column:playback_hls_url"`
	StartedAt        time.Time `gorm:"column:started_at"`
	EndedAt          time.Time `gorm:"column:ended_at"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (LiveSessionRecord) TableName() string { return "t_live_session" }

// LivePublishSessionRecord 表示直播推流会话表映射。
type LivePublishSessionRecord struct {
	PublishSessionID uint64    `gorm:"column:publish_session_id;primaryKey"`
	ChannelID        uint64    `gorm:"column:channel_id"`
	SessionID        uint64    `gorm:"column:session_id"`
	StreamKey        string    `gorm:"column:stream_key"`
	PublishIP        string    `gorm:"column:publish_ip"`
	PublishStatus    int       `gorm:"column:publish_status"`
	ConnectedAt      time.Time `gorm:"column:connected_at"`
	DisconnectedAt   time.Time `gorm:"column:disconnected_at"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (LivePublishSessionRecord) TableName() string { return "t_live_publish_session" }

// JobRecord 表示任务表记录。
type JobRecord struct {
	JobID                    uint64    `gorm:"column:job_id;primaryKey"`
	RequestID                string    `gorm:"column:request_id"`
	BizKey                   string    `gorm:"column:biz_key"`
	Status                   int       `gorm:"column:status"`
	Priority                 int       `gorm:"column:priority"`
	SourceURL                string    `gorm:"column:source_url"`
	ProfileID                uint64    `gorm:"column:profile_id"`
	SegmentDurationSec       int       `gorm:"column:segment_duration_sec"`
	SupportDash              bool      `gorm:"column:support_dash"`
	SupportHLS               bool      `gorm:"column:support_hls"`
	EnableWatermark          bool      `gorm:"column:enable_watermark"`
	WatermarkImageURL        string    `gorm:"column:watermark_image_url"`
	WatermarkAnchor          int       `gorm:"column:watermark_anchor"`
	WatermarkXRatio          float64   `gorm:"column:watermark_x_ratio"`
	WatermarkYRatio          float64   `gorm:"column:watermark_y_ratio"`
	WatermarkWidthRatio      float64   `gorm:"column:watermark_width_ratio"`
	WatermarkOpacity         float64   `gorm:"column:watermark_opacity"`
	OutputStorageID          uint64    `gorm:"column:output_storage_id"`
	OutputBasePrefix         string    `gorm:"column:output_base_prefix"`
	AssignedNodeID           uint64    `gorm:"column:assigned_node_id"`
	AssignedWorkerID         string    `gorm:"column:assigned_worker_id"`
	ExecutorWorkerInstanceID uint64    `gorm:"column:executor_worker_instance_id"`
	SelectedExecutionHWAccel string    `gorm:"column:selected_execution_hwaccel"`
	SelectedGPUIndex         int       `gorm:"column:selected_gpu_index"`
	SelectedGPUDeviceID      uint64    `gorm:"column:selected_gpu_device_id"`
	LeaseOwner               string    `gorm:"column:lease_owner"`
	LeaseGeneration          uint64    `gorm:"column:lease_generation"`
	AttemptNo                int       `gorm:"column:attempt_no"`
	ProgressPermille         int       `gorm:"column:progress_permille"`
	ProgressStage            string    `gorm:"column:progress_stage"`
	ErrorCode                string    `gorm:"column:error_code"`
	ErrorMessage             string    `gorm:"column:error_message"`
	CreatedAt                time.Time `gorm:"column:created_at"`
	UpdatedAt                time.Time `gorm:"column:updated_at"`
}

func (JobRecord) TableName() string { return "t_transcode_job" }

// SegmentRecord 表示分片表记录。
type SegmentRecord struct {
	SegmentID          uint64    `gorm:"column:segment_id;primaryKey"`
	JobID              uint64    `gorm:"column:job_id"`
	RenditionID        uint64    `gorm:"column:rendition_id"`
	MediaType          int       `gorm:"column:media_type"`
	IsInitSegment      bool      `gorm:"column:is_init_segment"`
	SequenceNo         int       `gorm:"column:sequence_no"`
	DurationMS         int       `gorm:"column:duration_ms"`
	SupportDash        bool      `gorm:"column:support_dash"`
	SupportHLS         bool      `gorm:"column:support_hls"`
	CodecName          string    `gorm:"column:codec_name"`
	ObjectKey          string    `gorm:"column:object_key"`
	ObjectSizeBytes    uint64    `gorm:"column:object_size_bytes"`
	ObjectETag         string    `gorm:"column:object_etag"`
	SHA256             string    `gorm:"column:sha256"`
	StartPTSMS         int64     `gorm:"column:start_pts_ms"`
	EndPTSMS           int64     `gorm:"column:end_pts_ms"`
	UploadStatus       int       `gorm:"column:upload_status"`
	UploadRetryCount   int       `gorm:"column:upload_retry_count"`
	UploadErrorMessage string    `gorm:"column:upload_error_message"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (SegmentRecord) TableName() string { return "t_transcode_segment" }

// OutboxRecord 表示回调事件表记录。
type OutboxRecord struct {
	EventID          uint64    `gorm:"column:event_id;primaryKey"`
	EventType        string    `gorm:"column:event_type"`
	JobID            uint64    `gorm:"column:job_id"`
	RequestID        string    `gorm:"column:request_id"`
	PayloadJSON      string    `gorm:"column:payload_json"`
	Status           int       `gorm:"column:status"`
	RetryCount       int       `gorm:"column:retry_count"`
	MaxRetryCount    int       `gorm:"column:max_retry_count"`
	NextRetryAt      time.Time `gorm:"column:next_retry_at"`
	LastErrorMessage string    `gorm:"column:last_error_message"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (OutboxRecord) TableName() string { return "t_event_outbox" }
