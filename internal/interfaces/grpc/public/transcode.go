// Package public 提供 gRPC 公共服务接口定义及默认实现。
//
// 公共服务面向外部业务系统，暴露转码任务创建和进度查询接口。
// 这些接口同时支持 HTTP 和 gRPC 两种访问方式。
//
// 集群模式：请求可被负载均衡到任意 API 节点处理
// 单机模式：请求直接在本地处理
package public

import (
	"context"
	"fmt"
	"time"

	"hvc/internal/infra/db/mysql"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// TranscodePublicServer 表示 gRPC 公共服务接口。
type TranscodePublicServer interface {
	CreateJob(req model.CreateJobRequest) (model.CreateJobResponseData, error)
	QueryProgress(requestID string) (model.ProgressSnapshot, error)
}

// transcodePublicServer 是 TranscodePublicServer 的默认实现。
type transcodePublicServer struct {
	jobRepository *mysql.JobRepository
	progressStore *rediscache.ProgressStore
}

// NewTranscodePublicServer 创建 gRPC 公共服务实现。
func NewTranscodePublicServer(
	jobRepository *mysql.JobRepository,
	progressStore *rediscache.ProgressStore,
) TranscodePublicServer {
	return &transcodePublicServer{
		jobRepository: jobRepository,
		progressStore: progressStore,
	}
}

// CreateJob 通过 gRPC 创建转码任务。
//
// 与 HTTP 接口 /api/transcode/job/create 功能一致，
// 但使用 gRPC 协议传输，适用于内部微服务间调用。
func (s *transcodePublicServer) CreateJob(req model.CreateJobRequest) (model.CreateJobResponseData, error) {
	if req.SourceURL == "" {
		return model.CreateJobResponseData{}, fmt.Errorf("source_url 不能为空")
	}
	if req.RequestID == "" {
		return model.CreateJobResponseData{}, fmt.Errorf("request_id 不能为空")
	}

	ctx := context.Background()

	segmentDuration := 6
	if req.SegmentOptions != nil && req.SegmentOptions.SegmentDurationSec > 0 {
		segmentDuration = req.SegmentOptions.SegmentDurationSec
	}
	supportDash := true
	if req.SegmentOptions != nil {
		supportDash = req.SegmentOptions.SupportDash
	}
	supportHLS := true
	if req.SegmentOptions != nil {
		supportHLS = req.SegmentOptions.SupportHLS
	}

	job := model.TranscodeJob{
		RequestID:          req.RequestID,
		BizKey:             req.BizKey,
		SourceURL:          req.SourceURL,
		ProfileID:          req.ProfileID,
		Priority:           req.Priority,
		EnableWatermark:    req.EnableWatermark,
		SegmentDurationSec: segmentDuration,
		SupportDash:        supportDash,
		SupportHLS:         supportHLS,
		Status:             model.JobStatusCreated,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if s.jobRepository != nil {
		if err := s.jobRepository.Save(ctx, job); err != nil {
			logx.Error("grpc.public.create_job_failed", err, logx.Fields{
				"request_id": req.RequestID,
				"source_url": req.SourceURL,
			})
			return model.CreateJobResponseData{}, fmt.Errorf("创建任务失败: %w", err)
		}
	}

	logx.Info("grpc.public.create_job_success", logx.Fields{
		"request_id": req.RequestID,
		"source_url": req.SourceURL,
	})

	return model.CreateJobResponseData{
		RequestID:  req.RequestID,
		Status:     job.Status,
		StatusName: "CREATED",
	}, nil
}

// QueryProgress 通过 gRPC 查询任务进度。
//
// 优先从 Redis 缓存读取实时进度（低延迟），
// 缓存未命中时回退到数据库查询（高延迟但权威）。
func (s *transcodePublicServer) QueryProgress(requestID string) (model.ProgressSnapshot, error) {
	ctx := context.Background()

	if requestID == "" {
		return model.ProgressSnapshot{}, fmt.Errorf("request_id 不能为空")
	}

	if s.jobRepository != nil && s.progressStore != nil {
		job, found := s.jobRepository.FindByRequestID(ctx, requestID)
		if !found {
			return model.ProgressSnapshot{}, fmt.Errorf("任务不存在: %s", requestID)
		}
		snapshot, cacheHit := s.progressStore.Get(ctx, job.JobID)
		if cacheHit {
			return snapshot, nil
		}
		return model.ProgressSnapshot{
			JobID:            job.JobID,
			Status:           job.Status,
			Stage:            job.ProgressStage,
			ProgressPermille: job.ProgressPermille,
		}, nil
	}

	return model.ProgressSnapshot{
		Stage:            model.StageQueued,
		ProgressPermille: 0,
	}, nil
}
