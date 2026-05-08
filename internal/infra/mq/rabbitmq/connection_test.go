package rabbitmq

import (
	"testing"

	"hvc/internal/config"
)

func TestBuildDSNUsesSplitMQFields(t *testing.T) {
	dsn := buildDSN(config.RuntimeConfig{
		MQ: config.MQConfig{
			RabbitMQHost:     "mq.internal",
			RabbitMQPort:     5673,
			RabbitMQUser:     "transcoder",
			RabbitMQPassword: "secret",
			RabbitMQVHost:    "video",
		},
	})

	if dsn != "amqp://transcoder:secret@mq.internal:5673/video" {
		t.Fatalf("unexpected dsn: %s", dsn)
	}
}

func TestBuildDSNUsesDefaultsWhenFieldsMissing(t *testing.T) {
	dsn := buildDSN(config.RuntimeConfig{})

	if dsn != "amqp://guest:guest@127.0.0.1:5672//" {
		t.Fatalf("unexpected default dsn: %s", dsn)
	}
}
