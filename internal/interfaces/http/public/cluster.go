package public

import (
	"encoding/json"
	"net/http"
	"time"

	clusterstate "hvc/internal/cluster"
	clusterheartbeat "hvc/internal/cluster/heartbeat"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// ClusterHandler 处理集群内部上报接口。
type ClusterHandler struct {
	cache              *clusterstate.StateCache
	leaseCache         *clusterstate.LeaseCache
	jobRepository      *mysql.JobRepository
	segmentRepository  *mysql.SegmentRepository
	workerInstanceRepo *mysql.WorkerInstanceRepository
	clusterNodeRepo    *mysql.ClusterNodeRepository
}

// NewClusterHandler 创建集群处理器。
func NewClusterHandler(cache *clusterstate.StateCache, leaseCache *clusterstate.LeaseCache, jobRepository *mysql.JobRepository, segmentRepository *mysql.SegmentRepository, workerInstanceRepo *mysql.WorkerInstanceRepository, clusterNodeRepo *mysql.ClusterNodeRepository) *ClusterHandler {
	return &ClusterHandler{
		cache:              cache,
		leaseCache:         leaseCache,
		jobRepository:      jobRepository,
		segmentRepository:  segmentRepository,
		workerInstanceRepo: workerInstanceRepo,
		clusterNodeRepo:    clusterNodeRepo,
	}
}

// ReportHeartbeat 处理心跳上报。
func (h *ClusterHandler) ReportHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req model.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.Error("http.cluster.heartbeat.bind", err, nil)
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	clusterheartbeat.SaveHeartbeat(r.Context(), h.cache, model.WorkerHeartbeat{
		NodeID:             req.NodeID,
		WorkerID:           req.WorkerID,
		StartupInstanceID:  req.StartupInstanceID,
		MachineFingerprint: req.MachineFingerprint,
		Timestamp:          req.Timestamp,
	})
	if h.clusterNodeRepo != nil {
		if err := h.clusterNodeRepo.TouchHeartbeat(r.Context(), req.NodeID, req.Timestamp); err != nil {
			logx.Error("http.cluster.heartbeat.node_touch_failed", err, logx.Fields{
				"node_id":   req.NodeID,
				"worker_id": req.WorkerID,
			})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "heartbeat persist failed"})
			return
		}
	}
	if h.workerInstanceRepo != nil {
		if err := h.workerInstanceRepo.TouchHeartbeat(r.Context(), req.WorkerID); err != nil {
			logx.Error("http.cluster.heartbeat.worker_touch_failed", err, logx.Fields{
				"node_id":   req.NodeID,
				"worker_id": req.WorkerID,
			})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "heartbeat persist failed"})
			return
		}
	}
	logx.Info("http.cluster.heartbeat.accepted", logx.Fields{
		"node_id":   req.NodeID,
		"worker_id": req.WorkerID,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"accepted": true}})
}

// ReportMetrics 处理节点指标上报。
func (h *ClusterHandler) ReportMetrics(w http.ResponseWriter, r *http.Request) {
	var req model.MetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.Error("http.cluster.metrics.bind", err, nil)
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	clusterheartbeat.SaveNodeMetrics(r.Context(), h.cache, model.NodeMetrics{
		NodeID:                  req.NodeID,
		CPUUsagePercent:         req.CPUUsagePercent,
		MemoryUsagePercent:      req.MemoryUsagePercent,
		GPUMemoryUsagePercent:   req.GPUMemoryUsagePercent,
		UploadQueueDepth:        req.UploadQueueDepth,
		ActiveTranscodeSessions: req.ActiveTranscodeSessions,
		GPUCapabilities:         req.GPUCapabilities,
		Timestamp:               req.Timestamp,
	})
	logx.Info("http.cluster.metrics.accepted", logx.Fields{
		"node_id":                   req.NodeID,
		"cpu_usage_percent":         req.CPUUsagePercent,
		"memory_usage_percent":      req.MemoryUsagePercent,
		"gpu_memory_usage_percent":  req.GPUMemoryUsagePercent,
		"upload_queue_depth":        req.UploadQueueDepth,
		"active_transcode_sessions": req.ActiveTranscodeSessions,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"accepted": true}})
}

// RenewLease 处理任务续租请求。
func (h *ClusterHandler) RenewLease(w http.ResponseWriter, r *http.Request) {
	var req model.LeaseRenewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.Error("http.cluster.lease.bind", err, nil)
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.jobRepository.RenewLease(r.Context(), req.JobID, req.WorkerID, req.LeaseGeneration); err != nil {
		logx.Error("http.cluster.lease.renew_failed", err, logx.Fields{
			"job_id":           req.JobID,
			"worker_id":        req.WorkerID,
			"lease_generation": req.LeaseGeneration,
		})
		logx.WriteJSON(w, http.StatusConflict, model.Response{Code: 409, Message: "lease renew rejected"})
		return
	}
	h.leaseCache.Save(r.Context(), clusterstate.LeaseState{
		JobID:           req.JobID,
		WorkerID:        req.WorkerID,
		LeaseGeneration: req.LeaseGeneration,
		ExpireAt:        req.ExpireAt,
	})
	logx.Info("http.cluster.lease.accepted", logx.Fields{
		"job_id":           req.JobID,
		"worker_id":        req.WorkerID,
		"lease_generation": req.LeaseGeneration,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"accepted": true, "lease_generation": req.LeaseGeneration}})
}

// ReportUploadFailed 处理分片上传失败回传。
func (h *ClusterHandler) ReportUploadFailed(w http.ResponseWriter, r *http.Request) {
	var req model.SegmentUploadFailedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.Error("http.cluster.segment_failed.bind", err, nil)
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.segmentRepository.MarkUploadFailed(r.Context(), req.SegmentID, req.ErrorMessage); err != nil {
		logx.Error("http.cluster.segment_failed.persist_failed", err, logx.Fields{
			"segment_id": req.SegmentID,
		})
		logx.WriteJSON(w, http.StatusConflict, model.Response{Code: 409, Message: "segment upload failure persist failed"})
		return
	}
	logx.Info("http.cluster.segment_failed.accepted", logx.Fields{
		"segment_id":    req.SegmentID,
		"retry_count":   req.RetryCount,
		"error_message": req.ErrorMessage,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"accepted": true}})
}

// ReportUploadSucceeded 处理分片上传成功回传。
func (h *ClusterHandler) ReportUploadSucceeded(w http.ResponseWriter, r *http.Request) {
	var req model.SegmentUploadedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.Error("http.cluster.segment_uploaded.bind", err, nil)
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.segmentRepository.MarkUploaded(r.Context(), req.SegmentID, req.ObjectETag, req.ObjectSizeBytes); err != nil {
		logx.Error("http.cluster.segment_uploaded.persist_failed", err, logx.Fields{
			"segment_id": req.SegmentID,
		})
		logx.WriteJSON(w, http.StatusConflict, model.Response{Code: 409, Message: "segment upload success persist failed"})
		return
	}
	logx.Info("http.cluster.segment_uploaded.accepted", logx.Fields{
		"segment_id":        req.SegmentID,
		"object_etag":       req.ObjectETag,
		"object_size_bytes": req.ObjectSizeBytes,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"accepted": true}})
}

var _ = time.Now
