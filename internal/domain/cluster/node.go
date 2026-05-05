package cluster

// Node 表示集群节点领域对象。
type Node struct {
	NodeID   uint64
	Enabled  bool
	Healthy  bool
}
