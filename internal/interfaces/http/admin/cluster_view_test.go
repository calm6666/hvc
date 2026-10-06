package admin

import (
	"encoding/json"
	"testing"
	"time"

	"hvc/internal/audit"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	schedulerview "hvc/internal/scheduler"
)

func TestClusterNodeListItemViewUsesSnakeCaseAndNoNestedMetricTimestamp(t *testing.T) {
	lastMetricsAt := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
	view := toClusterNodeListItemView(ClusterNodeView{
		MetricsAvailable: true,
		MetricsFresh:     true,
		OnlineEstimate:   true,
		ClusterNodeRecord: mysql.ClusterNodeRecord{
			NodeTags:    "mode:cluster-allinone",
			Enabled:     true,
			HTTPHost:    ":8080",
			Draining:    true,
			DrainReason: "rolling_upgrade",
		},
		LastMetricsAt: &lastMetricsAt,
		Metrics: &model.NodeMetrics{
			CPUUsagePercent:         55,
			MemoryUsagePercent:      61,
			GPUMemoryUsagePercent:   42,
			UploadQueueDepth:        3,
			ActiveTranscodeSessions: 2,
			Timestamp:               lastMetricsAt,
		},
	})

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal cluster node list item failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal cluster node list item failed: %v", err)
	}

	if _, ok := payload["last_metrics_at"]; !ok {
		t.Fatal("last_metrics_at should be present at top level")
	}
	if _, ok := payload["grpc_host"]; !ok {
		t.Fatal("grpc_host should use snake_case")
	}
	if value, ok := payload["node_role"].(string); !ok || value != "cluster-allinone" {
		t.Fatal("node_role should be present")
	}
	if value, ok := payload["control_plane"].(bool); !ok || !value {
		t.Fatal("control_plane should be present")
	}
	if value, ok := payload["admin_accessible"].(bool); !ok || !value {
		t.Fatal("admin_accessible should be present")
	}
	if value, ok := payload["online_signal_source"].(string); !ok || value != "metrics" {
		t.Fatal("online_signal_source should be metrics")
	}
	if value, ok := payload["draining"].(bool); !ok || !value {
		t.Fatal("draining should be present and true")
	}
	if value, ok := payload["drain_reason"].(string); !ok || value != "rolling_upgrade" {
		t.Fatal("drain_reason should be present")
	}
	if _, ok := payload["grpcHost"]; ok {
		t.Fatal("grpcHost camelCase key should not exist")
	}

	metrics, ok := payload["metrics"].(map[string]any)
	if !ok {
		t.Fatal("metrics should exist as object")
	}
	if _, ok := metrics["timestamp"]; ok {
		t.Fatal("nested metrics.timestamp should not be exposed when last_metrics_at already exists")
	}
}

func TestClusterNodeDetailViewKeepsGPURuntimeCapability(t *testing.T) {
	detail := toClusterNodeDetailView(ClusterNodeView{
		GPUDevices: []ClusterNodeGPUView{
			{
				RuntimeCapability: &model.GPUCapability{
					GPUUUID:     "gpu-uuid-1",
					GPUIndex:    0,
					MaxSessions: 6,
				},
			},
		},
	})

	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal cluster node detail failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal cluster node detail failed: %v", err)
	}
	devices, ok := payload["gpu_devices"].([]any)
	if !ok || len(devices) != 1 {
		t.Fatalf("unexpected gpu_devices payload: %+v", payload["gpu_devices"])
	}
	device, ok := devices[0].(map[string]any)
	if !ok {
		t.Fatal("gpu_devices[0] should be object")
	}
	if _, ok := device["runtime_capability"]; !ok {
		t.Fatal("runtime_capability should remain available in node detail")
	}
}

func TestClusterWorkerListItemViewUsesSnakeCaseAndOnlineEstimate(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	view := toClusterWorkerListItemView(mysql.WorkerInstanceRecord{
		ID:              1,
		NodeID:          2,
		WorkerID:        "worker-1",
		Status:          1,
		ExitReason:      "manual_offline",
		LastHeartbeatAt: now.Add(-time.Minute),
	}, now)

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal cluster worker list item failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal cluster worker list item failed: %v", err)
	}

	if _, ok := payload["online_estimate"]; !ok {
		t.Fatal("online_estimate should be present")
	}
	if _, ok := payload["startup_instance_id"]; !ok {
		t.Fatal("startup_instance_id should use snake_case")
	}
	if value, ok := payload["status_source"].(string); !ok || value != "heartbeat" {
		t.Fatal("status_source should be present")
	}
	if value, ok := payload["exit_reason"].(string); !ok || value != "manual_offline" {
		t.Fatal("exit_reason should use snake_case and preserve value")
	}
	if _, ok := payload["startupInstanceID"]; ok {
		t.Fatal("startupInstanceID camelCase key should not exist")
	}
}

func TestClusterSchedulerInsightViewUsesSnakeCase(t *testing.T) {
	view := toClusterSchedulerInsightView(schedulerview.DispatchInsight{
		GeneratedAt:                time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		TopK:                       3,
		DispatchPolicy:             "top_k_random",
		MaxGlobalTranscodeSessions: 24,
		ActiveExecutionCount:       5,
		QueuedJobCount:             2,
		CandidateTotal:             3,
		PassedCandidateTotal:       2,
		RecommendedNodeID:          10,
		RecommendedDecision: &model.DispatchDecision{
			NodeID:              10,
			SelectedGPUIndex:    1,
			SelectedGPUDeviceID: 99,
			SelectedExecutionHW: model.ExecutionHWNVIDIA,
			LeaseGeneration:     1,
			AttemptNo:           1,
		},
		Candidates: []schedulerview.CandidateInsight{{
			NodeID:             10,
			NodeName:           "node-a",
			NodeRole:           "cluster-allinone",
			Draining:           true,
			ControlPlane:       true,
			AdminAccessible:    true,
			OnlineSignalSource: "metrics",
			SchedulerReady:     true,
			StateReason:        "ready",
			FilterPassed:       true,
			InTopKPool:         true,
			Metrics: model.NodeMetrics{
				CPUUsagePercent: 20,
			},
		}},
	})

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal cluster scheduler insight view failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal cluster scheduler insight view failed: %v", err)
	}

	if _, ok := payload["recommended_decision"]; !ok {
		t.Fatal("recommended_decision should be present")
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) != 1 {
		t.Fatalf("unexpected candidates payload: %+v", payload["candidates"])
	}
	candidate, ok := candidates[0].(map[string]any)
	if !ok {
		t.Fatal("candidates[0] should be object")
	}
	if _, ok := candidate["filter_reasons"]; !ok {
		t.Fatal("filter_reasons should be present")
	}
	if value, ok := candidate["node_role"].(string); !ok || value != "cluster-allinone" {
		t.Fatal("node_role should be present on scheduler candidate")
	}
	if value, ok := candidate["control_plane"].(bool); !ok || !value {
		t.Fatal("control_plane should be present on scheduler candidate")
	}
	if value, ok := candidate["admin_accessible"].(bool); !ok || !value {
		t.Fatal("admin_accessible should be present on scheduler candidate")
	}
	if value, ok := candidate["online_signal_source"].(string); !ok || value != "metrics" {
		t.Fatal("online_signal_source should be present on scheduler candidate")
	}
	if value, ok := candidate["scheduler_ready"].(bool); !ok || !value {
		t.Fatal("scheduler_ready should be present on scheduler candidate")
	}
	if value, ok := candidate["state_reason"].(string); !ok || value != "ready" {
		t.Fatal("state_reason should be present on scheduler candidate")
	}
	if value, ok := candidate["draining"].(bool); !ok || !value {
		t.Fatal("draining should be present on scheduler candidate")
	}
	if _, ok := candidate["in_top_k_pool"]; !ok {
		t.Fatal("in_top_k_pool should use snake_case")
	}
	if _, ok := candidate["inTopKPool"]; ok {
		t.Fatal("inTopKPool camelCase key should not exist")
	}
}

func TestClusterMemberViewUsesSnakeCase(t *testing.T) {
	registryAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	lastMetricsAt := time.Date(2026, 5, 9, 12, 1, 0, 0, time.UTC)
	view := clusterMemberView{
		NodeID:              1,
		NodeName:            "node-a",
		NodeRole:            "cluster-control",
		Host:                ":8080",
		HostIP:              "10.0.0.1",
		GRPCHost:            ":9090",
		HTTPHost:            ":8080",
		Enabled:             true,
		Quarantined:         false,
		Draining:            true,
		ControlPlane:        true,
		AdminAccessible:     true,
		OnlineEstimate:      true,
		OnlineSignalSource:  "registry",
		SchedulerReady:      false,
		StateReason:         "node_record_missing",
		Source:              "node_table+registry",
		LastHeartbeatAt:     registryAt,
		RegistryHeartbeatAt: &registryAt,
		LastMetricsAt:       &lastMetricsAt,
		WorkerTotal:         2,
		WorkerOnlineTotal:   1,
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal cluster member view failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal cluster member view failed: %v", err)
	}

	if _, ok := payload["worker_online_total"]; !ok {
		t.Fatal("worker_online_total should be present")
	}
	if value, ok := payload["node_role"].(string); !ok || value != "cluster-control" {
		t.Fatal("node_role should be present")
	}
	if value, ok := payload["control_plane"].(bool); !ok || !value {
		t.Fatal("control_plane should be present")
	}
	if value, ok := payload["admin_accessible"].(bool); !ok || !value {
		t.Fatal("admin_accessible should be present")
	}
	if value, ok := payload["online_signal_source"].(string); !ok || value != "registry" {
		t.Fatal("online_signal_source should be present")
	}
	if value, ok := payload["scheduler_ready"].(bool); !ok || value {
		t.Fatal("scheduler_ready should be present and false")
	}
	if value, ok := payload["state_reason"].(string); !ok || value != "node_record_missing" {
		t.Fatal("state_reason should be present")
	}
	if value, ok := payload["draining"].(bool); !ok || !value {
		t.Fatal("draining should be present on member view")
	}
	if _, ok := payload["registry_heartbeat_at"]; !ok {
		t.Fatal("registry_heartbeat_at should use snake_case")
	}
	if _, ok := payload["registryHeartbeatAt"]; ok {
		t.Fatal("registryHeartbeatAt camelCase key should not exist")
	}
}

func TestEstimateNodeOnlineWithRegistryKeepsControlPlaneVisible(t *testing.T) {
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	nodeHeartbeat := now.Add(-10 * time.Minute)
	registryHeartbeat := now.Add(-30 * time.Second)

	online := estimateNodeOnlineWithRegistry(
		mysql.ClusterNodeRecord{LastHeartbeatAt: nodeHeartbeat},
		nil,
		&registryHeartbeat,
		now,
	)
	if !online {
		t.Fatal("control-plane node with fresh registry heartbeat should still be considered online")
	}
}

func TestResolveMemberOnlineSignalSourceUsesRegistryFallback(t *testing.T) {
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	nodeHeartbeat := now.Add(-10 * time.Minute)
	registryHeartbeat := now.Add(-30 * time.Second)

	source := resolveMemberOnlineSignalSource(
		mysql.ClusterNodeRecord{LastHeartbeatAt: nodeHeartbeat},
		nil,
		&registryHeartbeat,
		now,
	)
	if source != "registry" {
		t.Fatalf("unexpected online signal source: %s", source)
	}
}

func TestWorkerStatusNameMapping(t *testing.T) {
	if got := workerStatusName(1); got != "online" {
		t.Fatalf("unexpected status name for 1: %s", got)
	}
	if got := workerStatusName(2); got != "offline" {
		t.Fatalf("unexpected status name for 2: %s", got)
	}
	if got := workerStatusName(3); got != "exited" {
		t.Fatalf("unexpected status name for 3: %s", got)
	}
}

func TestResolveNodeOnlineSignalSourceFallsBackToNodeHeartbeat(t *testing.T) {
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	lastHeartbeat := now.Add(-time.Minute)
	source := resolveNodeOnlineSignalSource(ClusterNodeView{
		OnlineEstimate: true,
		ClusterNodeRecord: mysql.ClusterNodeRecord{
			LastHeartbeatAt: lastHeartbeat,
		},
	})
	if source != "node_heartbeat" {
		t.Fatalf("unexpected online signal source: %s", source)
	}
}

func TestClusterNodeResourceDistributionViewUsesSnakeCase(t *testing.T) {
	view := clusterNodeResourceDistributionView{
		NodeID:             1,
		NodeName:           "node-a",
		NodeRole:           "cluster-control",
		ControlPlane:       true,
		AdminAccessible:    true,
		OnlineEstimate:     true,
		OnlineSignalSource: "registry",
		SchedulerReady:     false,
		StateReason:        "metrics_missing",
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal cluster node resource distribution failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal cluster node resource distribution failed: %v", err)
	}

	if value, ok := payload["control_plane"].(bool); !ok || !value {
		t.Fatal("control_plane should be present")
	}
	if value, ok := payload["admin_accessible"].(bool); !ok || !value {
		t.Fatal("admin_accessible should be present")
	}
	if value, ok := payload["online_signal_source"].(string); !ok || value != "registry" {
		t.Fatal("online_signal_source should be present")
	}
	if _, ok := payload["onlineSignalSource"]; ok {
		t.Fatal("onlineSignalSource camelCase key should not exist")
	}
}

func TestNormalizeAdminBaseURL(t *testing.T) {
	if got := normalizeAdminBaseURL(":8888", "10.0.0.9"); got != "http://10.0.0.9:8888" {
		t.Fatalf("unexpected normalized admin base url: %s", got)
	}
	if got := normalizeAdminBaseURL("http://10.0.0.1:8888", "10.0.0.9"); got != "http://10.0.0.1:8888" {
		t.Fatalf("unexpected normalized admin base url: %s", got)
	}
	if got := normalizeAdminBaseURL("0.0.0.0:8888", "10.0.0.9"); got != "http://10.0.0.9:8888" {
		t.Fatalf("unexpected normalized admin base url: %s", got)
	}
}

func TestClusterGovernanceRecentActionViewUsesSnakeCase(t *testing.T) {
	view := toClusterGovernanceRecentActionView(audit.Item{
		AdminUserID:   1,
		Username:      "admin",
		ActionName:    "cluster.node.drain",
		TargetType:    "cluster_node",
		TargetID:      "1",
		ResultCode:    0,
		ResultMessage: `{"target_status":"draining","takeover_active_jobs":true,"takeover_result":{"matched_job_total":3,"reset_job_total":2,"abandoned_execution_total":1},"reason":"rolling_upgrade"}`,
		RequestIP:     "127.0.0.1:5000",
		CreatedAt:     time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
	})

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal governance recent action failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal governance recent action failed: %v", err)
	}
	if _, ok := payload["action_name"]; !ok {
		t.Fatal("action_name should be present")
	}
	if _, ok := payload["result_message"]; !ok {
		t.Fatal("result_message should be present")
	}
	result, ok := payload["result"].(map[string]any)
	if !ok {
		t.Fatal("result should be present as structured object")
	}
	if value, ok := result["target_status"].(string); !ok || value != "draining" {
		t.Fatal("result.target_status should be parsed")
	}
	if value, ok := result["takeover_active_jobs"].(bool); !ok || !value {
		t.Fatal("result.takeover_active_jobs should be parsed")
	}
	takeoverResult, ok := result["takeover_result"].(map[string]any)
	if !ok {
		t.Fatal("result.takeover_result should be present")
	}
	if value, ok := takeoverResult["matched_job_total"].(float64); !ok || value != 3 {
		t.Fatal("takeover_result.matched_job_total should be parsed")
	}
	extras, ok := result["extras"].(map[string]any)
	if !ok {
		t.Fatal("result.extras should keep unknown fields")
	}
	if value, ok := extras["reason"].(string); !ok || value != "rolling_upgrade" {
		t.Fatal("result.extras.reason should keep extra payload")
	}
	if _, ok := payload["adminUserID"]; ok {
		t.Fatal("camelCase adminUserID should not exist")
	}
}

func TestAccumulateGovernanceActionStats(t *testing.T) {
	var summary clusterGovernanceSummaryView
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{
		ActionName: "cluster.node.enable",
		Result:     &clusterGovernanceActionResultView{TargetStatus: "enabled"},
	})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{
		ActionName: "cluster.node.enable",
		Result:     &clusterGovernanceActionResultView{TargetStatus: "disabled"},
	})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{
		ActionName: "cluster.node.quarantine",
		Result:     &clusterGovernanceActionResultView{TargetStatus: "quarantined"},
	})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{
		ActionName: "cluster.node.quarantine",
		Result:     &clusterGovernanceActionResultView{TargetStatus: "active"},
	})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{
		ActionName: "cluster.node.drain",
		Result:     &clusterGovernanceActionResultView{TargetStatus: "draining"},
	})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{
		ActionName: "cluster.node.drain",
		Result:     &clusterGovernanceActionResultView{TargetStatus: "schedulable"},
	})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{ActionName: "cluster.worker.offline"})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{ActionName: "cluster.worker.exit"})
	accumulateGovernanceActionStats(&summary, clusterGovernanceRecentActionView{ActionName: "cluster.job.takeover"})

	if summary.NodeEnableTotal != 1 || summary.NodeDisableTotal != 1 {
		t.Fatalf("unexpected node enable stats: %+v", summary)
	}
	if summary.NodeQuarantineTotal != 1 || summary.NodeUnquarantineTotal != 1 {
		t.Fatalf("unexpected node quarantine stats: %+v", summary)
	}
	if summary.NodeDrainTotal != 1 || summary.NodeResumeTotal != 1 {
		t.Fatalf("unexpected node drain stats: %+v", summary)
	}
	if summary.WorkerOfflineTotal != 1 || summary.WorkerExitTotal != 1 || summary.JobTakeoverTotal != 1 {
		t.Fatalf("unexpected worker/takeover stats: %+v", summary)
	}
}
