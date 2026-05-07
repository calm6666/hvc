package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	clusterv1 "hvc/api/pb/clusterv1"
	transcodev1 "hvc/api/pb/transcodev1"
	"hvc/internal/cluster"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	grpcinterceptor "hvc/internal/interfaces/grpc/interceptor"
	grpcpublic "hvc/internal/interfaces/grpc/public"
	"hvc/internal/model"
	"hvc/pkg/logx"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPCServer 管理 gRPC 服务的生命周期，支持动态启停和监听地址切换。
type GRPCServer struct {
	initialConfig   config.DynamicRuntimeConfig
	effectiveConfig *configcenter.EffectiveConfig
	publicServer    grpcpublic.TranscodePublicServer
	stateCache      *cluster.StateCache
	progressStore   *rediscache.ProgressStore
	segmentRepo     *mysql.SegmentRepository
	jobRepo         *mysql.JobRepository
	mu              sync.Mutex
	running         bool
	cancel          context.CancelFunc
	currentAddress  string
}

// NewGRPCServer 创建 gRPC 服务器。
func NewGRPCServer(initialConfig config.DynamicRuntimeConfig, effectiveConfig *configcenter.EffectiveConfig, publicServer grpcpublic.TranscodePublicServer, stateCache *cluster.StateCache, progressStore *rediscache.ProgressStore, segmentRepo *mysql.SegmentRepository, jobRepo *mysql.JobRepository) *GRPCServer {
	return &GRPCServer{
		initialConfig:   initialConfig,
		effectiveConfig: effectiveConfig,
		publicServer:    publicServer,
		stateCache:      stateCache,
		progressStore:   progressStore,
		segmentRepo:     segmentRepo,
		jobRepo:         jobRepo,
	}
}

// Reconcile 根据当前生效配置协调 gRPC 服务的启停状态。
func (s *GRPCServer) Reconcile(parent context.Context) {
	cfg := s.currentConfig()
	address := cfg.GRPC.ListenAddress
	if address == "" {
		address = ":9090"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !cfg.Mode.EnableGRPCServer {
		s.stopLocked()
		return
	}
	if s.running && s.currentAddress == address {
		return
	}
	s.stopLocked()

	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.running = true
	s.currentAddress = address
	go s.serve(ctx, cfg, address)
}

func (s *GRPCServer) serve(ctx context.Context, cfg config.DynamicRuntimeConfig, address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logx.Error("grpc.server.listen_failed", err, logx.Fields{"address": address})
		s.markStopped(address)
		return
	}

	serverOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpcinterceptor.UnaryLogInterceptor()),
	}
	if cfg.GRPC.MaxRecvMsgSizeMB > 0 {
		serverOpts = append(serverOpts, grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSizeMB*1024*1024))
	}
	if cfg.GRPC.MaxSendMsgSizeMB > 0 {
		serverOpts = append(serverOpts, grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSizeMB*1024*1024))
	}
	grpcServer := grpc.NewServer(serverOpts...)
	transcodev1.RegisterTranscodePublicServiceServer(grpcServer, &transcodePublicRPCServer{service: s.publicServer})
	clusterv1.RegisterClusterInternalServiceServer(grpcServer, &clusterInternalRPCServer{
		stateCache:    s.stateCache,
		progressStore: s.progressStore,
		segmentRepo:   s.segmentRepo,
		jobRepo:       s.jobRepo,
	})
	reflection.Register(grpcServer)

	go func() {
		<-ctx.Done()
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			grpcServer.Stop()
		}
		_ = listener.Close()
	}()

	if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
		logx.Error("grpc.server.serve_failed", err, logx.Fields{"address": address})
	}
	s.markStopped(address)
}

func (s *GRPCServer) currentConfig() config.DynamicRuntimeConfig {
	if s.effectiveConfig == nil {
		return s.initialConfig
	}
	return s.effectiveConfig.Snapshot()
}

func (s *GRPCServer) stopLocked() {
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = nil
	s.running = false
	s.currentAddress = ""
}

func (s *GRPCServer) markStopped(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentAddress == address {
		s.running = false
		s.currentAddress = ""
		s.cancel = nil
	}
}

type transcodePublicRPCServer struct {
	transcodev1.UnimplementedTranscodePublicServiceServer
	service grpcpublic.TranscodePublicServer
}

func (s *transcodePublicRPCServer) CreateJob(ctx context.Context, req *transcodev1.CreateJobRequest) (*transcodev1.CreateJobResponse, error) {
	resp, err := s.service.CreateJob(toModelCreateJobRequest(req))
	if err != nil {
		return &transcodev1.CreateJobResponse{
			Error: &transcodev1.Error{Code: 400, Message: err.Error()},
		}, nil
	}
	return &transcodev1.CreateJobResponse{
		JobId:      resp.JobID,
		RequestId:  resp.RequestID,
		Status:     int32(resp.Status),
		StatusName: resp.StatusName,
	}, nil
}

func (s *transcodePublicRPCServer) QueryProgress(ctx context.Context, req *transcodev1.QueryProgressRequest) (*transcodev1.QueryProgressResponse, error) {
	progress, err := s.service.QueryProgress(req.GetRequestId())
	if err != nil {
		return &transcodev1.QueryProgressResponse{
			Error: &transcodev1.Error{Code: 404, Message: err.Error()},
		}, nil
	}
	return &transcodev1.QueryProgressResponse{
		JobId:              progress.JobID,
		Status:             int32(progress.Status),
		Stage:              progress.Stage,
		ProgressPermille:   int32(progress.ProgressPermille),
		CurrentFps:         progress.CurrentFPS,
		CurrentBitrateKbps: progress.CurrentBitrateKbps,
		CurrentSpeed:       progress.CurrentSpeed,
	}, nil
}

type clusterInternalRPCServer struct {
	clusterv1.UnimplementedClusterInternalServiceServer
	stateCache    *cluster.StateCache
	progressStore *rediscache.ProgressStore
	segmentRepo   *mysql.SegmentRepository
	jobRepo       *mysql.JobRepository
}

func (s *clusterInternalRPCServer) WorkerHeartbeat(ctx context.Context, req *clusterv1.WorkerHeartbeatRequest) (*clusterv1.WorkerHeartbeatResponse, error) {
	if s.stateCache != nil {
		s.stateCache.SaveHeartbeat(ctx, model.WorkerHeartbeat{
			NodeID:             req.GetNodeId(),
			WorkerID:           req.GetWorkerId(),
			StartupInstanceID:  req.GetStartupInstanceId(),
			MachineFingerprint: req.GetMachineFingerprint(),
			Timestamp:          time.Now(),
		})
	}
	return &clusterv1.WorkerHeartbeatResponse{Accepted: true}, nil
}

func (s *clusterInternalRPCServer) ReportProgress(ctx context.Context, req *clusterv1.ReportProgressRequest) (*clusterv1.ReportProgressResponse, error) {
	snapshot := model.ProgressSnapshot{
		JobID:              req.GetJobId(),
		Status:             int(req.GetStatus()),
		Stage:              req.GetStage(),
		ProgressPermille:   int(req.GetProgressPermille()),
		CurrentFPS:         req.GetFps(),
		CurrentBitrateKbps: req.GetBitrateKbps(),
		CurrentSpeed:       req.GetSpeed(),
		UpdatedAt:          time.Now(),
	}
	if s.progressStore != nil {
		s.progressStore.Save(ctx, snapshot)
	}
	if s.jobRepo != nil {
		_ = s.jobRepo.UpdateProgress(ctx, snapshot.JobID, snapshot.ProgressPermille, snapshot.Stage)
	}
	return &clusterv1.ReportProgressResponse{Accepted: true}, nil
}

func (s *clusterInternalRPCServer) SegmentUploaded(ctx context.Context, req *clusterv1.SegmentUploadedRequest) (*clusterv1.SegmentUploadedResponse, error) {
	if s.segmentRepo == nil {
		return &clusterv1.SegmentUploadedResponse{Accepted: true}, nil
	}
	if err := s.segmentRepo.MarkUploaded(ctx, req.GetSegmentId(), req.GetObjectEtag(), req.GetObjectSizeBytes()); err != nil {
		return nil, err
	}
	return &clusterv1.SegmentUploadedResponse{Accepted: true}, nil
}

func toModelCreateJobRequest(req *transcodev1.CreateJobRequest) model.CreateJobRequest {
	result := model.CreateJobRequest{
		RequestID:       req.GetRequestId(),
		BizKey:          req.GetBizKey(),
		SourceURL:       req.GetSourceUrl(),
		ProfileID:       req.GetProfileId(),
		Priority:        int(req.GetPriority()),
		EnableWatermark: req.GetEnableWatermark(),
		CallbackURL:     req.GetCallbackUrl(),
	}
	if seg := req.GetSegmentOptions(); seg != nil {
		result.SegmentOptions = &model.SegmentOptions{
			SupportDash:        seg.GetSupportDash(),
			SupportHLS:         seg.GetSupportHls(),
			SegmentDurationSec: int(seg.GetSegmentDurationSec()),
			NamingTemplateID:   seg.GetNamingTemplateId(),
		}
	}
	if storage := req.GetStorageOptions(); storage != nil {
		result.StorageOptions = &model.StorageOptions{
			StorageID:     storage.GetStorageId(),
			BucketPrefix:  storage.GetBucketPrefix(),
			SegmentPrefix: storage.GetSegmentPrefix(),
		}
	}
	if schedule := req.GetScheduleOptions(); schedule != nil {
		result.ScheduleOptions = &model.ScheduleOptions{
			PreferredHWAccel:            schedule.GetPreferredHwaccel(),
			AllowSoftwareDecodeFallback: schedule.GetAllowSoftwareDecodeFallback(),
			MaxWaitSeconds:              int(schedule.GetMaxWaitSeconds()),
		}
	}
	if watermark := req.GetWatermark(); watermark != nil {
		result.Watermark = &model.Watermark{
			ImageURL:        watermark.GetImageUrl(),
			Anchor:          int(watermark.GetAnchor()),
			XRatio:          watermark.GetXRatio(),
			YRatio:          watermark.GetYRatio(),
			WidthRatio:      watermark.GetWidthRatio(),
			Opacity:         watermark.GetOpacity(),
			SafeMarginRatio: watermark.GetSafeMarginRatio(),
		}
	}
	if video := req.GetVideoOptions(); video != nil {
		result.VideoOptions = &model.VideoOptions{
			OutputAspectKeep: video.GetOutputAspectKeep(),
			AspectFillMode:   video.GetAspectFillMode(),
		}
	}
	if thumb := req.GetThumbnailOptions(); thumb != nil {
		result.ThumbnailOptions = &model.ThumbnailOptions{
			EnableSprite:        thumb.GetEnableSprite(),
			SpriteRows:          int(thumb.GetSpriteRows()),
			SpriteCols:          int(thumb.GetSpriteCols()),
			ThumbIntervalSec:    int(thumb.GetThumbIntervalSec()),
			ThumbWidth:          int(thumb.GetThumbWidth()),
			ThumbHeight:         int(thumb.GetThumbHeight()),
			SpriteImageFormat:   thumb.GetSpriteImageFormat(),
			SpriteStoragePrefix: thumb.GetSpriteStoragePrefix(),
			EnableBinaryIndex:   thumb.GetEnableBinaryIndex(),
			BinaryStoragePrefix: thumb.GetBinaryStoragePrefix(),
			BinaryMaxSizeBytes:  thumb.GetBinaryMaxSizeBytes(),
		}
	}
	if len(req.GetRenditions()) > 0 {
		result.Renditions = make([]model.RenditionOption, 0, len(req.GetRenditions()))
		for _, rendition := range req.GetRenditions() {
			result.Renditions = append(result.Renditions, model.RenditionOption{
				Name:             rendition.GetName(),
				Width:            int(rendition.GetWidth()),
				Height:           int(rendition.GetHeight()),
				VideoCodec:       rendition.GetVideoCodec(),
				VideoBitrateKbps: int(rendition.GetVideoBitrateKbps()),
				VideoMaxrateKbps: int(rendition.GetVideoMaxrateKbps()),
				VideoBufsizeKbps: int(rendition.GetVideoBufsizeKbps()),
				Preset:           rendition.GetPreset(),
			})
		}
	}
	return result
}
