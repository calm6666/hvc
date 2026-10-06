package worker

import (
	"context"
	"testing"
	"time"

	"hvc/internal/cluster"
)

func TestModuleSetOfflineStopsNewAssignments(t *testing.T) {
	module := &Module{workerID: "worker-a"}
	if err := module.SetOffline(context.Background(), "worker-a", "manual isolate"); err != nil {
		t.Fatalf("set offline failed: %v", err)
	}
	if module.acceptingNewAssignments() {
		t.Fatal("offline worker should stop accepting new assignments")
	}
}

func TestModuleRequestExitMarksExitRequested(t *testing.T) {
	module := &Module{workerID: "worker-a"}
	if err := module.RequestExit(context.Background(), "worker-a", "operator_exit"); err != nil {
		t.Fatalf("request exit failed: %v", err)
	}
	if module.acceptingNewAssignments() {
		t.Fatal("exiting worker should stop accepting new assignments")
	}
	if got := module.shutdownReason(nil); got != "operator_exit" {
		t.Fatalf("unexpected shutdown reason: %s", got)
	}
}

func TestModuleControlRejectsForeignWorkerID(t *testing.T) {
	module := &Module{workerID: "worker-a"}
	if err := module.SetOffline(context.Background(), "worker-b", "manual isolate"); err == nil {
		t.Fatal("expected foreign worker offline request to fail")
	}
	if err := module.RequestExit(context.Background(), "worker-b", "operator_exit"); err == nil {
		t.Fatal("expected foreign worker exit request to fail")
	}
}

func TestResolveDBHeartbeatPersistInterval(t *testing.T) {
	testCases := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{name: "default when timeout invalid", timeout: 0, want: 20 * time.Second / 3},
		{name: "minimum clamp", timeout: 12 * time.Second, want: 5 * time.Second},
		{name: "normal third", timeout: 30 * time.Second, want: 10 * time.Second},
		{name: "maximum clamp", timeout: 90 * time.Second, want: 15 * time.Second},
	}
	for _, tc := range testCases {
		if got := cluster.ResolveHeartbeatPersistInterval(tc.timeout); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestModuleShouldPersistDBHeartbeat(t *testing.T) {
	module := &Module{}
	now := time.Now()
	if !module.shouldPersistDBHeartbeat(now, time.Time{}, 30*time.Second) {
		t.Fatal("first heartbeat should persist")
	}
	if module.shouldPersistDBHeartbeat(now, now.Add(-9*time.Second), 30*time.Second) {
		t.Fatal("heartbeat within throttle interval should not persist")
	}
	if !module.shouldPersistDBHeartbeat(now, now.Add(-10*time.Second), 30*time.Second) {
		t.Fatal("heartbeat at throttle interval should persist")
	}
}
