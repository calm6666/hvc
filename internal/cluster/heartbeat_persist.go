package cluster

import (
	"strings"
	"sync"
	"time"
)

// HeartbeatPersistGate 控制“Redis 高频、MySQL 节流持久化”的心跳落库节奏。
//
// 说明：
// 1. Redis 仍承担秒级热路径，调度器和监控优先读 Redis；
// 2. 数据库只保留最近一次可审计心跳时间，不再承接每轮上报写入；
// 3. 这里按 node_id / worker_id 维度分别记忆最近一次成功持久化时间。
type HeartbeatPersistGate struct {
	mu           sync.RWMutex
	lastNodeAt   map[uint64]time.Time
	lastWorkerAt map[string]time.Time
}

// NewHeartbeatPersistGate 创建心跳落库门闩。
func NewHeartbeatPersistGate() *HeartbeatPersistGate {
	return &HeartbeatPersistGate{
		lastNodeAt:   make(map[uint64]time.Time),
		lastWorkerAt: make(map[string]time.Time),
	}
}

// ShouldPersistNode 判断节点心跳本轮是否需要落库。
func (g *HeartbeatPersistGate) ShouldPersistNode(now time.Time, nodeID uint64, timeout time.Duration) bool {
	if g == nil || nodeID == 0 {
		return true
	}
	g.mu.RLock()
	lastAt := g.lastNodeAt[nodeID]
	g.mu.RUnlock()
	if lastAt.IsZero() {
		return true
	}
	return now.Sub(lastAt) >= ResolveHeartbeatPersistInterval(timeout)
}

// MarkNodePersisted 记录节点心跳成功落库时间。
func (g *HeartbeatPersistGate) MarkNodePersisted(nodeID uint64, at time.Time) {
	if g == nil || nodeID == 0 {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastNodeAt[nodeID] = at
}

// ShouldPersistWorker 判断 Worker 心跳本轮是否需要落库。
func (g *HeartbeatPersistGate) ShouldPersistWorker(now time.Time, workerID string, timeout time.Duration) bool {
	if g == nil || strings.TrimSpace(workerID) == "" {
		return true
	}
	workerID = strings.TrimSpace(workerID)
	g.mu.RLock()
	lastAt := g.lastWorkerAt[workerID]
	g.mu.RUnlock()
	if lastAt.IsZero() {
		return true
	}
	return now.Sub(lastAt) >= ResolveHeartbeatPersistInterval(timeout)
}

// MarkWorkerPersisted 记录 Worker 心跳成功落库时间。
func (g *HeartbeatPersistGate) MarkWorkerPersisted(workerID string, at time.Time) {
	workerID = strings.TrimSpace(workerID)
	if g == nil || workerID == "" {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastWorkerAt[workerID] = at
}

// ResolveHeartbeatPersistInterval 计算数据库心跳持久化间隔。
//
// 目标：
// 1. 明显低于心跳超时窗口，避免离线误判；
// 2. 足够大，保证节流收益；
// 3. 在超时窗口内至少保留多次持久化机会。
func ResolveHeartbeatPersistInterval(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	interval := timeout / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	if interval > 15*time.Second {
		interval = 15 * time.Second
	}
	if interval >= timeout {
		interval = timeout / 2
	}
	if interval <= 0 {
		return 5 * time.Second
	}
	return interval
}
