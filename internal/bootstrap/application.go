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
	"hvc/internal/scheduler"
	"hvc/internal/server"
	"hvc/internal/service"
	livesvc "hvc/internal/service/live"
	transcodesvc "hvc/internal/service/transcode"
	authusecase "hvc/internal/usecase/auth"
	transcodeusecase "hvc/internal/usecase/transcode"
	"hvc/internal/worker"
	"time"
)

// Application 表示服务进程装配结果。
type Application struct {
	baseConfig    config.RuntimeConfig
	dynamicConfig *configcenter.EffectiveConfig
	httpServer    *server.HTTPServer
	coordinator   *cluster.Coordinator
	clusterCache  *cluster.StateCache
	leaseCache    *cluster.LeaseCache
	hotpathBus    *hotpath.MemoryBus
	scheduler     *scheduler.Manager
	worker        *worker.Module
	callback      *callback.Dispatcher
	grpcServer    *server.GRPCServer
	mqConsumer    *server.MQConsumer
}

// NewApplication 创建服务实例。
func NewApplication(baseConfig config.RuntimeConfig) (*Application, error) {
	db, err := mysql.Open(baseConfig.MySQL)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(context.Background()); err != nil {
		return nil, err
	}
	redisClient, err := rediscache.Open(baseConfig.Redis)
	if err != nil {
		return nil, err
	}
	runtimeConfigRepository := mysql.NewRuntimeConfigRepository(db)
	bootstrapDynamicConfig := config.LoadBootstrapDynamicRuntimeConfig(baseConfig)
	if err := runtimeConfigRepository.EnsureBootstrapPublished(context.Background(), mysql.NewBootstrapRuntimeConfigRecord(bootstrapDynamicConfig, "bootstrap_local")); err != nil {
		return nil, fmt.Errorf("ensure bootstrap runtime config failed: %w", err)
	}
	dynamicConfigValue, ok := func() (config.DynamicRuntimeConfig, bool) {
		result, found := mysql.LoadDynamicRuntimeConfigFromDB(context.Background(), db)
		if !found {
			return config.DynamicRuntimeConfig{}, false
		}
		cfg, ok := result.Config.(config.DynamicRuntimeConfig)
		return cfg, ok
	}()
	if !ok {
		dynamicConfigValue = bootstrapDynamicConfig
	}
	effectiveConfig := configcenter.NewEffectiveConfig(dynamicConfigValue)
	coordinator := cluster.NewCoordinator(dynamicConfigValue, baseConfig.Server.NodeID, baseConfig.Server.ListenAddress)
	systemService := service.NewSystemService(baseConfig.Server.ServiceName, resolveModeName(dynamicConfigValue))
	systemHandler := handler.NewSystemHandler(systemService)
	auditRepository := audit.NewRepository(db)
	jobRepository := mysql.NewJobRepository(db)
	jobRequestOverrideRepository := mysql.NewJobRequestOverrideRepository(db)
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
	channelService := livesvc.NewChannelService(dynamicConfigValue, liveChannelRepository, liveSessionRepository, liveProfileRenditionRepository)
	channelService.SetConfigSnapshot(effectiveConfig.Snapshot)
	transcodeHandler := publichttp.NewTranscodeHandler(transcodeService)
	manifestBuilder := manifest.NewBuilder(segmentRepository, jobRepository, effectiveConfig)
	manifestHandler := publichttp.NewManifestHandler(manifestBuilder)
	clusterHandler := publichttp.NewClusterHandler(clusterCache, leaseCache, jobRepository, segmentRepository, workerInstanceRepository)
	liveHandler := publichttp.NewLiveHandler(channelService)
	authHandler := adminhttp.NewAuthHandler(loginUseCase, adminRepository)
	configHandler := adminhttp.NewConfigHandler(runtimeConfigRepository, effectiveConfig)
	callbackHandler := adminhttp.NewCallbackHandler(callbackConfigRepository)
	rbacHandler := adminhttp.NewRBACHandler(adminRepository, adminRBACRepository, auditRepository)
	writeRBACHandler := adminhttp.NewWriteRBAC(adminRBACRepository, adminRepository)
	configCenterHandler := adminhttp.NewConfigCenterHandler(configCenterBindingRepository, effectiveConfig)
	adminClusterHandler := adminhttp.NewClusterHandler(clusterNodeRepository, clusterCache, coordinator.Registry())
	adminTranscodeHandler := adminhttp.NewTranscodeHandler(jobRepository, progressStore)
	namingTemplateRepository := mysql.NewNamingTemplateRepository(db)
	namingTemplateHandler := adminhttp.NewNamingTemplateHandler(namingTemplateRepository, runtimeConfigRepository, effectiveConfig)
	liveManager := live.NewManager(live.NewRepositoryChannelStore(liveChannelRepository), live.NewRepositorySessionStore(liveSessionRepository))
	adminLiveHandler := adminhttp.NewLiveHandler(liveManager, channelService)
	monitorHandler := wsmonitor.NewSnapshotHandler(clusterCache, hotpathBus, progressStore, jobRepository, clusterNodeRepository, resolveModeName(dynamicConfigValue))
	grpcPublicServer := grpcpublic.NewTranscodePublicServer(transcodeService)
	mqCreateJobConsumer := mqconsumer.NewCreateJobConsumer(transcodeService)
	schedulerManager := scheduler.NewManager(dynamicConfigValue, effectiveConfig, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, clusterCache, jobRepository, jobRequestOverrideRepository, jobExecutionRepository)
	schedulerManager.SetLeaseCache(leaseCache)
	schedulerManager.SetClusterNodeRepository(clusterNodeRepository)
	return &Application{
		baseConfig:    baseConfig,
		dynamicConfig: effectiveConfig,
		httpServer:    server.NewHTTPServer(baseConfig.Server, systemHandler, transcodeHandler, clusterHandler, liveHandler, manifestHandler, authHandler, configHandler, callbackHandler, rbacHandler, writeRBACHandler, configCenterHandler, adminClusterHandler, adminTranscodeHandler, adminLiveHandler, namingTemplateHandler, monitorHandler),
		coordinator:   coordinator,
		clusterCache:  clusterCache,
		leaseCache:    leaseCache,
		hotpathBus:    hotpathBus,
		scheduler:     schedulerManager,
		worker:        worker.NewModule(dynamicConfigValue, effectiveConfig, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, jobRepository, segmentRepository, progressStore, outboxRepository, hotpathBus, clusterCache, workerInstanceRepository, gpuDeviceRepository, gpuCapabilityRepository, jobExecutionRepository),
		callback:      callback.NewDispatcher(effectiveConfig, outboxRepository, callbackConfigRepository, jobRequestOverrideRepository, deliveryFailureQueueRepository),
		grpcServer:    server.NewGRPCServer(dynamicConfigValue, effectiveConfig, grpcPublicServer, clusterCache, progressStore, segmentRepository, jobRepository),
		mqConsumer:    server.NewMQConsumer(dynamicConfigValue, effectiveConfig, mqCreateJobConsumer),
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
	errCh := make(chan error, 4)
	httpTask := &managedTask{name: "http"}

	go func() { errCh <- a.scheduler.Start(ctx) }()
	go func() { errCh <- a.worker.Start(ctx) }()
	go func() { errCh <- a.callback.Start(ctx) }()
	go func() { errCh <- a.coordinator.Start(ctx) }()
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
