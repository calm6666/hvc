package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// JobStepRepository 读写 t_transcode_job_step：B2 断点续跑的进度锚点。
type JobStepRepository struct {
	db *DB
}

// NewJobStepRepository 创建步骤仓储。
func NewJobStepRepository(db *DB) *JobStepRepository {
	return &JobStepRepository{db: db}
}

// UpsertStep 写入或更新"某任务某步骤"的状态。
//
// 唯一键是 (job_id, step)，这里用"先查后写"而不是依赖具体数据库的 upsert 语法：
//   - 首次出现该步骤 ⇒ Create（attempt 从 1 起算，因为这一次就是第一次尝试）；
//   - 已存在 ⇒ 只更新状态相关列，input_hash 只在调用方给出非空值时覆盖
//     （避免步骤推进时把首次记录下的指纹抹掉）。
//
// state 语义见 JobStepNotStarted/Running/Done/Failed；进入 Running 时累加 attempt 并记开始时间，
// 进入 Done/Failed 时记结束时间。
func (r *JobStepRepository) UpsertStep(ctx context.Context, jobID uint64, step string, state int, inputHash string, detail *string) error {
	if r == nil || r.db == nil {
		return nil
	}

	now := time.Now()
	updates := map[string]any{
		"state":      state,
		"updated_at": now,
	}

	if inputHash != "" {
		updates["input_hash"] = inputHash
	}

	if detail != nil {
		updates["detail"] = *detail
	}

	switch state {
	case JobStepRunning:
		updates["started_at"] = now
		updates["attempt"] = gorm.Expr("attempt + 1")

	case JobStepDone, JobStepFailed:
		updates["finished_at"] = now
	}

	record := JobStepRecord{}
	err := r.db.WithContext(ctx).Where("job_id = ? AND step = ?", jobID, step).First(&record).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(&JobStepRecord{
			JobID:     jobID,
			Step:      step,
			State:     state,
			InputHash: inputHash,
			Detail:    detail,
			Attempt:   attemptOf(state),
			StartedAt: startedAtOf(state, now),
			CreatedAt: now,
			UpdatedAt: now,
		}).Error
	}

	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Model(&JobStepRecord{}).
		Where("job_id = ? AND step = ?", jobID, step).
		Updates(updates).Error
}

// ListByJob 返回某任务的全部步骤行（按主键顺序，即首次出现顺序）。
// 找不到或出错时返回空切片：续跑判断是"能不能少做几步"的优化，读不到就按整任务跑。
func (r *JobStepRepository) ListByJob(ctx context.Context, jobID uint64) []JobStepRecord {
	if r == nil || r.db == nil {
		return nil
	}

	var records []JobStepRecord
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Order("id ASC").Find(&records).Error; err != nil {
		return nil
	}

	return records
}

// IsStepDone 判断某步骤是否已完成 ⇒ 续跑时用它跳过已完成的前缀步骤。
// 查询失败按"未完成"处理：宁可重做一步，也不要漏做一步。
func (r *JobStepRepository) IsStepDone(ctx context.Context, jobID uint64, step string) bool {
	if r == nil || r.db == nil {
		return false
	}

	var count int64
	if err := r.db.WithContext(ctx).Model(&JobStepRecord{}).
		Where("job_id = ? AND step = ? AND state = ?", jobID, step, JobStepDone).
		Count(&count).Error; err != nil {
		return false
	}

	return count > 0
}

// MarkStepDone 把步骤标为已完成（成功路径的统一出口）。
func (r *JobStepRepository) MarkStepDone(ctx context.Context, jobID uint64, step string, inputHash string, detail *string) error {
	return r.UpsertStep(ctx, jobID, step, JobStepDone, inputHash, detail)
}

// attemptOf 返回新建步骤行时的 attempt 初值：Running 表示"这一次就是第一次尝试"。
func attemptOf(state int) int {
	if state == JobStepRunning {
		return 1
	}

	return 0
}

// startedAtOf 返回新建步骤行时的开始时间：只有 Running 才算已经开始。
func startedAtOf(state int, now time.Time) *time.Time {
	if state != JobStepRunning {
		return nil
	}

	return &now
}

/*
 * JobRepository 上的步骤读写入口。
 *
 * 为什么做成薄封装而不是给 worker 再注入一个 JobStepRepository：步骤表与任务表在同一个 *DB 上，
 * 而 JobRepository 已经注入到 worker（以及其它所有调用点）。多注入一个依赖会连带改 NewModule 的
 * 全部构造点与测试，收益为零；这里转发一层即可。
 */
func (r *JobRepository) steps() *JobStepRepository {
	if r == nil {
		return nil
	}

	return &JobStepRepository{db: r.db}
}

// UpsertJobStep 写入/更新某任务某步骤的状态（见 JobStepRepository.UpsertStep）。
func (r *JobRepository) UpsertJobStep(ctx context.Context, jobID uint64, step string, state int, inputHash string, detail *string) error {
	return r.steps().UpsertStep(ctx, jobID, step, state, inputHash, detail)
}

// IsJobStepDone 判断某步骤是否已完成（续跑时跳过已完成的前缀步骤）。
func (r *JobRepository) IsJobStepDone(ctx context.Context, jobID uint64, step string) bool {
	return r.steps().IsStepDone(ctx, jobID, step)
}

// ListJobSteps 返回某任务的全部步骤行。
func (r *JobRepository) ListJobSteps(ctx context.Context, jobID uint64) []JobStepRecord {
	return r.steps().ListByJob(ctx, jobID)
}

// DeleteJobSteps 清空某任务的步骤行：输入指纹变了必须整任务重跑，旧的步骤进度不能再被当成续跑依据。
func (r *JobRepository) DeleteJobSteps(ctx context.Context, jobID uint64) error {
	if r == nil || r.db == nil {
		return nil
	}

	return r.db.WithContext(ctx).Where("job_id = ?", jobID).Delete(&JobStepRecord{}).Error
}
