// Package cluster 提供集群节点领域对象。
//
// Node 表示集群中的一个计算节点，包含节点身份、健康状态和资源能力。
// 领域对象不包含持久化逻辑，仅封装业务规则和状态判断。
//
// 支持集群模式和单机模式：
//   - 集群模式：节点通过心跳上报状态，调度器根据状态选择节点
//   - 单机模式：只有一个节点，始终为启用和健康状态
package cluster

import "time"

// Node 表示集群节点领域对象。
type Node struct {
	NodeID      uint64
	WorkerID    string
	Host        string
	Port        int
	Enabled     bool
	Healthy     bool
	Quarantined bool
	LastHeartbeat time.Time
	GPUCount    int
	MaxSessions int
}

// IsAvailable 判断节点是否可用于调度。
//
// 节点可用的条件：
//  1. 已启用（Enabled = true）
//  2. 未被隔离（Quarantined = false）
//  3. 心跳新鲜（最近心跳时间在超时阈值内）
func (n *Node) IsAvailable(heartbeatTimeout time.Duration) bool {
	if !n.Enabled {
		return false
	}
	if n.Quarantined {
		return false
	}
	if n.LastHeartbeat.IsZero() {
		return false
	}
	return time.Since(n.LastHeartbeat) < heartbeatTimeout
}

// SessionUtilization 计算节点会话利用率（0.0 ~ 1.0）。
func (n *Node) SessionUtilization(activeSessions int) float64 {
	if n.MaxSessions <= 0 {
		return 0
	}
	util := float64(activeSessions) / float64(n.MaxSessions)
	if util > 1.0 {
		return 1.0
	}
	return util
}

// RemainingSessions 返回节点剩余可用会话数。
func (n *Node) RemainingSessions(activeSessions int) int {
	remaining := n.MaxSessions - activeSessions
	if remaining < 0 {
		return 0
	}
	return remaining
}
