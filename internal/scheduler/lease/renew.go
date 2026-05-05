package lease

import "hvc/internal/model"

// BuildRenewRequest 构造续租请求。
func BuildRenewRequest(job model.TranscodeJob, workerID string) model.LeaseRenewRequest {
	return model.LeaseRenewRequest{
		JobID:           job.JobID,
		WorkerID:        workerID,
		LeaseGeneration: job.LeaseGeneration,
	}
}
