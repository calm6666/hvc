package public

import (
	"encoding/json"
	"errors"
	"net/http"

	"hvc/internal/model"
	transcodesvc "hvc/internal/service/transcode"
	"hvc/pkg/logx"
)

var errJobNotFound = errors.New("任务不存在")

// TranscodeHandler 处理转码公共接口。
type TranscodeHandler struct {
	service *transcodesvc.Service
}

// NewTranscodeHandler 创建转码处理器。
func NewTranscodeHandler(service *transcodesvc.Service) *TranscodeHandler {
	return &TranscodeHandler{
		service: service,
	}
}

// CreateJob 处理创建任务请求。
func (h *TranscodeHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req model.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.Error("http.transcode.create.bind", err, logx.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		})
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求体格式无效"})
		return
	}

	logx.Info("http.transcode.create.request", logx.Fields{
		"request_id":       req.RequestID,
		"source_url":       req.SourceURL,
		"profile_id":       req.ProfileID,
		"priority":         req.Priority,
		"enable_watermark": req.EnableWatermark,
	})

	result, err := h.service.CreateJob(r.Context(), req)
	if err != nil {
		logx.Error("http.transcode.create.execute", err, logx.Fields{
			"request_id": req.RequestID,
		})
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	logx.Info("http.transcode.create.success", logx.Fields{
		"request_id": req.RequestID,
		"job_id":     result.JobID,
		"status":     result.Status,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    result,
	})
}

// QueryProgress 处理任务进度查询请求。
func (h *TranscodeHandler) QueryProgress(w http.ResponseWriter, r *http.Request) {
	requestID := r.URL.Query().Get("request_id")
	logx.Info("http.transcode.progress.request", logx.Fields{
		"request_id": requestID,
	})
	progress, err := h.service.QueryProgress(r.Context(), requestID)
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, errJobNotFound) {
			statusCode = http.StatusNotFound
		}
		logx.Error("http.transcode.progress.failed", err, logx.Fields{
			"request_id": requestID,
		})
		logx.WriteJSON(w, statusCode, model.Response{Code: statusCode, Message: err.Error()})
		return
	}
	logx.Info("http.transcode.progress.success", logx.Fields{
		"request_id":        requestID,
		"job_id":            progress.JobID,
		"progress_permille": progress.ProgressPermille,
		"stage":             progress.Stage,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: progress})
}
