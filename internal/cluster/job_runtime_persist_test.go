package cluster

import (
	"testing"
	"time"

	"hvc/internal/model"
)

func TestJobRuntimePersistGate_ShouldPersistProgress(t *testing.T) {
	gate := NewJobRuntimePersistGate()
	now := time.Now()
	snapshot := model.ProgressSnapshot{
		JobID:            1001,
		Status:           model.JobStatusRunning,
		Stage:            model.StageTranscoding,
		ProgressPermille: 120,
	}
	if !gate.ShouldPersistProgress(snapshot, now) {
		t.Fatal("first progress snapshot should persist")
	}
	gate.MarkProgressPersisted(snapshot, now)

	if gate.ShouldPersistProgress(snapshot, now.Add(2*time.Second)) {
		t.Fatal("same snapshot within interval should not persist")
	}

	stageChanged := snapshot
	stageChanged.Stage = model.StageUploading
	if !gate.ShouldPersistProgress(stageChanged, now.Add(2*time.Second)) {
		t.Fatal("stage change should persist immediately")
	}

	progressJump := snapshot
	progressJump.ProgressPermille = snapshot.ProgressPermille + jobProgressPersistDeltaPermille
	if !gate.ShouldPersistProgress(progressJump, now.Add(2*time.Second)) {
		t.Fatal("large progress jump should persist immediately")
	}

	if !gate.ShouldPersistProgress(snapshot, now.Add(jobProgressPersistInterval)) {
		t.Fatal("progress snapshot at interval should persist")
	}
}

func TestJobRuntimePersistGate_ShouldPersistExecutionHeartbeat(t *testing.T) {
	gate := NewJobRuntimePersistGate()
	now := time.Now()
	timeout := 30 * time.Second
	if !gate.ShouldPersistExecutionHeartbeat(now, 1001, 3, timeout) {
		t.Fatal("first execution heartbeat should persist")
	}
	gate.MarkExecutionHeartbeatPersisted(1001, 3, now)
	if gate.ShouldPersistExecutionHeartbeat(now.Add(9*time.Second), 1001, 3, timeout) {
		t.Fatal("execution heartbeat within interval should not persist")
	}
	if !gate.ShouldPersistExecutionHeartbeat(now.Add(10*time.Second), 1001, 3, timeout) {
		t.Fatal("execution heartbeat at interval should persist")
	}
}
