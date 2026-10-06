// Package monitor 提供 WebSocket 实时监控推送实现。
//
// 支持两种访问方式：
//   - HTTP GET /admin/transcode/monitor/snapshot -- 一次性拉取完整快照
//   - WebSocket /admin/transcode/monitor/ws -- 实时推送增量更新
//
// WebSocket 协议：
//   - 连接建立后立即发送全量快照（type=snapshot）
//   - 每 3 秒推送周期快照（type=snapshot）
//   - 每 30 秒发送心跳（type=ping）
//   - 客户端发送 type=pong 响应心跳
//   - 连接断开后自动清理
package monitor

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"hvc/internal/cluster"
	"hvc/internal/cluster/hotpath"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"

	"github.com/gorilla/websocket"
)

// MonitorSnapshot 表示监控快照。
type MonitorSnapshot struct {
	Timestamp     int64                 `json:"timestamp"`
	Mode          string                `json:"mode"`
	PendingJobs   int                   `json:"pending_jobs"`
	ActiveJobs    int                   `json:"active_jobs"`
	NodeCount     int                   `json:"node_count"`
	SystemMetrics SystemMetricsSnapshot `json:"system_metrics"`
	Nodes         []NodeSnapshot        `json:"nodes"`
}

// SystemMetricsSnapshot 表示系统指标快照。
type SystemMetricsSnapshot struct {
	TotalActiveSessions int `json:"total_active_sessions"`
	TotalPendingJobs    int `json:"total_pending_jobs"`
}

// NodeSnapshot 表示节点快照。
type NodeSnapshot struct {
	NodeID uint64 `json:"node_id"`
	Online bool   `json:"online"`
}

const monitorNodeOnlineGrace = 2 * time.Minute
const monitorSnapshotCacheTTL = 2 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSMessage 表示 WebSocket 消息格式。
type WSMessage struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// SnapshotHandler 处理监控快照和 WebSocket 推送。
type SnapshotHandler struct {
	stateCache      *cluster.StateCache
	hotpathBus      *hotpath.MemoryBus
	progressStore   *rediscache.ProgressStore
	jobRepository   *mysql.JobRepository
	clusterNodeRepo *mysql.ClusterNodeRepository
	mode            string

	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

// NewSnapshotHandler 创建监控快照处理器。
func NewSnapshotHandler(
	stateCache *cluster.StateCache,
	hotpathBus *hotpath.MemoryBus,
	progressStore *rediscache.ProgressStore,
	jobRepository *mysql.JobRepository,
	clusterNodeRepo *mysql.ClusterNodeRepository,
	mode string,
) *SnapshotHandler {
	h := &SnapshotHandler{
		stateCache:      stateCache,
		hotpathBus:      hotpathBus,
		progressStore:   progressStore,
		jobRepository:   jobRepository,
		clusterNodeRepo: clusterNodeRepo,
		mode:            mode,
		clients:         make(map[*websocket.Conn]struct{}),
	}
	go h.broadcastLoop()
	return h
}

// Snapshot 处理 HTTP 快照请求。
func (h *SnapshotHandler) Snapshot(w http.ResponseWriter, r *http.Request) {
	if payload, ok := h.getCachedSnapshotPayload(r.Context()); ok {
		writeSnapshotResponse(w, payload)
		return
	}
	snapshot := h.buildSnapshot(r)
	payload, err := json.Marshal(model.Response{
		Code:    0,
		Message: "ok",
		Data:    snapshot,
	})
	if err != nil {
		logx.Error("monitor.snapshot.encode_failed", err, nil)
		return
	}
	if err := h.saveCachedSnapshotPayload(r.Context(), string(payload)); err != nil {
		logx.Error("monitor.snapshot.cache_save_failed", err, nil)
	}
	writeSnapshotResponse(w, string(payload))
}

// HandleWS 处理 WebSocket 连接升级。
//
// GET /admin/transcode/monitor/ws
//
// 连接建立后：
//  1. 立即发送全量快照
//  2. 注册到广播列表
//  3. 启动读取协程（处理客户端消息和断连检测）
func (h *SnapshotHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Error("monitor.ws.upgrade_failed", err, nil)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	snapshot := h.buildSnapshot(r)
	snapshotData, _ := json.Marshal(snapshot)
	initMsg := WSMessage{
		Type:      "snapshot",
		Timestamp: time.Now().UnixMilli(),
		Data:      snapshotData,
	}
	if msgBytes, err := json.Marshal(initMsg); err == nil {
		conn.WriteMessage(websocket.TextMessage, msgBytes)
	}

	go h.readPump(conn)

	logx.Info("monitor.ws.client_connected", logx.Fields{
		"remote_addr":  conn.RemoteAddr().String(),
		"client_count": len(h.clients),
	})
}

// readPump 读取客户端消息，检测断连。
func (h *SnapshotHandler) readPump(conn *websocket.Conn) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
		logx.Info("monitor.ws.client_disconnected", logx.Fields{
			"client_count": len(h.clients),
		})
	}()

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// broadcastLoop 周期性广播快照到所有 WebSocket 客户端。
func (h *SnapshotHandler) broadcastLoop() {
	snapshotTicker := time.NewTicker(3 * time.Second)
	pingTicker := time.NewTicker(30 * time.Second)
	defer snapshotTicker.Stop()
	defer pingTicker.Stop()

	for {
		select {
		case <-snapshotTicker.C:
			h.broadcastSnapshot()
		case <-pingTicker.C:
			h.broadcastPing()
		}
	}
}

// broadcastSnapshot 广播快照到所有客户端。
func (h *SnapshotHandler) broadcastSnapshot() {
	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
	}
	h.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	ctx := context.Background()
	snapshot := h.buildSnapshotFromCtx(ctx)
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		return
	}

	msg := WSMessage{
		Type:      "snapshot",
		Timestamp: time.Now().UnixMilli(),
		Data:      snapshotData,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	var disconnected []*websocket.Conn
	for _, conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			disconnected = append(disconnected, conn)
		}
	}

	if len(disconnected) > 0 {
		h.mu.Lock()
		for _, conn := range disconnected {
			delete(h.clients, conn)
			conn.Close()
		}
		h.mu.Unlock()
	}
}

// broadcastPing 广播心跳到所有客户端。
func (h *SnapshotHandler) broadcastPing() {
	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
	}
	h.mu.RUnlock()

	msg := WSMessage{
		Type:      "ping",
		Timestamp: time.Now().UnixMilli(),
	}
	msgBytes, _ := json.Marshal(msg)

	var disconnected []*websocket.Conn
	for _, conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			disconnected = append(disconnected, conn)
		}
	}

	if len(disconnected) > 0 {
		h.mu.Lock()
		for _, conn := range disconnected {
			delete(h.clients, conn)
			conn.Close()
		}
		h.mu.Unlock()
	}
}

// ClientCount 返回当前 WebSocket 客户端数量。
func (h *SnapshotHandler) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *SnapshotHandler) buildSnapshot(r *http.Request) MonitorSnapshot {
	return h.buildSnapshotFromCtx(r.Context())
}

func (h *SnapshotHandler) buildSnapshotFromCtx(ctx context.Context) MonitorSnapshot {
	snapshot := MonitorSnapshot{
		Timestamp: time.Now().UnixMilli(),
		Mode:      h.mode,
	}

	if h.jobRepository != nil {
		snapshot.PendingJobs = int(h.jobRepository.CountByStatus(ctx, model.JobStatusQueued))
	}

	if h.hotpathBus != nil {
		snapshot.ActiveJobs = h.hotpathBus.ActiveProgressCount(ctx)
	}

	snapshot.SystemMetrics = SystemMetricsSnapshot{
		TotalActiveSessions: snapshot.ActiveJobs,
		TotalPendingJobs:    snapshot.PendingJobs,
	}

	if h.clusterNodeRepo != nil {
		nodes := h.clusterNodeRepo.ListHeartbeatSnapshot(ctx)
		nodeIDs := make([]uint64, 0, len(nodes))
		for _, node := range nodes {
			nodeIDs = append(nodeIDs, node.NodeID)
		}
		metricsByNode := h.getNodeMetricsBatch(ctx, nodeIDs)
		now := time.Now()
		snapshot.NodeCount = len(nodes)
		for _, node := range nodes {
			metrics, ok := metricsByNode[node.NodeID]
			lastMetricsAt := time.Time{}
			if ok {
				lastMetricsAt = metrics.Timestamp
			}
			snapshot.Nodes = append(snapshot.Nodes, NodeSnapshot{
				NodeID: node.NodeID,
				Online: isMonitorNodeOnline(node.LastHeartbeatAt, lastMetricsAt, now),
			})
		}
	} else {
		snapshot.NodeCount = 1
		snapshot.Nodes = []NodeSnapshot{{Online: true}}
	}

	return snapshot
}

func (h *SnapshotHandler) getNodeMetricsBatch(ctx context.Context, nodeIDs []uint64) map[uint64]model.NodeMetrics {
	if h.stateCache == nil || len(nodeIDs) == 0 {
		return map[uint64]model.NodeMetrics{}
	}
	return h.stateCache.GetNodeMetricsBatch(ctx, nodeIDs)
}

func isMonitorNodeOnline(lastHeartbeatAt time.Time, lastMetricsAt time.Time, now time.Time) bool {
	if !lastMetricsAt.IsZero() && now.Sub(lastMetricsAt) <= monitorNodeOnlineGrace {
		return true
	}
	if !lastHeartbeatAt.IsZero() && now.Sub(lastHeartbeatAt) <= monitorNodeOnlineGrace {
		return true
	}
	return false
}

func (h *SnapshotHandler) getCachedSnapshotPayload(ctx context.Context) (string, bool) {
	if h == nil || h.stateCache == nil {
		return "", false
	}
	return h.stateCache.GetJSONSnapshot(ctx, h.snapshotCacheKey())
}

func (h *SnapshotHandler) saveCachedSnapshotPayload(ctx context.Context, payload string) error {
	if h == nil || h.stateCache == nil {
		return nil
	}
	return h.stateCache.SaveJSONSnapshot(ctx, h.snapshotCacheKey(), payload, monitorSnapshotCacheTTL)
}

func (h *SnapshotHandler) snapshotCacheKey() string {
	return cluster.MonitorSnapshotCacheKey(h.mode)
}

func writeSnapshotResponse(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(payload))
}
