package dto

// WorkerHeartbeatPayload 表示 gRPC 内部 DTO。
type WorkerHeartbeatPayload struct {
	NodeID   uint64
	WorkerID string
}
