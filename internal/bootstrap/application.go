package bootstrap

import (
	"context"
	"fmt"
	"hvc/internal/auth"
	"hvc/internal/audit"
	"hvc/internal/callback"
	"hvc/internal/cluster"
	hotpath "hvc/internal/cluster/hotpath"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/handler"
	"hvc/internal/manifest"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	adminhttp "hvc/internal/interfaces/http/admin"
	publichttp "hvc/internal/interfaces/http/public"
	"hvc/internal/live"
	livesvc "hvc/internal/service/live"
	"hvc/internal/scheduler"
	"hvc/internal/server"
	"hvc/internal/service"
	authusecase "hvc/internal/usecase/auth"
	transcodeusecase "hvc/internal/usecase/transcode"
	"hvc/internal/worker"
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
	dynamicConfigValue, ok := func() (config.DynamicRuntimeConfig, bool) {
		result, found := mysql.LoadDynamicRuntimeConfigFromDB(context.Background(), db)
		if !found {
			return config.DynamicRuntimeConfig{}, false
		}
		cfg, ok := result.Config.(config.DynamicRuntimeConfig)
		return cfg, ok
	}()
	if !ok {
		dynamicConfigValue, err = config.LoadDynamicRuntimeConfig(baseConfig)
		if err != nil {
			return nil, fmt.Errorf("load dynamic runtime config failed: %w", err)
		}
	}
	effectiveConfig := configcenter.NewEffectiveConfig(dynamicConfigValue)
	coordinator := cluster.NewCoordinator(dynamicConfigValue, baseConfig.Server.NodeID, baseConfig.Server.ListenAddress)
	systemService := service.NewSystemService(baseConfig.Server.ServiceName, resolveModeName(dynamicConfigValue))
	systemHandler := handler.NewSystemHandler(systemService)
	channelService := livesvc.NewChannelService(dynamicConfigValue)
	auditRepository := audit.NewRepository(db)
	jobRepository := mysql.NewJobRepository(db)
	jobRequestOverrideRepository := mysql.NewJobRequestOverrideRepository(db)
	segmentRepository := mysql.NewSegmentRepository(db)
	outboxRepository := mysql.NewOutboxRepository(db)
	workerInstanceRepository := mysql.NewWorkerInstanceRepository(db)
	gpuDeviceRepository := mysql.NewGPUDeviceRepository(db)
	gpuCapabilityRepository := mysql.NewWorkerCodecCapabilityRepository(db)
	jobExecutionRepository := mysql.NewJobExecutionRepository(db)
	adminRepository := mysql.NewAdminRepository(db)
	adminRBACRepository := mysql.NewAdminRBACRepository(db)
	runtimeConfigRepository := mysql.NewRuntimeConfigRepository(db)
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
	transcodeHandler := publichttp.NewTranscodeHandler(createJobUseCase, queryProgressUseCase, jobRepository, jobRequestOverrideRepository)
	manifestBuilder := manifest.NewBuilder(segmentRepository, jobRepository, dynamicConfigValue.Storage.PlayDomain, dynamicConfigValue.Worker.SegmentTemplate)
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
	liveManager := live.NewManager(live.NewMemoryChannelStore(), live.NewMemorySessionStore())
	adminLiveHandler := adminhttp.NewLiveHandler(liveManager, channelService)
	return &Application{
		baseConfig:    baseConfig,
		dynamicConfig: effectiveConfig,
		httpServer:    server.NewHTTPServer(baseConfig.Server, systemHandler, transcodeHandler, clusterHandler, liveHandler, manifestHandler, authHandler, configHandler, callbackHandler, rbacHandler, writeRBACHandler, configCenterHandler, adminClusterHandler, adminTranscodeHandler, adminLiveHandler),
		coordinator:   coordinator,
		clusterCache:  clusterCache,
		leaseCache:    leaseCache,
		hotpathBus:    hotpathBus,
		scheduler:     scheduler.NewManager(dynamicConfigValue, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, clusterCache, jobRepository, jobRequestOverrideRepository, jobExecutionRepository),
		worker:        worker.NewModule(dynamicConfigValue, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, jobRepository, segmentRepository, progressStore, outboxRepository, hotpathBus, clusterCache, workerInstanceRepository, gpuDeviceRepository, gpuCapabilityRepository, jobExecutionRepository),
		callback:      callback.NewDispatcher(dynamicConfigValue, outboxRepository, callbackConfigRepository),
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
	current := a.dynamicConfig.Snapshot()

	if current.Mode.EnableHTTPServer {
		go func() { errCh <- a.httpServer.Start(ctx) }()
	}
	if current.Mode.EnableScheduler {
		go func() { errCh <- a.scheduler.Start(ctx) }()
	}
	if current.Mode.EnableWorker {
		go func() { errCh <- a.worker.Start(ctx) }()
	}
	if current.Mode.EnableCallback {
		go func() { errCh <- a.callback.Start(ctx) }()
	}
	go func() { errCh <- a.coordinator.Start(ctx) }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
}
