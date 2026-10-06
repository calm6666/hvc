package gate

import (
	"testing"

	"hvc/internal/model"
)

/* ---------- A9 回调幂等 ---------- */

func TestCallbackIdempotencySingleEventPasses(t *testing.T) {
	events := []model.OutboxEvent{
		{EventID: 1, EventType: "transcode.completed", JobID: 7},
		{EventID: 2, EventType: "transcode.failed", JobID: 7},
	}

	result := RunCallbackIdempotency(7, events)

	if result.Skipped || !result.Passed {
		t.Fatalf("单条完成事件应通过：passed=%v skipped=%v detail=%s", result.Passed, result.Skipped, result.Detail)
	}
}

func TestCallbackIdempotencyDuplicateDetected(t *testing.T) {
	events := []model.OutboxEvent{
		{EventID: 1, EventType: "transcode.completed", JobID: 7},
		{EventID: 2, EventType: "transcode.completed", JobID: 7},
	}

	result := RunCallbackIdempotency(7, events)

	if result.Passed || result.Skipped {
		t.Fatalf("重复完成事件必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestCallbackIdempotencyOtherJobIgnored(t *testing.T) {
	events := []model.OutboxEvent{
		{EventID: 1, EventType: "transcode.completed", JobID: 7},
		{EventID: 2, EventType: "transcode.completed", JobID: 8},
	}

	result := RunCallbackIdempotency(7, events)

	if !result.Passed {
		t.Fatalf("别的任务的事件不应影响本任务判定：%s", result.Detail)
	}
}

func TestCallbackIdempotencyMissingEventSkips(t *testing.T) {
	result := RunCallbackIdempotency(7, nil)

	if !result.Skipped || result.Passed {
		t.Fatalf("没有事件时必须 Skipped 且不算通过：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

/* ---------- A10 断点续跑自洽 ---------- */

func TestResumeConsistencyPrefixPasses(t *testing.T) {
	steps := []StepRecord{
		{Step: "PROBE", State: StepDone, InputHash: "h1"},
		{Step: "PLAN", State: StepDone, InputHash: "h1"},
		{Step: "SEGMENT", State: StepDone, InputHash: "h1"},
		{Step: "UPLOAD", State: StepRunning, InputHash: "h1"},
	}

	result := RunResumeConsistency("h1", steps)

	if !result.Passed || result.Skipped {
		t.Fatalf("连续前缀应通过：passed=%v skipped=%v detail=%s", result.Passed, result.Skipped, result.Detail)
	}
}

func TestResumeConsistencyForeignFingerprintDetected(t *testing.T) {
	steps := []StepRecord{
		{Step: "PROBE", State: StepDone, InputHash: "h1"},
		{Step: "PLAN", State: StepDone, InputHash: "h2"},
	}

	result := RunResumeConsistency("h1", steps)

	if result.Passed || result.Skipped {
		t.Fatalf("混入其它指纹的步骤必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestResumeConsistencyGapAfterUnfinishedDetected(t *testing.T) {
	steps := []StepRecord{
		{Step: "PROBE", State: StepDone, InputHash: "h1"},
		{Step: "PLAN", State: StepFailed, InputHash: "h1"},
		{Step: "SEGMENT", State: StepDone, InputHash: "h1"},
	}

	result := RunResumeConsistency("h1", steps)

	if result.Passed || result.Skipped {
		t.Fatalf("未完成步骤之后又出现完成态必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestResumeConsistencyMissingFingerprintSkips(t *testing.T) {
	result := RunResumeConsistency("", []StepRecord{{Step: "PROBE", State: StepDone}})

	if !result.Skipped || result.Passed {
		t.Fatalf("任务无指纹时必须 Skipped：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

/* ---------- A11 失败收敛 ---------- */

func TestFailureConvergenceFailedWithEvidencePasses(t *testing.T) {
	job := model.TranscodeJob{JobID: 3, Status: model.JobStatusFailed, ErrorCode: "RUN_FAILED", ErrorMessage: "ffmpeg exit 1"}

	result := RunFailureConvergence(job)

	if !result.Passed || result.Skipped {
		t.Fatalf("有据可查的失败应通过：passed=%v skipped=%v detail=%s", result.Passed, result.Skipped, result.Detail)
	}
}

func TestFailureConvergenceSilentFailureDetected(t *testing.T) {
	job := model.TranscodeJob{JobID: 3, Status: model.JobStatusFailed}

	result := RunFailureConvergence(job)

	if result.Passed || result.Skipped {
		t.Fatalf("静默失败必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestFailureConvergenceResidualErrorOnSuccessDetected(t *testing.T) {
	job := model.TranscodeJob{JobID: 3, Status: model.JobStatusPublished, ErrorCode: "STALE", ErrorMessage: "leftover"}

	result := RunFailureConvergence(job)

	if result.Passed || result.Skipped {
		t.Fatalf("终态残留错误信息必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestFailureConvergenceRunningSkips(t *testing.T) {
	result := RunFailureConvergence(model.TranscodeJob{JobID: 3, Status: model.JobStatusRunning})

	if !result.Skipped || result.Passed {
		t.Fatalf("未到终态时必须 Skipped：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

/* ---------- A12 进度合法性 ---------- */

func TestProgressSanityCompletedAtFullPasses(t *testing.T) {
	snapshots := []model.ProgressSnapshot{
		{JobID: 5, Status: model.JobStatusRunning, ProgressPermille: 300},
		{JobID: 5, Status: model.JobStatusUploading, ProgressPermille: 800},
		{JobID: 5, Status: model.JobStatusCompleted, ProgressPermille: 1000},
	}

	result := RunProgressSanity(snapshots)

	if !result.Passed || result.Skipped {
		t.Fatalf("合法进度应通过：passed=%v skipped=%v detail=%s", result.Passed, result.Skipped, result.Detail)
	}
}

func TestProgressSanityOutOfRangeDetected(t *testing.T) {
	snapshots := []model.ProgressSnapshot{{JobID: 5, Status: model.JobStatusRunning, ProgressPermille: 1200}}

	result := RunProgressSanity(snapshots)

	if result.Passed || result.Skipped {
		t.Fatalf("越界进度必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestProgressSanityCompletedNotFullDetected(t *testing.T) {
	snapshots := []model.ProgressSnapshot{{JobID: 5, Status: model.JobStatusCompleted, ProgressPermille: 970}}

	result := RunProgressSanity(snapshots)

	if result.Passed || result.Skipped {
		t.Fatalf("终态非满进度必须被判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestProgressSanityEmptySkips(t *testing.T) {
	result := RunProgressSanity(nil)

	if !result.Skipped || result.Passed {
		t.Fatalf("无快照时必须 Skipped：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}
