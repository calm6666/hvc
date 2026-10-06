package cluster

import (
	"testing"
	"time"
)

func TestResolveHeartbeatPersistInterval(t *testing.T) {
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
		if got := ResolveHeartbeatPersistInterval(tc.timeout); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestHeartbeatPersistGate(t *testing.T) {
	gate := NewHeartbeatPersistGate()
	now := time.Now()
	timeout := 30 * time.Second

	if !gate.ShouldPersistNode(now, 1, timeout) {
		t.Fatal("first node heartbeat should persist")
	}
	gate.MarkNodePersisted(1, now)
	if gate.ShouldPersistNode(now.Add(9*time.Second), 1, timeout) {
		t.Fatal("node heartbeat within throttle interval should not persist")
	}
	if !gate.ShouldPersistNode(now.Add(10*time.Second), 1, timeout) {
		t.Fatal("node heartbeat at throttle interval should persist")
	}

	if !gate.ShouldPersistWorker(now, "worker-1", timeout) {
		t.Fatal("first worker heartbeat should persist")
	}
	gate.MarkWorkerPersisted("worker-1", now)
	if gate.ShouldPersistWorker(now.Add(9*time.Second), "worker-1", timeout) {
		t.Fatal("worker heartbeat within throttle interval should not persist")
	}
	if !gate.ShouldPersistWorker(now.Add(10*time.Second), "worker-1", timeout) {
		t.Fatal("worker heartbeat at throttle interval should persist")
	}
}
