package reporter

import (
	"context"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// ReportProgress 上报进度。
func ReportProgress(ctx context.Context, progress model.TranscodeProgress) {
	_ = ctx
	logx.Info("worker.report.progress", logx.Fields{
		"job_id": progress.JobID,
		"stage":  progress.Stage,
	})
}
