package transcode

import (
	"context"
	"hvc/internal/model"
	transcodeusecase "hvc/internal/usecase/transcode"
)

// Service 表示转码服务。
type Service struct {
	createJobUseCase     *transcodeusecase.CreateJobUseCase
	queryProgressUseCase *transcodeusecase.QueryProgressUseCase
}

// NewService 创建转码服务。
func NewService(createJobUseCase *transcodeusecase.CreateJobUseCase, queryProgressUseCase *transcodeusecase.QueryProgressUseCase) *Service {
	return &Service{
		createJobUseCase:     createJobUseCase,
		queryProgressUseCase: queryProgressUseCase,
	}
}

// CreateJob 创建转码任务。
func (s *Service) CreateJob(ctx context.Context, req model.CreateJobRequest) (model.CreateJobResponseData, error) {
	result, err := s.createJobUseCase.Execute(req)
	if err != nil {
		return model.CreateJobResponseData{}, err
	}
	return model.CreateJobResponseData{
		JobID:      result.Job.JobID,
		RequestID:  result.Job.RequestID,
		Status:     result.Job.Status,
		StatusName: result.Job.ProgressStage,
	}, nil
}

// QueryProgress 查询转码进度。
func (s *Service) QueryProgress(ctx context.Context, requestID string) (model.ProgressSnapshot, error) {
	return s.queryProgressUseCase.Execute(ctx, requestID)
}
