package metrics

import "hvc/internal/model"

// Snapshot 表示监控快照。
type Snapshot struct {
	Overview map[string]any
	Nodes    []model.NodeMetrics
}

// BuildSnapshot 构造监控快照。
func BuildSnapshot(metrics []model.NodeMetrics) Snapshot {
	return Snapshot{
		Overview: map[string]any{
			"node_count": len(metrics),
		},
		Nodes: metrics,
	}
}
