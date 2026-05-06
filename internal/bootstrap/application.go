package bootstrap

import (
	"context"
	"hvc/internal/callback"
	"hvc/internal/cluster"
	hotpath "hvc/internal/cluster/hotpath"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/handler"
	publichttp "hvc/internal/interfaces/http/public"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/scheduler"
	"hvc/internal/server"
	"hvc/internal/service"
	livesvc "hvc/internal/service/live"
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
	dynamicConfigValue, err := config.LoadDynamicRuntimeConfig(baseConfig)
	if err != nil {
		return nil, err
	}
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
	effectiveConfig := configcenter.NewEffectiveConfig(dynamicConfigValue)
	systemService := service.NewSystemService(baseConfig.Server.ServiceName, resolveModeName(dynamicConfigValue))
	systemHandler := handler.NewSystemHandler(systemService)
	channelService := livesvc.NewChannelService()
	jobRepository := mysql.NewJobRepository(db)
	segmentRepository := mysql.NewSegmentRepository(db)
	outboxRepository := mysql.NewOutboxRepository(db)
	progressStore := rediscache.NewProgressStore(redisClient)
	clusterCache := cluster.NewStateCache(redisClient)
	leaseCache := cluster.NewLeaseCache(redisClient)
	hotpathBus := hotpath.NewMemoryBus()
	createJobUseCase := transcodeusecase.NewCreateJobUseCase()
	queryProgressUseCase := transcodeusecase.NewQueryProgressUseCase(jobRepository, progressStore)
	transcodeHandler := publichttp.NewTranscodeHandler(createJobUseCase, queryProgressUseCase, jobRepository)
	clusterHandler := publichttp.NewClusterHandler(clusterCache, leaseCache, jobRepository, segmentRepository)
	liveHandler := publichttp.NewLiveHandler(channelService)
	return &Application{
		baseConfig:    baseConfig,
		dynamicConfig: effectiveConfig,
		httpServer:    server.NewHTTPServer(baseConfig.Server, systemHandler, transcodeHandler, clusterHandler, liveHandler),
		coordinator:   cluster.NewCoordinator(dynamicConfigValue),
		clusterCache:  clusterCache,
		leaseCache:    leaseCache,
		hotpathBus:    hotpathBus,
		scheduler:     scheduler.NewManager(dynamicConfigValue, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, clusterCache, jobRepository),
		worker:        worker.NewModule(dynamicConfigValue, baseConfig.Server.NodeID, baseConfig.Server.WorkerID, jobRepository, segmentRepository, progressStore, outboxRepository, hotpathBus),
		callback:      callback.NewDispatcher(dynamicConfigValue, outboxRepository),
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
