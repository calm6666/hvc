package cluster

import (
	"testing"
	"time"
)

func TestCoordinatorShouldPersistHeartbeat(t *testing.T) {
	coord := &Coordinator{}
	now := time.Now()
	if !coord.shouldPersistHeartbeat(now) {
		t.Fatal("first heartbeat should persist")
	}
	coord.lastHeartbeatPersist = now.Add(-10 * time.Second)
	if coord.shouldPersistHeartbeat(now) {
		t.Fatal("heartbeat within throttle interval should not persist")
	}
	coord.lastHeartbeatPersist = now.Add(-coordinatorHeartbeatPersistInterval)
	if !coord.shouldPersistHeartbeat(now) {
		t.Fatal("heartbeat at throttle interval should persist")
	}
}
