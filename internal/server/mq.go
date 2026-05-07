package server

import (
	"context"
	"sync"

	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/infra/mq/rabbitmq"
	mqconsumer "hvc/internal/interfaces/mq/consumer"
	"hvc/pkg/logx"
)

// MQConsumer 管理 MQ 消费者的生命周期，支持动态启停和连接参数切换。
type MQConsumer struct {
	initialConfig   config.DynamicRuntimeConfig
	effectiveConfig *configcenter.EffectiveConfig
	consumer        *mqconsumer.CreateJobConsumer
	mu              sync.Mutex
	running         bool
	cancel          context.CancelFunc
	currentDSNKey   string
}

// NewMQConsumer 创建 MQ 消费者管理器。
func NewMQConsumer(initialConfig config.DynamicRuntimeConfig, effectiveConfig *configcenter.EffectiveConfig, consumer *mqconsumer.CreateJobConsumer) *MQConsumer {
	return &MQConsumer{
		initialConfig:   initialConfig,
		effectiveConfig: effectiveConfig,
		consumer:        consumer,
	}
}

// Reconcile 根据当前生效配置协调 MQ 消费者的启停状态。
func (c *MQConsumer) Reconcile(parent context.Context) {
	cfg := c.currentConfig()
	dsnKey := mqConfigKey(cfg.MQ)

	c.mu.Lock()
	defer c.mu.Unlock()

	if !cfg.Mode.EnableMQConsumer || cfg.MQ.QueueName == "" {
		c.stopLocked()
		return
	}
	if c.running && c.currentDSNKey == dsnKey {
		return
	}
	c.stopLocked()

	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.running = true
	c.currentDSNKey = dsnKey
	go c.consumeLoop(ctx, cfg)
}

func (c *MQConsumer) consumeLoop(ctx context.Context, cfg config.DynamicRuntimeConfig) {
	conn, err := rabbitmq.OpenRuntime(cfg.MQ)
	if err != nil {
		logx.Error("mq.consumer.connect_failed", err, logx.Fields{"queue": cfg.MQ.QueueName})
		c.markStopped(cfg)
		return
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		logx.Error("mq.consumer.channel_failed", err, logx.Fields{"queue": cfg.MQ.QueueName})
		c.markStopped(cfg)
		return
	}
	defer channel.Close()

	if cfg.MQ.PrefetchCount > 0 {
		if err := channel.Qos(cfg.MQ.PrefetchCount, 0, false); err != nil {
			logx.Error("mq.consumer.qos_failed", err, logx.Fields{"queue": cfg.MQ.QueueName})
		}
	}
	deliveries, err := channel.Consume(cfg.MQ.QueueName, cfg.MQ.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		logx.Error("mq.consumer.start_failed", err, logx.Fields{"queue": cfg.MQ.QueueName})
		c.markStopped(cfg)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				c.markStopped(cfg)
				return
			}
			if err := c.consumer.Consume(ctx, delivery); err != nil {
				logx.Error("mq.consumer.consume_failed", err, logx.Fields{"queue": cfg.MQ.QueueName})
				_ = delivery.Nack(false, true)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}

func (c *MQConsumer) currentConfig() config.DynamicRuntimeConfig {
	if c.effectiveConfig == nil {
		return c.initialConfig
	}
	return c.effectiveConfig.Snapshot()
}

func (c *MQConsumer) stopLocked() {
	if c.cancel != nil {
		c.cancel()
	}
	c.cancel = nil
	c.running = false
	c.currentDSNKey = ""
}

func (c *MQConsumer) markStopped(cfg config.DynamicRuntimeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.currentDSNKey == mqConfigKey(cfg.MQ) {
		c.running = false
		c.currentDSNKey = ""
		c.cancel = nil
	}
}

func mqConfigKey(cfg config.MQRuntimeConfig) string {
	return cfg.Host + "|" + cfg.QueueName + "|" + cfg.ConsumerTag
}
