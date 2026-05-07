package public

import (
	"context"

	"hvc/internal/model"
	transcodesvc "hvc/internal/service/transcode"
)

// TranscodePublicServer 表示 gRPC 公共服务接口。
type TranscodePublicServer interface {
	CreateJob(req model.CreateJobRequest) (model.CreateJobResponseData, error)
	QueryProgress(requestID string) (model.ProgressSnapshot, error)
}

type transcodePublicServer struct {
	service *transcodesvc.Service
}

// NewTranscodePublicServer 创建 gRPC 公共服务实现。
func NewTranscodePublicServer(service *transcodesvc.Service) TranscodePublicServer {
	return &transcodePublicServer{service: service}
}

func (s *transcodePublicServer) CreateJob(req model.CreateJobRequest) (model.CreateJobResponseData, error) {
	return s.service.CreateJob(context.Background(), req)
}

func (s *transcodePublicServer) QueryProgress(requestID string) (model.ProgressSnapshot, error) {
	return s.service.QueryProgress(context.Background(), requestID)
}
