package scheduler

import (
	"testing"

	"hvc/internal/model"
)

func TestMergeGPUActiveSessions(t *testing.T) {
	metrics := model.NodeMetrics{
		NodeID: 1,
		GPUCapabilities: []model.GPUCapability{
			{GPUIndex: 0, ActiveSessions: 1},
			{GPUIndex: 1, ActiveSessions: 2},
		},
	}

	merged := mergeGPUActiveSessions(metrics, map[int]int{
		0: 3,
		1: 5,
	})

	if merged.GPUCapabilities[0].ActiveSessions != 3 {
		t.Fatalf("unexpected gpu 0 active sessions: %d", merged.GPUCapabilities[0].ActiveSessions)
	}
	if merged.GPUCapabilities[1].ActiveSessions != 5 {
		t.Fatalf("unexpected gpu 1 active sessions: %d", merged.GPUCapabilities[1].ActiveSessions)
	}
}
