package gate

import (
	"fmt"

	"hvc/internal/model"
)

/*
 * 本文件是门禁的第二批判据：A9 回调幂等、A10 断点续跑自洽、A11 失败收敛、A12 进度真实性。
 * 它们与 gate.go 里的 A4/A5/A6/A7 一样是纯数据判定（不需要对象存储/HTTP/硬件信息），
 * 因此可以在单测里直接构造输入；判据来源同样是**推导**（见 gate.go 的包注释）。
 */

/*
 * 步骤状态取值镜像 sql/109_transcode_job_step_schema.sql 与 mysql 包里的 JobStep* 常量。
 *
 * 为什么不直接 import mysql：门禁包的输入是"行数据"，让调用方把 mysql 的行类型转成这里的
 * 中性结构，可以让门禁不依赖持久层实现（换存储、写测试都不受影响）。镜像关系由
 * TestStepStateMirrorsSchema 用例守住。
 */
const (
	StepNotStarted = 0
	StepRunning    = 1
	StepDone       = 2
	StepFailed     = 3
)

// StepRecord 门禁视角的"步骤行"（对应 t_transcode_job_step 的一行）。
type StepRecord struct {
	Step      string
	State     int
	InputHash string
}

// RunCallbackIdempotency A9（推导）：同一任务只允许存在一条完成回调事件。
//
// 为什么这是判据而不是实现细节：A1 把"发回调"推迟到内容完整之后，靠的是 TakePendingCompletion
// 的比较交换来保证只发一次。判据在这里对**结果**做校验 —— 即使实现被改坏、或者两个节点同时发布，
// 只要库里出现两条 transcode.completed，门禁就必须报出来。
func RunCallbackIdempotency(jobID uint64, events []model.OutboxEvent) CheckResult {
	const id = "A9"
	const title = "同一任务只有一条完成回调事件（推导）"

	const completedEventType = "transcode.completed"

	completed := 0

	for _, event := range events {
		if event.EventType != completedEventType {
			continue
		}

		if event.JobID != jobID {
			continue
		}

		completed++

		if completed > 1 {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("任务 %d 存在多条 %s 事件（至少 event_id=%d 是重复的）",
					jobID, completedEventType, event.EventID),
			}
		}
	}

	if completed == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有任何完成回调事件，无法判定"}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// RunResumeConsistency A10（推导）：步骤行必须与任务行上的输入指纹自洽、且完成状态是连续前缀。
//
// 判据内容：
//   - 每条步骤行的 input_hash 必须为空或等于任务行的 input_hash —— 出现第三种取值说明步骤是在
//     另一版输入下留下的，续跑会拿错中间产物；
//   - 完成状态必须是前缀：一旦出现未完成的步骤，其后的步骤不允许是完成态（否则续跑会跳过缺口）。
func RunResumeConsistency(jobInputHash string, steps []StepRecord) CheckResult {
	const id = "A10"
	const title = "步骤指纹与任务一致且完成状态连续（推导）"

	if len(steps) == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有步骤行，无法判定"}
	}

	if jobInputHash == "" {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "任务行未记录输入指纹，无法判定"}
	}

	for _, step := range steps {
		if step.State < StepNotStarted || step.State > StepFailed {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("步骤 %s 的状态取值非法：%d", step.Step, step.State),
			}
		}

		if step.InputHash != "" && step.InputHash != jobInputHash {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("步骤 %s 的指纹与任务不一致：步骤=%s 任务=%s",
					step.Step, step.InputHash, jobInputHash),
			}
		}
	}

	gap := false

	for _, step := range steps {
		if step.State != StepDone {
			gap = true
			continue
		}

		if gap {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("步骤 %s 已完成，但它前面存在未完成的步骤（完成态必须是前缀）", step.Step),
			}
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// RunFailureConvergence A11（推导）：失败必须是一次"有据可查"的收敛，而不是静默。
//
// 判据内容：任务处于 FAILED 时，必须带错误码与错误描述；处于终态（COMPLETED 及其之后的
// PUBLISHED / CALLBACK_SENT）时不允许残留错误信息。
func RunFailureConvergence(job model.TranscodeJob) CheckResult {
	const id = "A11"
	const title = "失败收敛且不静默（推导）"

	switch job.Status {
	case model.JobStatusFailed:
		if job.ErrorCode == "" || job.ErrorMessage == "" {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("任务 %d 处于失败态但缺少错误信息：code=%q message=%q",
					job.JobID, job.ErrorCode, job.ErrorMessage),
			}
		}

	case model.JobStatusCompleted, model.JobStatusPublished, model.JobStatusCallbackSent:
		if job.ErrorCode != "" || job.ErrorMessage != "" {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("任务 %d 已到终态却残留错误信息：code=%q message=%q",
					job.JobID, job.ErrorCode, job.ErrorMessage),
			}
		}

	default:
		return CheckResult{
			ID: id, Title: title, Skipped: true,
			Detail: fmt.Sprintf("任务 %d 尚未到终态（status=%d），无法判定", job.JobID, job.Status),
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// RunProgressSanity A12（推导）：进度快照必须落在合法区间，且到达终态时是满进度。
//
// 这里刻意**不判"进度单调不减"**：往回 seek、重跑、换清晰度都会让进度合法地回退，
// 只看快照序列无法区分"合法回退"和"抖动/错报"。能无争议判定的只有两条：
// 取值必须在 [0,1000]，以及到终态的最后一个快照必须是 1000。
func RunProgressSanity(snapshots []model.ProgressSnapshot) CheckResult {
	const id = "A12"
	const title = "进度取值合法且终态满进度（推导）"

	if len(snapshots) == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有进度快照，无法判定"}
	}

	for _, snapshot := range snapshots {
		if snapshot.ProgressPermille < 0 || snapshot.ProgressPermille > 1000 {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("任务 %d 的进度越界：%d‰", snapshot.JobID, snapshot.ProgressPermille),
			}
		}
	}

	last := snapshots[len(snapshots)-1]

	switch last.Status {
	case model.JobStatusCompleted, model.JobStatusPublished, model.JobStatusCallbackSent:
		if last.ProgressPermille != 1000 {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("任务 %d 已到终态但进度是 %d‰，不是 1000‰", last.JobID, last.ProgressPermille),
			}
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}
