// Package admin 提供后台管理接口的转码任务管理处理器。
//
// 实现以下接口：
//   - GET  /admin/transcode/job/list     -- 任务列表（分页+筛选）
//   - GET  /admin/transcode/job/detail   -- 任务详情
//   - POST /admin/transcode/job/retry    -- 重试失败任务
//   - POST /admin/transcode/job/cancel   -- 取消任务
//   - GET  /admin/transcode/job/progress -- 后台管理进度查询（含额外信息）
package admin

import (
	"fmt"
	"net/http"
	"strconv"

	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// TranscodeHandler 处理后台转码任务管理接口。
type TranscodeHandler struct {
	jobRepository *mysql.JobRepository
	progressStore *rediscache.ProgressStore
}

// NewTranscodeHandler 创建后台转码任务管理处理器。
func NewTranscodeHandler(jobRepository *mysql.JobRepository, progressStore *rediscache.ProgressStore) *TranscodeHandler {
	return &TranscodeHandler{
		jobRepository: jobRepository,
		progressStore: progressStore,
	}
}

// ListJobs 返回任务列表。
//
// GET /admin/transcode/job/list?page=1&page_size=20&status=0&biz_key=xxx
//
// 支持按状态和业务键筛选，分页返回。
func (h *TranscodeHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, pageSize := parsePageParams(r)
	statusFilter, _ := strconv.Atoi(r.URL.Query().Get("status"))
	bizKey := r.URL.Query().Get("biz_key")
	requestID := r.URL.Query().Get("request_id")

	if h.jobRepository == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "任务仓储未初始化"})
		return
	}

	jobs, total, err := h.jobRepository.ListPage(ctx, mysql.JobListFilter{
		Page:      page,
		PageSize:  pageSize,
		Status:    statusFilter,
		BizKey:    bizKey,
		RequestID: requestID,
	})
	if err != nil {
		logx.Error("admin.list_jobs.query_failed", err, logx.Fields{
			"page":       page,
			"page_size":  pageSize,
			"status":     statusFilter,
			"biz_key":    bizKey,
			"request_id": requestID,
		})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "任务列表查询失败"})
		return
	}

	items := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, map[string]any{
			"job_id":             job.JobID,
			"request_id":         job.RequestID,
			"biz_key":            job.BizKey,
			"source_url":         job.SourceURL,
			"status":             job.Status,
			"status_name":        statusName(job.Status),
			"progress_permille":  job.ProgressPermille,
			"stage":              job.ProgressStage,
			"assigned_node_id":   job.AssignedNodeID,
			"assigned_worker_id": job.AssignedWorkerID,
			"created_at":         job.CreatedAt,
			"updated_at":         job.UpdatedAt,
		})
	}

	writePageResponse(w, page, pageSize, total, items)
}

// JobDetail 返回任务详情。
//
// GET /admin/transcode/job/detail?job_id=123
func (h *TranscodeHandler) JobDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID, _ := strconv.ParseUint(r.URL.Query().Get("job_id"), 10, 64)
	if jobID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "job_id 参数无效"})
		return
	}

	if h.jobRepository == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "任务仓储未初始化"})
		return
	}

	job, found := h.jobRepository.GetByID(ctx, jobID)
	if !found {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "任务不存在"})
		return
	}

	detail := map[string]any{
		"job_id":                job.JobID,
		"request_id":            job.RequestID,
		"biz_key":               job.BizKey,
		"source_url":            job.SourceURL,
		"status":                job.Status,
		"status_name":           statusName(job.Status),
		"progress_permille":     job.ProgressPermille,
		"stage":                 job.ProgressStage,
		"profile_id":            job.ProfileID,
		"enable_watermark":      job.EnableWatermark,
		"segment_duration_sec":  job.SegmentDurationSec,
		"support_dash":          job.SupportDash,
		"support_hls":           job.SupportHLS,
		"selected_execution_hw": job.SelectedExecutionHWAccel,
		"assigned_node_id":      job.AssignedNodeID,
		"assigned_worker_id":    job.AssignedWorkerID,
		"lease_generation":      job.LeaseGeneration,
		"created_at":            job.CreatedAt,
		"updated_at":            job.UpdatedAt,
	}

	if h.progressStore != nil {
		snapshot, found := h.progressStore.Get(ctx, jobID)
		if found {
			detail["realtime_progress"] = map[string]any{
				"current_fps":            snapshot.CurrentFPS,
				"current_bitrate_kbps":   snapshot.CurrentBitrateKbps,
				"current_speed":          snapshot.CurrentSpeed,
				"elapsed_ms":             snapshot.ElapsedMS,
				"estimated_remaining_ms": snapshot.EstimatedRemainingMS,
			}
		}
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: detail})
}

// RetryJob 重试失败任务。
//
// POST /admin/transcode/job/retry
// Body: {"job_id": 123}
func (h *TranscodeHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		JobID uint64 `json:"job_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.JobID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "job_id 不能为空"})
		return
	}

	if h.jobRepository == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "任务仓储未初始化"})
		return
	}

	job, found := h.jobRepository.GetByID(ctx, req.JobID)
	if !found {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "任务不存在"})
		return
	}

	if job.Status != model.JobStatusFailed {
		logx.WriteJSON(w, http.StatusOK, model.Response{
			Code:    400,
			Message: fmt.Sprintf("任务状态为 %s，只有失败任务可以重试", statusName(job.Status)),
		})
		return
	}

	if err := h.jobRepository.ResetToQueued(ctx, req.JobID); err != nil {
		logx.Error("admin.retry_job.reset_failed", err, logx.Fields{"job_id": req.JobID})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "重试失败"})
		return
	}

	logx.Info("admin.retry_job.success", logx.Fields{"job_id": req.JobID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// CancelJob 取消任务。
//
// POST /admin/transcode/job/cancel
// Body: {"job_id": 123}
func (h *TranscodeHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		JobID uint64 `json:"job_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.JobID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "job_id 不能为空"})
		return
	}

	if h.jobRepository == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "任务仓储未初始化"})
		return
	}

	job, found := h.jobRepository.GetByID(ctx, req.JobID)
	if !found {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "任务不存在"})
		return
	}

	if job.Status == model.JobStatusCompleted || job.Status == model.JobStatusCanceled {
		logx.WriteJSON(w, http.StatusOK, model.Response{
			Code:    400,
			Message: fmt.Sprintf("任务状态为 %s，无法取消", statusName(job.Status)),
		})
		return
	}

	if err := h.jobRepository.MarkCanceled(ctx, req.JobID); err != nil {
		logx.Error("admin.cancel_job.mark_failed", err, logx.Fields{"job_id": req.JobID})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "取消失败"})
		return
	}

	logx.Info("admin.cancel_job.success", logx.Fields{"job_id": req.JobID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// JobProgress 后台管理进度查询。
//
// GET /admin/transcode/job/progress?job_id=123
//
// 与公共进度查询相比，额外返回 Worker 节点信息、实时 GPU 使用率、上传队列深度等。
func (h *TranscodeHandler) JobProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID, _ := strconv.ParseUint(r.URL.Query().Get("job_id"), 10, 64)
	if jobID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "job_id 参数无效"})
		return
	}

	if h.progressStore != nil {
		snapshot, found := h.progressStore.Get(ctx, jobID)
		if found {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: snapshot})
			return
		}
	}

	if h.jobRepository != nil {
		job, found := h.jobRepository.GetByID(ctx, jobID)
		if found {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
				"job_id":            job.JobID,
				"status":            job.Status,
				"stage":             job.ProgressStage,
				"progress_permille": job.ProgressPermille,
			}})
			return
		}
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "任务不存在"})
}

// statusName 返回任务状态的中文描述。
func statusName(status int) string {
	switch status {
	case model.JobStatusCreated:
		return "已创建"
	case model.JobStatusQueued:
		return "排队中"
	case model.JobStatusAssigned:
		return "已分配"
	case model.JobStatusRunning:
		return "转码中"
	case model.JobStatusUploading:
		return "上传中"
	case model.JobStatusCompleted:
		return "已完成"
	case model.JobStatusFailed:
		return "失败"
	case model.JobStatusCanceled:
		return "已取消"
	default:
		return fmt.Sprintf("未知(%d)", status)
	}
}
