package public

import (
	"encoding/json"
	"errors"
	"net/http"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	transcodeusecase "hvc/internal/usecase/transcode"
	"hvc/pkg/logx"
)

var errJobNotFound = errors.New("job not found")

// TranscodeHandler 处理转码公共接口。
type TranscodeHandler struct {
	createJobUseCase          *transcodeusecase.CreateJobUseCase
	queryProgressUseCase      *transcodeusecase.QueryProgressUseCase
	jobRepository             *mysql.JobRepository
	jobRequestOverrideRepo    *mysql.JobRequestOverrideRepository
}

// NewTranscodeHandler 创建转码处理器。
func NewTranscodeHandler(createJobUseCase *transcodeusecase.CreateJobUseCase, queryProgressUseCase *transcodeusecase.QueryProgressUseCase, jobRepository *mysql.JobRepository, jobRequestOverrideRepo *mysql.JobRequestOverrideRepository) *TranscodeHandler {
	return &TranscodeHandler{
		createJobUseCase:       createJobUseCase,
		queryProgressUseCase:   queryProgressUseCase,
		jobRepository:          jobRepository,
		jobRequestOverrideRepo: jobRequestOverrideRepo,
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
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}

	logx.Info("http.transcode.create.request", logx.Fields{
		"request_id":       req.RequestID,
		"source_url":       req.SourceURL,
		"profile_id":       req.ProfileID,
		"priority":         req.Priority,
		"enable_watermark": req.EnableWatermark,
	})

	if existing, ok := h.jobRepository.FindByRequestID(r.Context(), req.RequestID); ok {
		logx.Info("http.transcode.create.idempotent_hit", logx.Fields{
			"request_id": req.RequestID,
			"job_id":     existing.JobID,
		})
		logx.WriteJSON(w, http.StatusOK, model.Response{
			Code:    0,
			Message: "ok",
			Data: model.CreateJobResponseData{
				JobID:      existing.JobID,
				RequestID:  existing.RequestID,
				Status:     existing.Status,
				StatusName: existing.ProgressStage,
			},
		})
		return
	}

	result, err := h.createJobUseCase.Execute(req)
	if err != nil {
		logx.Error("http.transcode.create.execute", err, logx.Fields{
			"request_id": req.RequestID,
		})
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	if err := h.jobRepository.Save(r.Context(), result.Job); err != nil {
		logx.Error("http.transcode.create.save", err, logx.Fields{
			"request_id": req.RequestID,
			"job_id":     result.Job.JobID,
		})
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save job failed"})
		return
	}
	if result.RequestOverride != nil && h.jobRequestOverrideRepo != nil {
		if err := h.jobRequestOverrideRepo.Save(r.Context(), *result.RequestOverride); err != nil {
			logx.Error("http.transcode.create.save_override", err, logx.Fields{
				"request_id": req.RequestID,
				"job_id":     result.Job.JobID,
			})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save job override failed"})
			return
		}
	}

	logx.Info("http.transcode.create.success", logx.Fields{
		"request_id": req.RequestID,
		"job_id":     result.Job.JobID,
		"status":     result.Job.Status,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data: model.CreateJobResponseData{
			JobID:      result.Job.JobID,
			RequestID:  result.Job.RequestID,
			Status:     result.Job.Status,
			StatusName: result.Job.ProgressStage,
		},
	})
}

// QueryProgress 处理任务进度查询请求。
func (h *TranscodeHandler) QueryProgress(w http.ResponseWriter, r *http.Request) {
	requestID := r.URL.Query().Get("request_id")
	logx.Info("http.transcode.progress.request", logx.Fields{
		"request_id": requestID,
	})
	progress, err := h.queryProgressUseCase.Execute(r.Context(), requestID)
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
