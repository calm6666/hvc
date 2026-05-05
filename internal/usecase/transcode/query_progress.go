package transcode

import (
	"context"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
)

// QueryProgressUseCase 表示任务进度查询用例。
type QueryProgressUseCase struct {
	jobRepository *mysql.JobRepository
	progressStore *rediscache.ProgressStore
}

// NewQueryProgressUseCase 创建任务进度查询用例。
func NewQueryProgressUseCase(jobRepository *mysql.JobRepository, progressStore *rediscache.ProgressStore) *QueryProgressUseCase {
	return &QueryProgressUseCase{jobRepository: jobRepository, progressStore: progressStore}
}

// Execute 查询任务进度。
func (u *QueryProgressUseCase) Execute(ctx context.Context, requestID string) (model.ProgressSnapshot, error) {
	if requestID == "" {
		return model.ProgressSnapshot{}, ErrRequestIDRequired
	}
	job, ok := u.jobRepository.FindByRequestID(ctx, requestID)
	if !ok {
		return model.ProgressSnapshot{}, ErrJobNotFound
	}
	if snapshot, ok := u.progressStore.Get(ctx, job.JobID); ok {
		return snapshot, nil
	}
	return model.ProgressSnapshot{
		JobID:                job.JobID,
		Status:               job.Status,
		Stage:                job.ProgressStage,
		ProgressPermille:     job.ProgressPermille,
		CurrentFPS:           0,
		CurrentBitrateKbps:   0,
		CurrentSpeed:         0,
		ElapsedMS:            0,
		EstimatedRemainingMS: 0,
	}, nil
}
