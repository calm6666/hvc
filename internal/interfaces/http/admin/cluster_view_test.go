package admin

import (
	"encoding/json"
	"testing"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	schedulerview "hvc/internal/scheduler"
)

func TestClusterNodeListItemViewUsesSnakeCaseAndNoNestedMetricTimestamp(t *testing.T) {
	lastMetricsAt := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
	view := toClusterNodeListItemView(ClusterNodeView{
		MetricsAvailable: true,
		OnlineEstimate:   true,
		ClusterNodeRecord: mysql.ClusterNodeRecord{
			Draining:    true,
			DrainReason: "rolling_upgrade",
		},
		LastMetricsAt:    &lastMetricsAt,
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
			NodeID:       10,
			NodeName:     "node-a",
			Draining:     true,
			FilterPassed: true,
			InTopKPool:   true,
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
