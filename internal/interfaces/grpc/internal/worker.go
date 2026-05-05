package internal

import "hvc/internal/model"

// WorkerInternalServer 表示 gRPC 内部服务接口。
type WorkerInternalServer interface {
	ReportHeartbeat(req model.HeartbeatRequest) error
	ReportMetrics(req model.MetricsRequest) error
	RenewLease(req model.LeaseRenewRequest) error
	ReportUploadFailed(req model.SegmentUploadFailedRequest) error
	ReportUploadSucceeded(req model.SegmentUploadedRequest) error
}
