package cluster

import "testing"

func TestAdminClusterSummaryCacheKey(t *testing.T) {
	key := AdminClusterSummaryCacheKey("overview", 7, 99)
	if key != "hvc:admin:cluster:overview:node:7:version:99" {
		t.Fatalf("unexpected key: %s", key)
	}
}

func TestMonitorSnapshotCacheKey(t *testing.T) {
	key := MonitorSnapshotCacheKey("cluster-control")
	if key != "hvc:monitor:snapshot:mode:cluster-control" {
		t.Fatalf("unexpected key: %s", key)
	}
}
