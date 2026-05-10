package mysql

import "time"

// ClusterNodeRecord 表示集群节点表映射。
type ClusterNodeRecord struct {
	NodeID               uint64    `gorm:"column:node_id;primaryKey"`
	NodeName             string    `gorm:"column:node_name"`
	HostIP               string    `gorm:"column:host_ip"`
	GRPCHost             string    `gorm:"column:grpc_host"`
	HTTPHost             string    `gorm:"column:http_host"`
	Enabled              bool      `gorm:"column:enabled"`
	Quarantined          bool      `gorm:"column:quarantined"`
	QuarantineReason     string    `gorm:"column:quarantine_reason"`
	Draining             bool      `gorm:"column:draining"`
	DrainReason          string    `gorm:"column:drain_reason"`
	LastStateChangeAt    time.Time `gorm:"column:last_state_change_at"`
	CapacityGeneration   uint64    `gorm:"column:capacity_generation"`
	SupportNVENC         bool      `gorm:"column:support_nvenc"`
	SupportQSV           bool      `gorm:"column:support_qsv"`
	SupportAMF           bool      `gorm:"column:support_amf"`
	SupportVAAPI         bool      `gorm:"column:support_vaapi"`
	SupportVideoToolbox  bool      `gorm:"column:support_videotoolbox"`
	CPUCores             int       `gorm:"column:cpu_cores"`
	MemoryTotalMB        int       `gorm:"column:memory_total_mb"`
	DiskTotalGB          int       `gorm:"column:disk_total_gb"`
	NetUpMbps            int       `gorm:"column:net_up_mbps"`
	NetDownMbps          int       `gorm:"column:net_down_mbps"`
	MaxTranscodeSessions int       `gorm:"column:max_transcode_sessions"`
	MaxUploadConcurrency int       `gorm:"column:max_upload_concurrency"`
	NodeTags             string    `gorm:"column:node_tags"`
	LastHeartbeatAt      time.Time `gorm:"column:last_heartbeat_at"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (ClusterNodeRecord) TableName() string { return "t_cluster_node" }

// NodeGPUDeviceRecord 表示节点 GPU 设备表映射。
type NodeGPUDeviceRecord struct {
	GPUDeviceID          uint64    `gorm:"column:gpu_device_id;primaryKey"`
	NodeID               uint64    `gorm:"column:node_id"`
	GPUIndex             int       `gorm:"column:gpu_index"`
	GPUUUID              string    `gorm:"column:gpu_uuid"`
	Vendor               string    `gorm:"column:vendor"`
	Model                string    `gorm:"column:model"`
	DriverVersion        string    `gorm:"column:driver_version"`
	MemoryTotalMB        int       `gorm:"column:memory_total_mb"`
	MaxTranscodeSessions int       `gorm:"column:max_transcode_sessions"`
	Healthy              bool      `gorm:"column:healthy"`
	Schedulable          bool      `gorm:"column:schedulable"`
	LastSeenAt           time.Time `gorm:"column:last_seen_at"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (NodeGPUDeviceRecord) TableName() string { return "t_node_gpu_device" }

// WorkerInstanceRecord 表示 Worker 实例表映射。
type WorkerInstanceRecord struct {
	ID                 uint64     `gorm:"column:id;primaryKey"`
	NodeID             uint64     `gorm:"column:node_id"`
	WorkerID           string     `gorm:"column:worker_id"`
	LogicalWorkerID    string     `gorm:"column:logical_worker_id"`
	PhysicalWorkerID   string     `gorm:"column:physical_worker_id"`
	MachineFingerprint string     `gorm:"column:machine_fingerprint"`
	StartupInstanceID  string     `gorm:"column:startup_instance_id"`
	BootID             string     `gorm:"column:boot_id"`
	PID                int        `gorm:"column:pid"`
	Version            string     `gorm:"column:version"`
	Status             int        `gorm:"column:status"`
	StartAt            time.Time  `gorm:"column:start_at"`
	ExitedAt           *time.Time `gorm:"column:exited_at"`
	ExitReason         string     `gorm:"column:exit_reason"`
	LastHeartbeatAt    time.Time  `gorm:"column:last_heartbeat_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (WorkerInstanceRecord) TableName() string { return "t_worker_instance" }

// WorkerCodecCapabilityRecord 表示 Worker 编解码能力快照表映射。
type WorkerCodecCapabilityRecord struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	NodeID                uint64    `gorm:"column:node_id"`
	WorkerInstanceID      uint64    `gorm:"column:worker_instance_id"`
	GPUDeviceID           uint64    `gorm:"column:gpu_device_id"`
	GPUIndex              int       `gorm:"column:gpu_index"`
	GPUUUID               string    `gorm:"column:gpu_uuid"`
	StartupInstanceID     string    `gorm:"column:startup_instance_id"`
	ProbeGeneration       uint64    `gorm:"column:probe_generation"`
	MachineFingerprint    string    `gorm:"column:machine_fingerprint"`
	CodecName             string    `gorm:"column:codec_name"`
	CapType               int       `gorm:"column:cap_type"`
	HWType                int       `gorm:"column:hw_type"`
	MaxSessions           int       `gorm:"column:max_sessions"`
	Enabled               bool      `gorm:"column:enabled"`
	IsLatest              bool      `gorm:"column:is_latest"`
	CapabilityPayloadJSON *string   `gorm:"column:capability_payload_json"`
	CollectedAt           time.Time `gorm:"column:collected_at"`
	CreatedAt             time.Time `gorm:"column:created_at"`
}

func (WorkerCodecCapabilityRecord) TableName() string { return "t_worker_codec_capability" }

// JobExecutionRecord 表示任务执行实例表映射。
type JobExecutionRecord struct {
	ExecutionID            uint64     `gorm:"column:execution_id;primaryKey"`
	JobID                  uint64     `gorm:"column:job_id"`
	AttemptNo              int        `gorm:"column:attempt_no"`
	LeaseGeneration        uint64     `gorm:"column:lease_generation"`
	NodeID                 uint64     `gorm:"column:node_id"`
	WorkerInstanceID       uint64     `gorm:"column:worker_instance_id"`
	GPUDeviceID            uint64     `gorm:"column:gpu_device_id"`
	SelectedGPUIndex       int        `gorm:"column:selected_gpu_index"`
	SelectedExecutionHWAcc string     `gorm:"column:selected_execution_hwaccel"`
	Status                 int        `gorm:"column:status"`
	LeaseOwner             string     `gorm:"column:lease_owner"`
	LeaseExpireAt          *time.Time `gorm:"column:lease_expire_at"`
	LastHeartbeatAt        *time.Time `gorm:"column:last_heartbeat_at"`
	FailureReason          string     `gorm:"column:failure_reason"`
	RecoverableFlag        bool       `gorm:"column:recoverable_flag"`
	StartedAt              *time.Time `gorm:"column:started_at"`
	FinishedAt             *time.Time `gorm:"column:finished_at"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

func (JobExecutionRecord) TableName() string { return "t_transcode_job_execution" }
