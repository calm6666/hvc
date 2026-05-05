package internal

import "hvc/internal/model"

// TranscodePublicServer 表示 gRPC 公共服务接口。
type TranscodePublicServer interface {
	CreateJob(req model.CreateJobRequest) (model.CreateJobResponseData, error)
	QueryProgress(requestID string) (model.ProgressSnapshot, error)
}
