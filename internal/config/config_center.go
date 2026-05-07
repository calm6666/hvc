package config

import "time"

// LoadBootstrapDynamicRuntimeConfig 构造首启时使用的运行时配置默认值。
//
// 约束如下：
// 1. 运行期业务配置的唯一生效来源是后台发布到数据库的 runtime config；
// 2. 本地 YAML 里的动态段只用于首启初始化默认值，不参与后续热更新；
// 3. 外部配置中心不再直接下发业务运行时配置，只负责启动所需的基础设施配置。
func LoadBootstrapDynamicRuntimeConfig(base RuntimeConfig) DynamicRuntimeConfig {
	cfg := DynamicRuntimeConfig{
		Mode: ModeConfig{
			EnableHTTPServer: true,
			EnableGRPCServer: base.GRPC.ListenAddress != "",
			EnableMQConsumer: (base.MQ.RabbitMQDSN != "" || base.MQ.RabbitMQHost != "") && base.MQ.QueueName != "",
			EnableScheduler:  true,
			EnableWorker:     true,
			EnableCallback:   true,
		},
		Scheduler: base.Scheduler,
		Worker:    base.Worker,
		Callback:  base.Callback,
		Storage:   base.Storage,
		GRPC:      base.GRPC,
		MQ: MQRuntimeConfig{
			QueueName:     base.MQ.QueueName,
			Host:          base.MQ.RabbitMQHost,
			Port:          base.MQ.RabbitMQPort,
			Username:      base.MQ.RabbitMQUser,
			Password:      base.MQ.RabbitMQPassword,
			VHost:         base.MQ.RabbitMQVHost,
			ConsumerTag:   base.MQ.ConsumerTag,
			PrefetchCount: base.MQ.PrefetchCount,
			LoopInterval:  base.MQ.LoopInterval,
			CallbackTopic: base.MQ.CallbackTopic,
		},
	}
	applyDynamicDefaults(&cfg)
	return cfg
}

func applyDynamicDefaults(cfg *DynamicRuntimeConfig) {
	if cfg == nil {
		return
	}

	if cfg.Scheduler.LoopInterval <= 0 {
		cfg.Scheduler.LoopInterval = 5 * time.Second
	}
	if cfg.Scheduler.JobLeaseTTL <= 0 {
		cfg.Scheduler.JobLeaseTTL = time.Minute
	}
	if cfg.Scheduler.WorkerHeartbeatTimeout <= 0 {
		cfg.Scheduler.WorkerHeartbeatTimeout = 20 * time.Second
	}
	if cfg.Scheduler.SoftDecodeCPULimitPercent <= 0 {
		cfg.Scheduler.SoftDecodeCPULimitPercent = 50
	}
	if cfg.Scheduler.NodeCPUSafetyLimitPercent <= 0 {
		cfg.Scheduler.NodeCPUSafetyLimitPercent = 85
	}
	if cfg.Scheduler.NodeMemorySafetyLimitPercent <= 0 {
		cfg.Scheduler.NodeMemorySafetyLimitPercent = 85
	}
	if cfg.Scheduler.NodeGPUSafetyLimitPercent <= 0 {
		cfg.Scheduler.NodeGPUSafetyLimitPercent = 90
	}
	if cfg.Scheduler.MaxGlobalTranscodeSessions <= 0 {
		cfg.Scheduler.MaxGlobalTranscodeSessions = 10
	}
	if cfg.Scheduler.MaxNodeTranscodeSessions <= 0 {
		cfg.Scheduler.MaxNodeTranscodeSessions = cfg.Scheduler.MaxGlobalTranscodeSessions
	}
	if cfg.Scheduler.MaxNodeUploadConcurrency <= 0 {
		cfg.Scheduler.MaxNodeUploadConcurrency = 4
	}

	if cfg.Worker.LoopInterval <= 0 {
		cfg.Worker.LoopInterval = 15 * time.Second
	}
	if cfg.Worker.SingleJobUploadConcurrency <= 0 {
		cfg.Worker.SingleJobUploadConcurrency = 4
	}
	if cfg.Worker.UploadRetryBaseDelay <= 0 {
		cfg.Worker.UploadRetryBaseDelay = time.Second
	}
	if cfg.Worker.UploadRetryMaxDelay <= 0 {
		cfg.Worker.UploadRetryMaxDelay = 30 * time.Second
	}
	if cfg.Worker.UploadMaxRetryCount <= 0 {
		cfg.Worker.UploadMaxRetryCount = 3
	}
	if cfg.Worker.SegmentTemplate == "" {
		cfg.Worker.SegmentTemplate = "{job_id}-{media_type}-{number}.m4s"
	}

	if cfg.Callback.HTTPTimeout <= 0 {
		cfg.Callback.HTTPTimeout = 5 * time.Second
	}
	if cfg.Callback.GRPCTimeout <= 0 {
		cfg.Callback.GRPCTimeout = 3 * time.Second
	}
	if cfg.Callback.MQTimeout <= 0 {
		cfg.Callback.MQTimeout = 3 * time.Second
	}
	if cfg.Callback.RetryBackoff <= 0 {
		cfg.Callback.RetryBackoff = 2 * time.Second
	}

	if cfg.GRPC.MaxRecvMsgSizeMB <= 0 {
		cfg.GRPC.MaxRecvMsgSizeMB = 64
	}
	if cfg.GRPC.MaxSendMsgSizeMB <= 0 {
		cfg.GRPC.MaxSendMsgSizeMB = 64
	}
	if cfg.GRPC.ConnectionTimeout <= 0 {
		cfg.GRPC.ConnectionTimeout = 5 * time.Second
	}

	if cfg.MQ.Port <= 0 {
		cfg.MQ.Port = 5672
	}
	if cfg.MQ.VHost == "" {
		cfg.MQ.VHost = "/"
	}
	if cfg.MQ.ConsumerTag == "" {
		cfg.MQ.ConsumerTag = "vod-mq-consumer"
	}
	if cfg.MQ.PrefetchCount <= 0 {
		cfg.MQ.PrefetchCount = 10
	}
	if cfg.MQ.LoopInterval <= 0 {
		cfg.MQ.LoopInterval = 15 * time.Second
	}
}
