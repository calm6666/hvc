package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	transcodev1 "hvc/api/pb/transcodev1"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	grpcinterceptor "hvc/internal/interfaces/grpc/interceptor"
	grpcpublic "hvc/internal/interfaces/grpc/public"
	"hvc/internal/model"
	"hvc/pkg/logx"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPCServer 管理对外 public gRPC 服务的生命周期。
//
// 这一层只承载面向外部调用方发布的业务 RPC，支持运行期热启停和监听地址切换。
// 集群内部通信使用独立的 InternalGRPCServer，避免热更新把内部链路一并打断。
type GRPCServer struct {
	initialConfig     config.DynamicRuntimeConfig
	effectiveConfig   *configcenter.EffectiveConfig
	publicServer      grpcpublic.TranscodePublicServer
	registryRepo      *mysql.RegistryEtcdConfigRepository
	runtimeConfigRepo *mysql.RuntimeConfigRepository
	advertiseHost     string
	mu                sync.Mutex
	running           bool
	cancel            context.CancelFunc
	currentAddress    string
	currentRegistryID uint64
}

// NewGRPCServer 创建对外 public gRPC 服务管理器。
func NewGRPCServer(initialConfig config.DynamicRuntimeConfig, effectiveConfig *configcenter.EffectiveConfig, publicServer grpcpublic.TranscodePublicServer, registryRepo *mysql.RegistryEtcdConfigRepository, runtimeConfigRepo *mysql.RuntimeConfigRepository, advertiseHost string) *GRPCServer {
	return &GRPCServer{
		initialConfig:     initialConfig,
		effectiveConfig:   effectiveConfig,
		publicServer:      publicServer,
		registryRepo:      registryRepo,
		runtimeConfigRepo: runtimeConfigRepo,
		advertiseHost:     strings.TrimSpace(advertiseHost),
	}
}

// Reconcile 根据当前生效配置协调 public gRPC 的启停状态。
func (s *GRPCServer) Reconcile(parent context.Context) {
	cfg := s.currentConfig()
	address := cfg.GRPC.ListenAddress
	if address == "" {
		address = ":9090"
	}
	registryID := s.resolvePublishedRegistryID(parent)

	s.mu.Lock()
	defer s.mu.Unlock()

	if !cfg.Mode.EnableGRPCServer {
		s.stopLocked()
		return
	}
	if s.running && s.currentAddress == address && s.currentRegistryID == registryID {
		return
	}
	s.stopLocked()

	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.running = true
	s.currentAddress = address
	s.currentRegistryID = registryID
	go s.serve(ctx, cfg, address)
}

func (s *GRPCServer) serve(ctx context.Context, cfg config.DynamicRuntimeConfig, address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logx.Error("grpc.public.listen_failed", err, logx.Fields{"address": address})
		s.markStopped(address)
		return
	}
	logx.Info("grpc.public.listening", logx.Fields{
		"address": address,
	})
	deregister := s.registerService(ctx, cfg, address)
	defer func() {
		if deregister != nil {
			deregister()
		}
	}()

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
		logx.Error("grpc.public.serve_failed", err, logx.Fields{"address": address})
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
	s.currentRegistryID = 0
}

func (s *GRPCServer) markStopped(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentAddress == address {
		s.running = false
		s.currentAddress = ""
		s.currentRegistryID = 0
		s.cancel = nil
	}
}

func (s *GRPCServer) registerService(ctx context.Context, cfg config.DynamicRuntimeConfig, address string) func() {
	if s.registryRepo == nil {
		return nil
	}
	record, ok := s.resolvePublishedRegistryConfig(ctx)
	if !ok {
		return nil
	}
	if !record.Enabled {
		return nil
	}
	endpoints := splitRegistryEndpoints(record.Endpoints)
	if len(endpoints) == 0 {
		return nil
	}
	endpoint, host, port, err := s.resolvePublishedEndpoint(address)
	if err != nil {
		logx.Error("grpc.public.registry.address_invalid", err, logx.Fields{"address": address})
		return nil
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: resolveRegistryDialTimeout(record),
	})
	if err != nil {
		logx.Error("grpc.public.registry.connect_failed", err, logx.Fields{"registry_id": record.RegistryID})
		return nil
	}
	leaseResp, err := client.Grant(ctx, int64(resolveRegistryLeaseTTL(record).Seconds()))
	if err != nil {
		_ = client.Close()
		logx.Error("grpc.public.registry.lease_failed", err, logx.Fields{"registry_id": record.RegistryID})
		return nil
	}
	key := buildPublicGRPCRegistryKey(record.ServiceNamespace, "transcode.v1.TranscodePublicService", endpoint)
	payload, _ := json.Marshal(map[string]any{
		"endpoint":   endpoint,
		"host":       host,
		"port":       port,
		"service":    "transcode.v1.TranscodePublicService",
		"updated_at": time.Now(),
	})
	if _, err := client.Put(ctx, key, string(payload), clientv3.WithLease(leaseResp.ID)); err != nil {
		_ = client.Close()
		logx.Error("grpc.public.registry.put_failed", err, logx.Fields{"registry_id": record.RegistryID, "key": key})
		return nil
	}
	keepAliveCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ch, err := client.KeepAlive(keepAliveCtx, leaseResp.ID)
		if err != nil {
			logx.Error("grpc.public.registry.keepalive_failed", err, logx.Fields{"registry_id": record.RegistryID, "key": key})
			return
		}
		for range ch {
		}
	}()
	logx.Info("grpc.public.registry.registered", logx.Fields{
		"registry_id": record.RegistryID,
		"key":         key,
		"endpoint":    endpoint,
	})
	return func() {
		cancel()
		_, _ = client.Revoke(context.Background(), leaseResp.ID)
		_ = client.Close()
	}
}

func (s *GRPCServer) resolvePublishedRegistryConfig(ctx context.Context) (mysql.RegistryEtcdConfigRecord, bool) {
	if s.registryRepo == nil {
		return mysql.RegistryEtcdConfigRecord{}, false
	}
	if published, ok := s.publishedRuntimeConfig(ctx); ok && published.PublicGRPCRegistryID > 0 {
		return s.registryRepo.FindByID(ctx, published.PublicGRPCRegistryID)
	}
	return mysql.RegistryEtcdConfigRecord{}, false
}

func (s *GRPCServer) resolvePublishedRegistryID(ctx context.Context) uint64 {
	if published, ok := s.publishedRuntimeConfig(ctx); ok {
		return published.PublicGRPCRegistryID
	}
	return 0
}

func (s *GRPCServer) publishedRuntimeConfig(ctx context.Context) (mysql.RuntimeConfigRecord, bool) {
	if s.runtimeConfigRepo == nil {
		return mysql.RuntimeConfigRecord{}, false
	}
	return s.runtimeConfigRepo.LatestPublished(ctx)
}

// resolvePublishedEndpoint 生成真正写入 etcd 的服务接入点。
//
// 监听地址允许使用 0.0.0.0 或空 host，但注册中心必须写可被其它实例访问的地址。
// 因此这里优先使用显式监听 host；若监听的是 wildcard，则回退到 bootstrap 注入的 advertise IP。
func (s *GRPCServer) resolvePublishedEndpoint(address string) (string, string, int, error) {
	host, port, err := splitServiceAddress(address)
	if err != nil {
		return "", "", 0, err
	}
	publishedHost := strings.TrimSpace(host)
	if publishedHost == "" || publishedHost == "0.0.0.0" || publishedHost == "::" {
		publishedHost = strings.TrimSpace(s.advertiseHost)
	}
	if publishedHost == "" {
		publishedHost = host
	}
	return net.JoinHostPort(publishedHost, strconv.Itoa(port)), publishedHost, port, nil
}

func splitRegistryEndpoints(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func splitServiceAddress(address string) (string, int, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	if host == "" {
		host = "0.0.0.0"
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func resolveRegistryDialTimeout(record mysql.RegistryEtcdConfigRecord) time.Duration {
	if record.DialTimeoutMS > 0 {
		return time.Duration(record.DialTimeoutMS) * time.Millisecond
	}
	return 3 * time.Second
}

func resolveRegistryLeaseTTL(record mysql.RegistryEtcdConfigRecord) time.Duration {
	if record.LeaseTTLSec > 0 {
		return time.Duration(record.LeaseTTLSec) * time.Second
	}
	return 30 * time.Second
}

func buildPublicGRPCRegistryKey(namespace, serviceName, endpoint string) string {
	namespace = strings.Trim(strings.TrimSpace(namespace), "/")
	serviceName = strings.Trim(strings.TrimSpace(serviceName), "/")
	endpoint = strings.Trim(strings.TrimSpace(endpoint), "/")
	if namespace == "" {
		return fmt.Sprintf("/%s/%s", serviceName, endpoint)
	}
	return fmt.Sprintf("/%s/%s/%s", namespace, serviceName, endpoint)
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
