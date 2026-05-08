package reporter

import (
	"context"
	"sync"
	"time"

	clusterstate "hvc/internal/cluster"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

var (
	metricsLogMu     sync.Mutex
	lastMetricsLogAt time.Time
)

// ProgressSink 定义进度持久化接口。
type ProgressSink interface {
	SaveProgress(ctx context.Context, progress model.TranscodeProgress)
}

// ReportProgress 将转码进度写入所有 Sink 并记录日志。
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

// ReportMetrics 将节点指标写入集群状态缓存并记录日志。
func ReportMetrics(ctx context.Context, cache *clusterstate.StateCache, metrics model.NodeMetrics) {
	if cache != nil {
		cache.SaveNodeMetrics(ctx, metrics)
	}
	if !shouldLogMetrics(metrics) {
		return
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

func shouldLogMetrics(metrics model.NodeMetrics) bool {
	if metrics.ActiveTranscodeSessions > 0 || metrics.UploadQueueDepth > 0 {
		return true
	}
	metricsLogMu.Lock()
	defer metricsLogMu.Unlock()
	if time.Since(lastMetricsLogAt) < time.Minute {
		return false
	}
	lastMetricsLogAt = time.Now()
	return true
}
