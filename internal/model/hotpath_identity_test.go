package model

import "testing"

func TestHeartbeatRequest_IdentityFields(t *testing.T) {
	req := HeartbeatRequest{
		NodeID:             1,
		WorkerID:           "worker-1",
		StartupInstanceID:  "worker-1-startup",
		MachineFingerprint: "node-worker-1",
	}
	if req.StartupInstanceID == "" {
		t.Fatalf("startup instance id should not be empty")
	}
	if req.MachineFingerprint == "" {
		t.Fatalf("machine fingerprint should not be empty")
	}
}
