package admin

import (
	"time"

	"hvc/internal/cluster/membership"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	schedulerview "hvc/internal/scheduler"
)

type clusterNodeMetricsSummaryView struct {
	CPUUsagePercent         int `json:"cpu_usage_percent"`
	MemoryUsagePercent      int `json:"memory_usage_percent"`
	GPUMemoryUsagePercent   int `json:"gpu_memory_usage_percent"`
	UploadQueueDepth        int `json:"upload_queue_depth"`
	ActiveTranscodeSessions int `json:"active_transcode_sessions"`
}

type clusterNodeGPUDeviceView struct {
	GPUDeviceID          uint64               `json:"gpu_device_id"`
	GPUIndex             int                  `json:"gpu_index"`
	GPUUUID              string               `json:"gpu_uuid"`
	Vendor               string               `json:"vendor"`
	Model                string               `json:"model"`
	DriverVersion        string               `json:"driver_version"`
	MemoryTotalMB        int                  `json:"memory_total_mb"`
	MaxTranscodeSessions int                  `json:"max_transcode_sessions"`
	Healthy              bool                 `json:"healthy"`
	Schedulable          bool                 `json:"schedulable"`
	LastSeenAt           time.Time            `json:"last_seen_at"`
	RuntimeCapability    *model.GPUCapability `json:"runtime_capability,omitempty"`
}

type clusterNodeListItemView struct {
	NodeID               uint64                         `json:"node_id"`
	NodeName             string                         `json:"node_name"`
	HostIP               string                         `json:"host_ip"`
	GRPCHost             string                         `json:"grpc_host"`
	HTTPHost             string                         `json:"http_host"`
	Enabled              bool                           `json:"enabled"`
	Quarantined          bool                           `json:"quarantined"`
	QuarantineReason     string                         `json:"quarantine_reason"`
	Draining             bool                           `json:"draining"`
	DrainReason          string                         `json:"drain_reason"`
	LastStateChangeAt    time.Time                      `json:"last_state_change_at"`
	CapacityGeneration   uint64                         `json:"capacity_generation"`
	SupportNVENC         bool                           `json:"support_nvenc"`
	SupportQSV           bool                           `json:"support_qsv"`
	SupportAMF           bool                           `json:"support_amf"`
	SupportVAAPI         bool                           `json:"support_vaapi"`
	SupportVideoToolbox  bool                           `json:"support_videotoolbox"`
	CPUCores             int                            `json:"cpu_cores"`
	MemoryTotalMB        int                            `json:"memory_total_mb"`
	DiskTotalGB          int                            `json:"disk_total_gb"`
	NetUpMbps            int                            `json:"net_up_mbps"`
	NetDownMbps          int                            `json:"net_down_mbps"`
	MaxTranscodeSessions int                            `json:"max_transcode_sessions"`
	MaxUploadConcurrency int                            `json:"max_upload_concurrency"`
	NodeTags             string                         `json:"node_tags"`
	LastHeartbeatAt      time.Time                      `json:"last_heartbeat_at"`
	MetricsAvailable     bool                           `json:"metrics_available"`
	MetricsFresh         bool                           `json:"metrics_fresh"`
	OnlineEstimate       bool                           `json:"online_estimate"`
	SchedulerReady       bool                           `json:"scheduler_ready"`
	StateReason          string                         `json:"state_reason"`
	LastMetricsAt        *time.Time                     `json:"last_metrics_at,omitempty"`
	Metrics              *clusterNodeMetricsSummaryView `json:"metrics,omitempty"`
	GPUSummary           ClusterNodeGPUSummary          `json:"gpu_summary"`
}

type clusterNodeDetailView struct {
	clusterNodeListItemView
	GPUDevices []clusterNodeGPUDeviceView `json:"gpu_devices"`
}

type clusterMemberView struct {
	NodeID              uint64     `json:"node_id"`
	NodeName            string     `json:"node_name"`
	NodeRole            string     `json:"node_role"`
	Host                string     `json:"host"`
	HostIP              string     `json:"host_ip"`
	GRPCHost            string     `json:"grpc_host"`
	HTTPHost            string     `json:"http_host"`
	Enabled             bool       `json:"enabled"`
	Quarantined         bool       `json:"quarantined"`
	Draining            bool       `json:"draining"`
	ControlPlane        bool       `json:"control_plane"`
	AdminAccessible     bool       `json:"admin_accessible"`
	OnlineEstimate      bool       `json:"online_estimate"`
	Source              string     `json:"source"`
	LastHeartbeatAt     time.Time  `json:"last_heartbeat_at"`
	RegistryHeartbeatAt *time.Time `json:"registry_heartbeat_at,omitempty"`
	LastMetricsAt       *time.Time `json:"last_metrics_at,omitempty"`
	WorkerTotal         int        `json:"worker_total"`
	WorkerOnlineTotal   int        `json:"worker_online_total"`
}

func toClusterNodeMetricsSummaryView(metrics *model.NodeMetrics) *clusterNodeMetricsSummaryView {
	if metrics == nil {
		return nil
	}
	return &clusterNodeMetricsSummaryView{
		CPUUsagePercent:         metrics.CPUUsagePercent,
		MemoryUsagePercent:      metrics.MemoryUsagePercent,
		GPUMemoryUsagePercent:   metrics.GPUMemoryUsagePercent,
		UploadQueueDepth:        metrics.UploadQueueDepth,
		ActiveTranscodeSessions: metrics.ActiveTranscodeSessions,
	}
}

func toClusterNodeListItemView(item ClusterNodeView) clusterNodeListItemView {
	return clusterNodeListItemView{
		NodeID:               item.NodeID,
		NodeName:             item.NodeName,
		HostIP:               item.HostIP,
		GRPCHost:             item.GRPCHost,
		HTTPHost:             item.HTTPHost,
		Enabled:              item.Enabled,
		Quarantined:          item.Quarantined,
		QuarantineReason:     item.QuarantineReason,
		Draining:             item.Draining,
		DrainReason:          item.DrainReason,
		LastStateChangeAt:    item.LastStateChangeAt,
		CapacityGeneration:   item.CapacityGeneration,
		SupportNVENC:         item.SupportNVENC,
		SupportQSV:           item.SupportQSV,
		SupportAMF:           item.SupportAMF,
		SupportVAAPI:         item.SupportVAAPI,
		SupportVideoToolbox:  item.SupportVideoToolbox,
		CPUCores:             item.CPUCores,
		MemoryTotalMB:        item.MemoryTotalMB,
		DiskTotalGB:          item.DiskTotalGB,
		NetUpMbps:            item.NetUpMbps,
		NetDownMbps:          item.NetDownMbps,
		MaxTranscodeSessions: item.MaxTranscodeSessions,
		MaxUploadConcurrency: item.MaxUploadConcurrency,
		NodeTags:             item.NodeTags,
		LastHeartbeatAt:      item.LastHeartbeatAt,
		MetricsAvailable:     item.MetricsAvailable,
		MetricsFresh:         item.MetricsFresh,
		OnlineEstimate:       item.OnlineEstimate,
		SchedulerReady:       item.SchedulerReady,
		StateReason:          item.StateReason,
		LastMetricsAt:        item.LastMetricsAt,
		Metrics:              toClusterNodeMetricsSummaryView(item.Metrics),
		GPUSummary:           item.GPUSummary,
	}
}

func toClusterNodeDetailView(item ClusterNodeView) clusterNodeDetailView {
	view := clusterNodeDetailView{
		clusterNodeListItemView: toClusterNodeListItemView(item),
		GPUDevices:              make([]clusterNodeGPUDeviceView, 0, len(item.GPUDevices)),
	}
	for _, gpu := range item.GPUDevices {
		view.GPUDevices = append(view.GPUDevices, clusterNodeGPUDeviceView{
			GPUDeviceID:          gpu.GPUDeviceID,
			GPUIndex:             gpu.GPUIndex,
			GPUUUID:              gpu.GPUUUID,
			Vendor:               gpu.Vendor,
			Model:                gpu.Model,
			DriverVersion:        gpu.DriverVersion,
			MemoryTotalMB:        gpu.MemoryTotalMB,
			MaxTranscodeSessions: gpu.MaxTranscodeSessions,
			Healthy:              gpu.Healthy,
			Schedulable:          gpu.Schedulable,
			LastSeenAt:           gpu.LastSeenAt,
			RuntimeCapability:    gpu.RuntimeCapability,
		})
	}
	return view
}

func toClusterMemberView(item membership.Node) clusterMemberView {
	registryAt := item.LastHeartbeatAt
	controlPlane, adminAccessible := evaluateNodeManagementCapability(item.NodeRole, item.Enabled, item.HTTPHost)
	return clusterMemberView{
		NodeID:              item.NodeID,
		NodeName:            item.NodeName,
		NodeRole:            item.NodeRole,
		Host:                item.Host,
		HostIP:              item.HostIP,
		GRPCHost:            item.GRPCHost,
		HTTPHost:            item.HTTPHost,
		Enabled:             item.Enabled,
		Quarantined:         item.Quarantined,
		Draining:            false,
		ControlPlane:        controlPlane,
		AdminAccessible:     adminAccessible,
		OnlineEstimate:      true,
		Source:              "registry",
		LastHeartbeatAt:     item.LastHeartbeatAt,
		RegistryHeartbeatAt: &registryAt,
	}
}

func evaluateNodeManagementCapability(nodeRole string, enabled bool, httpHost string) (bool, bool) {
	adminAccessible := enabled && httpHost != ""
	controlPlane := false
	switch nodeRole {
	case "standalone", "cluster-control", "cluster-allinone":
		controlPlane = enabled
	}
	return controlPlane, adminAccessible
}

func pageSlice[T any](items []T, page, pageSize int) ([]T, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}, total
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total
}

func toClusterNodeRecordView(record mysql.ClusterNodeRecord) clusterNodeListItemView {
	return toClusterNodeListItemView(ClusterNodeView{ClusterNodeRecord: record})
}

type clusterWorkerListItemView struct {
	ID                 uint64     `json:"id"`
	NodeID             uint64     `json:"node_id"`
	WorkerID           string     `json:"worker_id"`
	LogicalWorkerID    string     `json:"logical_worker_id"`
	PhysicalWorkerID   string     `json:"physical_worker_id"`
	MachineFingerprint string     `json:"machine_fingerprint"`
	StartupInstanceID  string     `json:"startup_instance_id"`
	BootID             string     `json:"boot_id"`
	PID                int        `json:"pid"`
	Version            string     `json:"version"`
	Status             int        `json:"status"`
	StatusName         string     `json:"status_name"`
	OnlineEstimate     bool       `json:"online_estimate"`
	StartAt            time.Time  `json:"start_at"`
	ExitedAt           *time.Time `json:"exited_at,omitempty"`
	ExitReason         string     `json:"exit_reason"`
	LastHeartbeatAt    time.Time  `json:"last_heartbeat_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type clusterSchedulerMetricsView struct {
	CPUUsagePercent         int                   `json:"cpu_usage_percent"`
	MemoryUsagePercent      int                   `json:"memory_usage_percent"`
	GPUMemoryUsagePercent   int                   `json:"gpu_memory_usage_percent"`
	UploadQueueDepth        int                   `json:"upload_queue_depth"`
	ActiveTranscodeSessions int                   `json:"active_transcode_sessions"`
	GPUCapabilities         []model.GPUCapability `json:"gpu_capabilities,omitempty"`
}

type clusterSchedulerDecisionPreviewView struct {
	NodeID              uint64 `json:"node_id"`
	SelectedGPUIndex    int    `json:"selected_gpu_index"`
	SelectedGPUDeviceID uint64 `json:"selected_gpu_device_id"`
	SelectedExecutionHW string `json:"selected_execution_hw"`
	LeaseGeneration     uint64 `json:"lease_generation"`
	AttemptNo           int    `json:"attempt_no"`
}

type clusterSchedulerCandidateView struct {
	NodeID                    uint64                               `json:"node_id"`
	NodeName                  string                               `json:"node_name"`
	Enabled                   bool                                 `json:"enabled"`
	Quarantined               bool                                 `json:"quarantined"`
	Draining                  bool                                 `json:"draining"`
	Shielded                  bool                                 `json:"shielded"`
	SupportsHardwareWatermark bool                                 `json:"supports_hardware_watermark"`
	MaxTranscodeSessions      int                                  `json:"max_transcode_sessions"`
	MaxUploadConcurrency      int                                  `json:"max_upload_concurrency"`
	MetricsAvailable          bool                                 `json:"metrics_available"`
	MetricsFresh              bool                                 `json:"metrics_fresh"`
	Online                    bool                                 `json:"online"`
	LastMetricsAt             time.Time                            `json:"last_metrics_at"`
	LastHeartbeatAt           time.Time                            `json:"last_heartbeat_at"`
	Score                     int                                  `json:"score"`
	ScoreRank                 int                                  `json:"score_rank"`
	FilterPassed              bool                                 `json:"filter_passed"`
	FilterReasons             []string                             `json:"filter_reasons"`
	InTopKPool                bool                                 `json:"in_top_k_pool"`
	DecisionPreview           *clusterSchedulerDecisionPreviewView `json:"decision_preview,omitempty"`
	Metrics                   clusterSchedulerMetricsView          `json:"metrics"`
}

type clusterSchedulerInsightView struct {
	GeneratedAt                time.Time                            `json:"generated_at"`
	TopK                       int                                  `json:"top_k"`
	DispatchPolicy             string                               `json:"dispatch_policy"`
	MaxGlobalTranscodeSessions int                                  `json:"max_global_transcode_sessions"`
	ActiveExecutionCount       int64                                `json:"active_execution_count"`
	QueuedJobCount             int64                                `json:"queued_job_count"`
	CandidateTotal             int                                  `json:"candidate_total"`
	PassedCandidateTotal       int                                  `json:"passed_candidate_total"`
	RecommendedNodeID          uint64                               `json:"recommended_node_id"`
	RecommendedDecision        *clusterSchedulerDecisionPreviewView `json:"recommended_decision,omitempty"`
	Candidates                 []clusterSchedulerCandidateView      `json:"candidates"`
}

type clusterNodeResourceGPUView struct {
	GPUIndex               int    `json:"gpu_index"`
	GPUUUID                string `json:"gpu_uuid"`
	Model                  string `json:"model"`
	Vendor                 string `json:"vendor"`
	MemoryTotalMB          int    `json:"memory_total_mb"`
	GPUMemoryUsagePercent  int    `json:"gpu_memory_usage_percent"`
	GPUUtilizationPercent  int    `json:"gpu_utilization_percent"`
	ActiveSessions         int    `json:"active_sessions"`
	MaxSessions            int    `json:"max_sessions"`
	Schedulable            bool   `json:"schedulable"`
	Healthy                bool   `json:"healthy"`
}

type clusterNodeResourceDistributionView struct {
	NodeID                      uint64                     `json:"node_id"`
	NodeName                    string                     `json:"node_name"`
	NodeRole                    string                     `json:"node_role"`
	Enabled                     bool                       `json:"enabled"`
	Quarantined                 bool                       `json:"quarantined"`
	Draining                    bool                       `json:"draining"`
	OnlineEstimate              bool                       `json:"online_estimate"`
	SchedulerReady              bool                       `json:"scheduler_ready"`
	StateReason                 string                     `json:"state_reason"`
	MetricsAvailable            bool                       `json:"metrics_available"`
	MetricsFresh                bool                       `json:"metrics_fresh"`
	LastMetricsAt               *time.Time                 `json:"last_metrics_at,omitempty"`
	LastHeartbeatAt             time.Time                  `json:"last_heartbeat_at"`
	ActiveExecutionTotal        int                        `json:"active_execution_total"`
	ActiveTranscodeSessions     int                        `json:"active_transcode_sessions"`
	UploadQueueDepth            int                        `json:"upload_queue_depth"`
	MaxTranscodeSessions        int                        `json:"max_transcode_sessions"`
	MaxUploadConcurrency        int                        `json:"max_upload_concurrency"`
	RemainingTranscodeCapacity  int                        `json:"remaining_transcode_capacity"`
	RemainingUploadCapacity     int                        `json:"remaining_upload_capacity"`
	CPUUsagePercent             int                        `json:"cpu_usage_percent"`
	MemoryUsagePercent          int                        `json:"memory_usage_percent"`
	GPUMemoryUsagePercent       int                        `json:"gpu_memory_usage_percent"`
	GPUActiveSessionTotal       int                        `json:"gpu_active_session_total"`
	GPUs                        []clusterNodeResourceGPUView `json:"gpus"`
}

func toClusterWorkerListItemView(record mysql.WorkerInstanceRecord, now time.Time) clusterWorkerListItemView {
	return clusterWorkerListItemView{
		ID:                 record.ID,
		NodeID:             record.NodeID,
		WorkerID:           record.WorkerID,
		LogicalWorkerID:    record.LogicalWorkerID,
		PhysicalWorkerID:   record.PhysicalWorkerID,
		MachineFingerprint: record.MachineFingerprint,
		StartupInstanceID:  record.StartupInstanceID,
		BootID:             record.BootID,
		PID:                record.PID,
		Version:            record.Version,
		Status:             record.Status,
		StatusName:         workerStatusName(record.Status),
		OnlineEstimate:     record.Status == 1 && !record.LastHeartbeatAt.IsZero() && now.Sub(record.LastHeartbeatAt) <= clusterNodeOnlineGracePeriod,
		StartAt:            record.StartAt,
		ExitedAt:           record.ExitedAt,
		ExitReason:         record.ExitReason,
		LastHeartbeatAt:    record.LastHeartbeatAt,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func toClusterSchedulerInsightView(insight schedulerview.DispatchInsight) clusterSchedulerInsightView {
	view := clusterSchedulerInsightView{
		GeneratedAt:                insight.GeneratedAt,
		TopK:                       insight.TopK,
		DispatchPolicy:             insight.DispatchPolicy,
		MaxGlobalTranscodeSessions: insight.MaxGlobalTranscodeSessions,
		ActiveExecutionCount:       insight.ActiveExecutionCount,
		QueuedJobCount:             insight.QueuedJobCount,
		CandidateTotal:             insight.CandidateTotal,
		PassedCandidateTotal:       insight.PassedCandidateTotal,
		RecommendedNodeID:          insight.RecommendedNodeID,
		Candidates:                 make([]clusterSchedulerCandidateView, 0, len(insight.Candidates)),
	}
	if insight.RecommendedDecision != nil {
		view.RecommendedDecision = toClusterSchedulerDecisionPreviewView(*insight.RecommendedDecision)
	}
	for _, item := range insight.Candidates {
		view.Candidates = append(view.Candidates, clusterSchedulerCandidateView{
			NodeID:                    item.NodeID,
			NodeName:                  item.NodeName,
			Enabled:                   item.Enabled,
			Quarantined:               item.Quarantined,
			Draining:                  item.Draining,
			Shielded:                  item.Shielded,
			SupportsHardwareWatermark: item.SupportsHardwareWatermark,
			MaxTranscodeSessions:      item.MaxTranscodeSessions,
			MaxUploadConcurrency:      item.MaxUploadConcurrency,
			MetricsAvailable:          item.MetricsAvailable,
			MetricsFresh:              item.MetricsFresh,
			Online:                    item.Online,
			LastMetricsAt:             item.LastMetricsAt,
			LastHeartbeatAt:           item.LastHeartbeatAt,
			Score:                     item.Score,
			ScoreRank:                 item.ScoreRank,
			FilterPassed:              item.FilterPassed,
			FilterReasons:             item.FilterReasons,
			InTopKPool:                item.InTopKPool,
			DecisionPreview:           toClusterSchedulerDecisionPreviewViewPtr(item.DecisionPreview),
			Metrics: clusterSchedulerMetricsView{
				CPUUsagePercent:         item.Metrics.CPUUsagePercent,
				MemoryUsagePercent:      item.Metrics.MemoryUsagePercent,
				GPUMemoryUsagePercent:   item.Metrics.GPUMemoryUsagePercent,
				UploadQueueDepth:        item.Metrics.UploadQueueDepth,
				ActiveTranscodeSessions: item.Metrics.ActiveTranscodeSessions,
				GPUCapabilities:         item.Metrics.GPUCapabilities,
			},
		})
	}
	return view
}

func toClusterSchedulerDecisionPreviewViewPtr(decision *model.DispatchDecision) *clusterSchedulerDecisionPreviewView {
	if decision == nil {
		return nil
	}
	return toClusterSchedulerDecisionPreviewView(*decision)
}

func toClusterSchedulerDecisionPreviewView(decision model.DispatchDecision) *clusterSchedulerDecisionPreviewView {
	return &clusterSchedulerDecisionPreviewView{
		NodeID:              decision.NodeID,
		SelectedGPUIndex:    decision.SelectedGPUIndex,
		SelectedGPUDeviceID: decision.SelectedGPUDeviceID,
		SelectedExecutionHW: decision.SelectedExecutionHW,
		LeaseGeneration:     decision.LeaseGeneration,
		AttemptNo:           decision.AttemptNo,
	}
}

func workerStatusName(status int) string {
	switch status {
	case 1:
		return "online"
	case 2:
		return "offline"
	case 3:
		return "exited"
	default:
		return "unknown"
	}
}
