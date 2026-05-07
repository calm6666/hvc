package reporter

import (
	"context"

	clusterstate "hvc/internal/cluster"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

type ProgressSink interface {
	SaveProgress(ctx context.Context, progress model.TranscodeProgress)
}

func ReportProgress(ctx context.Context, progress model.TranscodeProgress, snapshot model.ProgressSnapshot, sinks ...ProgressSink) {
	for _, sink := range sinks {
		sink.SaveProgress(ctx, progress)
	}
	logx.Info("worker.report.progress", logx.Fields{
		"job_id":            snapshot.JobID,
		"stage":             snapshot.Stage,
		"progress_permille": snapshot.ProgressPermille,
		"fps":               snapshot.CurrentFPS,
		"speed":             snapshot.CurrentSpeed,
	})
}

func ReportMetrics(ctx context.Context, cache *clusterstate.StateCache, metrics model.NodeMetrics) {
	if cache != nil {
		cache.SaveNodeMetrics(ctx, metrics)
	}
	logx.Info("worker.report.metrics", logx.Fields{
		"node_id":                   metrics.NodeID,
		"cpu_usage_percent":         metrics.CPUUsagePercent,
		"memory_usage_percent":      metrics.MemoryUsagePercent,
		"gpu_memory_usage_percent":  metrics.GPUMemoryUsagePercent,
		"upload_queue_depth":        metrics.UploadQueueDepth,
		"active_transcode_sessions": metrics.ActiveTranscodeSessions,
		"gpu_capability_count":      len(metrics.GPUCapabilities),
	})
}
