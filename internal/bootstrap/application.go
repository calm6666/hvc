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
	dynamicConfigSnapshot, ok := loadPublishedDynamicRuntimeConfig(context.Background(), runtimeConfigCache, db, baseConfig.EffectiveNodeMode())
	if !ok {
		dynamicConfigSnapshot = rediscache.RuntimeConfigSnapshot{
			ConfigVersion: 0,
			Config:        bootstrapDynamicConfig,
		}
	}
	effectiveConfig := configcenter.NewEffectiveConfig(dynamicConfigSnapshot.Config)
	effectiveConfig.ReplaceWithVersion(dynamicConfigSnapshot.Config, dynamicConfigSnapshot.ConfigVersion)
	clusterNodeRepository := mysql.NewClusterNodeRepository(db)
	_ = clusterNodeRepository.EnsureLocalNode(
		context.Background(),
		baseConfig.Server.NodeID,
		baseConfig.Server.ServiceName,
		baseConfig.ResolveAdvertiseIP(),
		baseConfig.InternalGRPC.ListenAddress,
		baseConfig.Server.ListenAddress,
		"mode:"+baseConfig.EffectiveNodeMode(),
	)
	coordinator := cluster.NewCoordinator(
		baseConfig.EffectiveNodeMode(),
		baseConfig.Server.NodeID,
		baseConfig.Server.ServiceName,
		baseConfig.ResolveAdvertiseIP(),
		baseConfig.Server.ListenAddress,
		baseConfig.InternalGRPC.ListenAddress,
		clusterNodeRepository,
		redisClient,
	)
	systemService := service.NewSystemService(baseConfig.Server.ServiceName, baseConfig.EffectiveNodeMode())
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
	registryEtcdConfigRepository := mysql.NewRegistryEtcdConfigRepository(db)
	configCenterBindingRepository := mysql.NewConfigCenterBindingRepository(db)
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
	clusterHandler := publichttp.NewClusterHandler(clusterCache, leaseCache, jobRepository, segmentRepository, workerInstanceRepository, clusterNodeRepository, effectiveConfig)
	liveManager := live.NewManager(live.NewRepositoryChannelStore(liveChannelRepository), live.NewRepositorySessionStore(liveSessionRepository))
	liveHandler := publichttp.NewLiveHandler(liveManager, channelService)
	authHandler := adminhttp.NewAuthHandler(loginUseCase, adminRepository)
	namingTemplateRepository := mysql.NewNamingTemplateRepository(db)
	configHandler := adminhttp.NewConfigHandler(runtimeConfigRepository, runtimeConfigCache, namingTemplateRepository, effectiveConfig, clusterCache, baseConfig.Server.NodeID, baseConfig.EffectiveNodeMode())
	callbackHandler := adminhttp.NewCallbackHandler(callbackConfigRepository)
	registryEtcdHandler := adminhttp.NewRegistryEtcdHandler(registryEtcdConfigRepository)
	rbacHandler := adminhttp.NewRBACHandler(adminRepository, adminRBACRepository, auditRepository, opsLogRepository)
	writeRBACHandler := adminhttp.NewWriteRBAC(adminRBACRepository, adminRepository)
	configCenterHandler := adminhttp.NewConfigCenterHandler(configCenterBindingRepository)
	schedulerManager := scheduler.NewManager(dynamicConfigSnapshot.Config, effectiveConfig, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, clusterCache, jobRepository, jobRequestOverrideRepository, jobExecutionRepository)
	schedulerManager.SetLeaseCache(leaseCache)
	schedulerManager.SetClusterNodeRepository(clusterNodeRepository)
	workerModule := worker.NewModule(dynamicConfigSnapshot.Config, effectiveConfig, baseConfig.EffectiveNodeMode(), baseConfig.Server.NodeID, baseConfig.Server.WorkerID, jobRepository, renditionRepository, segmentRepository, progressStore, outboxRepository, hotpathBus, clusterCache, workerInstanceRepository, clusterNodeRepository, gpuDeviceRepository, gpuCapabilityRepository, jobExecutionRepository)
	adminClusterHandler := adminhttp.NewClusterHandler(clusterNodeRepository, gpuDeviceRepository, clusterCache, coordinator.Registry(), effectiveConfig, runtimeConfigRepository, runtimeConfigCache, registryEtcdConfigRepository, jobRepository, jobExecutionRepository, workerInstanceRepository, schedulerManager, workerModule, auditRepository, db, baseConfig.InternalGRPC, baseConfig.EffectiveNodeMode(), baseConfig.Server.NodeID, baseConfig.Server.ListenAddress, baseConfig.ResolveAdvertiseIP())
	adminTranscodeHandler := adminhttp.NewTranscodeHandler(jobRepository, progressStore)
	namingTemplateHandler := adminhttp.NewNamingTemplateHandler(namingTemplateRepository, runtimeConfigRepository, runtimeConfigCache, effectiveConfig)
	liveManager.SetSessionEventWriter(liveSessionEventRepository)
	liveManager.SetPublishSessionWriter(livePublishSessionRepository)
	adminLiveHandler := adminhttp.NewLiveHandler(liveManager, channelService)
	adminLiveHandler.ConfigureInternalControl(clusterNodeRepository, baseConfig.InternalGRPC.SharedToken, baseConfig.Server.NodeID)
	monitorHandler := wsmonitor.NewSnapshotHandler(clusterCache, hotpathBus, progressStore, jobRepository, clusterNodeRepository, baseConfig.EffectiveNodeMode())
	grpcPublicServer := grpcpublic.NewTranscodePublicServer(transcodeService)
	mqCreateJobConsumer := mqconsumer.NewCreateJobConsumer(transcodeService)
	return &Application{
		baseConfig:    baseConfig,
		dynamicConfig: effectiveConfig,
		httpServer:    server.NewHTTPServer(baseConfig.Server, baseConfig.InternalGRPC.SharedToken, systemHandler, transcodeHandler, clusterHandler, liveHandler, manifestHandler, authHandler, configHandler, callbackHandler, registryEtcdHandler, rbacHandler, writeRBACHandler, configCenterHandler, adminClusterHandler, adminTranscodeHandler, adminLiveHandler, namingTemplateHandler, monitorHandler),
		internalGRPC:  server.NewInternalGRPCServer(baseConfig.InternalGRPC, clusterCache, progressStore, segmentRepository, jobRepository, clusterNodeRepository, workerInstanceRepository, workerModule, effectiveConfig),
		coordinator:   coordinator,
		clusterCache:  clusterCache,
		leaseCache:    leaseCache,
		hotpathBus:    hotpathBus,
		scheduler:     schedulerManager,
		worker:        workerModule,
		callback:      callback.NewDispatcher(effectiveConfig, outboxRepository, callbackConfigRepository, registryEtcdConfigRepository, jobRequestOverrideRepository, deliveryFailureQueueRepository, jobRepository),
		grpcServer:    server.NewGRPCServer(dynamicConfigSnapshot.Config, effectiveConfig, grpcPublicServer, registryEtcdConfigRepository, runtimeConfigRepository, baseConfig.ResolveAdvertiseIP()),
		mqConsumer:    server.NewMQConsumer(dynamicConfigSnapshot.Config, effectiveConfig, mqCreateJobConsumer),
		configSyncer:  newRuntimeConfigSyncer(runtimeConfigCache, db, effectiveConfig, baseConfig.EffectiveNodeMode()),
	}, nil
}

// Run 启动服务实例。
func (a *Application) Run(ctx context.Context) error {
	errCh := make(chan error, 5)
	httpTask := &managedTask{name: "http"}
	callbackTask := &managedTask{name: "callback"}

	go func() { errCh <- a.internalGRPC.Start(ctx) }()
	go func() { errCh <- a.scheduler.Start(ctx) }()
	go func() { errCh <- a.worker.Start(ctx) }()
	go func() { errCh <- a.coordinator.Start(ctx) }()
	go func() { errCh <- a.configSyncer.Start(ctx) }()
	reconcileTicker := time.NewTicker(time.Second)
	defer reconcileTicker.Stop()
	current := a.dynamicConfig.Snapshot()
	httpTask.Reconcile(ctx, shouldRunHTTPServer(a.baseConfig.EffectiveNodeMode(), current), a.baseConfig.Server.ListenAddress, a.httpServer.Start)
	callbackTask.Reconcile(ctx, current.Mode.EnableCallback, "callback", a.callback.Start)
	a.grpcServer.Reconcile(ctx)
	a.mqConsumer.Reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-reconcileTicker.C:
			cfg := a.dynamicConfig.Snapshot()
			httpTask.Reconcile(ctx, shouldRunHTTPServer(a.baseConfig.EffectiveNodeMode(), cfg), a.baseConfig.Server.ListenAddress, a.httpServer.Start)
			callbackTask.Reconcile(ctx, cfg.Mode.EnableCallback, "callback", a.callback.Start)
			a.grpcServer.Reconcile(ctx)
			a.mqConsumer.Reconcile(ctx)
			// A1 对账：已完成且"本任务待传分片数"已归零的任务要发布（写回调 outbox 事件）。
			// 就地触发（分片上传成功时）是主路径；这里兜住"最后一片由别的节点上传"与本进程重启
			// 这两类没有本地唤醒的情况，判据与 publishCompletion 一致。
			if a.worker != nil {
				a.worker.ReconcilePublish(ctx)
			}
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
}

func shouldRunHTTPServer(nodeMode string, cfg config.DynamicRuntimeConfig) bool {
	if cfg.Mode.EnableHTTPServer {
		return true
	}
	// cluster-worker 节点即便不承载北向 HTTP，也仍需要内部控制面入口，
	// 用于接收控制节点转发的直播启停、心跳与分片事件上报。
	return nodeMode == config.NodeModeClusterWorker
}

// loadPublishedDynamicRuntimeConfig 按“Redis 优先、数据库回源、再回填 Redis”的顺序加载已发布运行时配置。
//
// 这样做的目的有两个：
// 1. 启动和热重载尽量避免直接把数据库打成热点；
// 2. 即便 Redis 因 TTL 或故障丢失，也能自动回源并恢复缓存。
func loadPublishedDynamicRuntimeConfig(ctx context.Context, runtimeConfigCache *rediscache.RuntimeConfigCache, db *mysql.DB, nodeMode string) (rediscache.RuntimeConfigSnapshot, bool) {
	if runtimeConfigCache != nil {
		if snapshot, ok, err := runtimeConfigCache.LoadPublished(ctx); err == nil && ok {
			snapshot.Config = config.ApplyNodeModeRuntimeConstraints(nodeMode, snapshot.Config)
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
		Config:        config.ApplyNodeModeRuntimeConstraints(nodeMode, cfg),
	}

	if runtimeConfigCache != nil {
		_ = runtimeConfigCache.SavePublished(ctx, snapshot)
	}
	return snapshot, true
}
