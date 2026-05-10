package bootstrap

import (
	"context"
	"fmt"
	"hvc/internal/audit"
	"hvc/internal/auth"
	"hvc/internal/callback"
	"hvc/internal/cluster"
	hotpath "hvc/internal/cluster/hotpath"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/handler"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	grpcpublic "hvc/internal/interfaces/grpc/public"
	adminhttp "hvc/internal/interfaces/http/admin"
	publichttp "hvc/internal/interfaces/http/public"
	mqconsumer "hvc/internal/interfaces/mq/consumer"
	wsmonitor "hvc/internal/interfaces/ws/monitor"
	"hvc/internal/live"
	"hvc/internal/manifest"
	"hvc/internal/opslog"
	"hvc/internal/scheduler"
	"hvc/internal/server"
	"hvc/internal/service"
	livesvc "hvc/internal/service/live"
	transcodesvc "hvc/internal/service/transcode"
	authusecase "hvc/internal/usecase/auth"
	transcodeusecase "hvc/internal/usecase/transcode"
	"hvc/internal/worker"
	"hvc/pkg/logx"
	"time"
)

// Application 表示服务进程装配结果。
type Application struct {
	baseConfig    config.RuntimeConfig
	dynamicConfig *configcenter.EffectiveConfig
	httpServer    *server.HTTPServer
	internalGRPC  *server.InternalGRPCServer
	coordinator   *cluster.Coordinator
	clusterCache  *cluster.StateCache
	leaseCache    *cluster.LeaseCache
	hotpathBus    *hotpath.MemoryBus
	scheduler     *scheduler.Manager
	worker        *worker.Module
	callback      *callback.Dispatcher
	grpcServer    *server.GRPCServer
	mqConsumer    *server.MQConsumer
	configSyncer  *runtimeConfigSyncer
}

// NewApplication 创建服务实例。
func NewApplication(baseConfig config.RuntimeConfig) (*Application, error) {
	db, err := mysql.Open(baseConfig.MySQL)
	if err != nil {
		return nil, err
	}
	redisClient, err := rediscache.Open(baseConfig.Redis)
	if err != nil {
		return nil, err
	}
	runtimeConfigRepository := mysql.NewRuntimeConfigRepository(db)
	runtimeConfigCache := rediscache.NewRuntimeConfigCache(redisClient)
	bootstrapDynamicConfig := config.LoadBootstrapDynamicRuntimeConfig(baseConfig)
	if err := runtimeConfigRepository.EnsureBootstrapPublished(context.Background(), mysql.NewBootstrapRuntimeConfigRecord(bootstrapDynamicConfig, "bootstrap_local")); err != nil {
		return nil, fmt.Errorf("ensure bootstrap runtime config failed: %w", err)
	}
	if changed, err := runtimeConfigRepository.NormalizeBootstrapPublicGRPCDefault(context.Background()); err != nil {
		return nil, fmt.Errorf("normalize bootstrap public grpc default failed: %w", err)
	} else if changed {
		_ = runtimeConfigCache.InvalidatePublished(context.Background())
	}
	dynamicConfigSnapshot, ok := loadPublishedDynamicRuntimeConfig(context.Background(), runtimeConfigCache, db)
	if !ok {
		dynamicConfigSnapshot = rediscache.RuntimeConfigSnapshot{
			ConfigVersion: 0,
			Config:        bootstrapDynamicConfig,
		}
	}
	effectiveConfig := configcenter.NewEffectiveConfig(dynamicConfigSnapshot.Config)
	effectiveConfig.ReplaceWithVersion(dynamicConfigSnapshot.Config, dynamicConfigSnapshot.ConfigVersion)
	coordinator := cluster.NewCoordinator(
		dynamicConfigSnapshot.Config,
		baseConfig.Server.NodeID,
		baseConfig.Server.ServiceName,
		baseConfig.ResolveAdvertiseIP(),
		baseConfig.Server.ListenAddress,
		baseConfig.InternalGRPC.ListenAddress,
	)
	systemService := service.NewSystemService(baseConfig.Server.ServiceName, resolveModeName(dynamicConfigSnapshot.Config))
	systemHandler := handler.NewSystemHandler(systemService)
	auditRepository := audit.NewRepository(db)
	opsLogRepository := opslog.NewRepository(db)
	logx.RegisterPersistenceSink(opslog.NewSink(opsLogRepository))
	jobRepository := mysql.NewJobRepository(db)
	jobRequestOverrideRepository := mysql.NewJobRequestOverrideRepository(db)
	renditionRepository := mysql.NewTranscodeRenditionRepository(db)
	segmentRepository := mysql.NewSegmentRepository(db)
	outboxRepository := mysql.NewOutboxRepository(db)
	deliveryFailureQueueRepository := mysql.NewDeliveryFailureQueueRepository(db)
	workerInstanceRepository := mysql.NewWorkerInstanceRepository(db)
	gpuDeviceRepository := mysql.NewGPUDeviceRepository(db)
	gpuCapabilityRepository := mysql.NewWorkerCodecCapabilityRepository(db)
	jobExecutionRepository := mysql.NewJobExecutionRepository(db)
	liveChannelRepository := mysql.NewLiveChannelRepository(db)
	liveSessionRepository := mysql.NewLiveSessionRepository(db)
	liveProfileRenditionRepository := mysql.NewLiveProfileRenditionRepository(db)
	livePlaybackTokenRepository := mysql.NewLivePlaybackTokenRepository(db)
	livePublishAuthLogRepository := mysql.NewLivePublishAuthLogRepository(db)
	livePublishSessionRepository := mysql.NewLivePublishSessionRepository(db)
	liveSessionEventRepository := mysql.NewLiveSessionEventRepository(db)
	adminRepository := mysql.NewAdminRepository(db)
	adminRBACRepository := mysql.NewAdminRBACRepository(db)
	callbackConfigRepository := mysql.NewCallbackConfigRepository(db)
	configCenterBindingRepository := mysql.NewConfigCenterBindingRepository(db)
	clusterNodeRepository := mysql.NewClusterNodeRepository(db)
	_ = clusterNodeRepository.EnsureLocalNode(context.Background(), baseConfig.Server.NodeID, baseConfig.Server.ServiceName, baseConfig.ResolveAdvertiseIP(), baseConfig.Server.ListenAddress)
	auth.ConfigureAdminAuth(adminRepository, adminRBACRepository)
	adminhttp.ConfigureAdminAudit(auditRepository)
	ensureAdminRBACSeed(context.Background(), adminRepository, adminRBACRepository)
	progressStore := rediscache.NewProgressStore(redisClient)
	clusterCache := cluster.NewStateCache(redisClient)
	leaseCache := cluster.NewLeaseCache(redisClient)
	hotpathBus := hotpath.NewMemoryBus()
	createJobUseCase := transcodeusecase.NewCreateJobUseCase()
	queryProgressUseCase := transcodeusecase.NewQueryProgressUseCase(jobRepository, progressStore)
	loginUseCase := &authusecase.LoginUseCase{}
	transcodeService := transcodesvc.NewService(createJobUseCase, queryProgressUseCase, jobRepository, jobRequestOverrideRepository)
	channelService := livesvc.NewChannelService(dynamicConfigSnapshot.Config, liveChannelRepository, liveSessionRepository, liveProfileRenditionRepository)
	channelService.SetConfigSnapshot(effectiveConfig.Snapshot)
	channelService.SetPlaybackTokenWriter(livePlaybackTokenRepository)
	channelService.SetPublishAuthLogger(livePublishAuthLogRepository)
	channelService.SetPublishSessionStore(livePublishSessionRepository)
	transcodeHandler := publichttp.NewTranscodeHandler(transcodeService)
	manifestBuilder := manifest.NewBuilder(segmentRepository, jobRepository, effectiveConfig)
	manifestHandler := publichttp.NewManifestHandler(manifestBuilder)
	clusterHandler := publichttp.NewClusterHandler(clusterCache, leaseCache, jobRepository, segmentRepository, workerInstanceRepository, clusterNodeRepository)
	liveManager := live.NewManager(live.NewRepositoryChannelStore(liveChannelRepository), live.NewRepositorySessionStore(liveSessionRepository))
	liveHandler := publichttp.NewLiveHandler(liveManager, channelService)
	authHandler := adminhttp.NewAuthHandler(loginUseCase, adminRepository)
	namingTemplateRepository := mysql.NewNamingTemplateRepository(db)
	configHandler := adminhttp.NewConfigHandler(runtimeConfigRepository, runtimeConfigCache, namingTemplateRepository, effectiveConfig)
	callbackHandler := adminhttp.NewCallbackHandler(callbackConfigRepository)
	rbacHandler := adminhttp.NewRBACHandler(adminRepository, adminRBACRepository, auditRepository, opsLogRepository)
	writeRBACHandler := adminhttp.NewWriteRBAC(adminRBACRepository, adminRepository)
	configCenterHandler := adminhttp.NewConfigCenterHandler(configCenterBindingRepository)
	schedulerManager := scheduler.NewManager(dynamicConfigSnapshot.Config, effectiveConfig, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, clusterCache, jobRepository, jobRequestOverrideRepository, jobExecutionRepository)
	schedulerManager.SetLeaseCache(leaseCache)
	schedulerManager.SetClusterNodeRepository(clusterNodeRepository)
	adminClusterHandler := adminhttp.NewClusterHandler(clusterNodeRepository, gpuDeviceRepository, clusterCache, coordinator.Registry(), effectiveConfig, runtimeConfigRepository, runtimeConfigCache, jobRepository, jobExecutionRepository, workerInstanceRepository, schedulerManager, db, baseConfig.InternalGRPC, baseConfig.Server.NodeID, baseConfig.Server.ListenAddress)
	adminTranscodeHandler := adminhttp.NewTranscodeHandler(jobRepository, progressStore)
	namingTemplateHandler := adminhttp.NewNamingTemplateHandler(namingTemplateRepository, runtimeConfigRepository, runtimeConfigCache, effectiveConfig)
	liveManager.SetSessionEventWriter(liveSessionEventRepository)
	liveManager.SetPublishSessionWriter(livePublishSessionRepository)
	adminLiveHandler := adminhttp.NewLiveHandler(liveManager, channelService)
	monitorHandler := wsmonitor.NewSnapshotHandler(clusterCache, hotpathBus, progressStore, jobRepository, clusterNodeRepository, resolveModeName(dynamicConfigSnapshot.Config))
	grpcPublicServer := grpcpublic.NewTranscodePublicServer(transcodeService)
	mqCreateJobConsumer := mqconsumer.NewCreateJobConsumer(transcodeService)
	return &Application{
		baseConfig:    baseConfig,
		dynamicConfig: effectiveConfig,
		httpServer:    server.NewHTTPServer(baseConfig.Server, baseConfig.InternalGRPC.SharedToken, systemHandler, transcodeHandler, clusterHandler, liveHandler, manifestHandler, authHandler, configHandler, callbackHandler, rbacHandler, writeRBACHandler, configCenterHandler, adminClusterHandler, adminTranscodeHandler, adminLiveHandler, namingTemplateHandler, monitorHandler),
		internalGRPC:  server.NewInternalGRPCServer(baseConfig.InternalGRPC, clusterCache, progressStore, segmentRepository, jobRepository, clusterNodeRepository),
		coordinator:   coordinator,
		clusterCache:  clusterCache,
		leaseCache:    leaseCache,
		hotpathBus:    hotpathBus,
		scheduler:     schedulerManager,
		worker:        worker.NewModule(dynamicConfigSnapshot.Config, effectiveConfig, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, jobRepository, renditionRepository, segmentRepository, progressStore, outboxRepository, hotpathBus, clusterCache, workerInstanceRepository, clusterNodeRepository, gpuDeviceRepository, gpuCapabilityRepository, jobExecutionRepository),
		callback:      callback.NewDispatcher(effectiveConfig, outboxRepository, callbackConfigRepository, jobRequestOverrideRepository, deliveryFailureQueueRepository),
		grpcServer:    server.NewGRPCServer(dynamicConfigSnapshot.Config, effectiveConfig, grpcPublicServer),
		mqConsumer:    server.NewMQConsumer(dynamicConfigSnapshot.Config, effectiveConfig, mqCreateJobConsumer),
		configSyncer:  newRuntimeConfigSyncer(runtimeConfigCache, db, effectiveConfig),
	}, nil
}

func resolveModeName(cfg config.DynamicRuntimeConfig) string {
	if cfg.IsStandalone() {
		return "standalone"
	}
	if cfg.IsClusterControl() {
		return "cluster-control"
	}
	if cfg.IsClusterWorker() {
		return "cluster-worker"
	}
	if cfg.IsClusterAllInOne() {
		return "cluster-allinone"
	}
	return "custom"
}

// Run 启动服务实例。
func (a *Application) Run(ctx context.Context) error {
	errCh := make(chan error, 5)
	httpTask := &managedTask{name: "http"}

	go func() { errCh <- a.internalGRPC.Start(ctx) }()
	go func() { errCh <- a.scheduler.Start(ctx) }()
	go func() { errCh <- a.worker.Start(ctx) }()
	go func() { errCh <- a.callback.Start(ctx) }()
	go func() { errCh <- a.coordinator.Start(ctx) }()
	go func() { errCh <- a.configSyncer.Start(ctx) }()
	reconcileTicker := time.NewTicker(time.Second)
	defer reconcileTicker.Stop()
	current := a.dynamicConfig.Snapshot()
	httpTask.Reconcile(ctx, current.Mode.EnableHTTPServer, a.baseConfig.Server.ListenAddress, a.httpServer.Start)
	a.grpcServer.Reconcile(ctx)
	a.mqConsumer.Reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-reconcileTicker.C:
			cfg := a.dynamicConfig.Snapshot()
			httpTask.Reconcile(ctx, cfg.Mode.EnableHTTPServer, a.baseConfig.Server.ListenAddress, a.httpServer.Start)
			a.grpcServer.Reconcile(ctx)
			a.mqConsumer.Reconcile(ctx)
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
}

// loadPublishedDynamicRuntimeConfig 按“Redis 优先、数据库回源、再回填 Redis”的顺序加载已发布运行时配置。
//
// 这样做的目的有两个：
// 1. 启动和热重载尽量避免直接把数据库打成热点；
// 2. 即便 Redis 因 TTL 或故障丢失，也能自动回源并恢复缓存。
func loadPublishedDynamicRuntimeConfig(ctx context.Context, runtimeConfigCache *rediscache.RuntimeConfigCache, db *mysql.DB) (rediscache.RuntimeConfigSnapshot, bool) {
	if runtimeConfigCache != nil {
		if snapshot, ok, err := runtimeConfigCache.LoadPublished(ctx); err == nil && ok {
			return snapshot, true
		}
	}

	result, found := mysql.LoadDynamicRuntimeConfigFromDB(ctx, db)
	if !found {
		return rediscache.RuntimeConfigSnapshot{}, false
	}
	cfg, ok := result.Config.(config.DynamicRuntimeConfig)
	if !ok {
		return rediscache.RuntimeConfigSnapshot{}, false
	}

	snapshot := rediscache.RuntimeConfigSnapshot{
		ConfigVersion: result.Record.ConfigVersion,
		Config:        cfg,
	}

	if runtimeConfigCache != nil {
		_ = runtimeConfigCache.SavePublished(ctx, snapshot)
	}
	return snapshot, true
}
