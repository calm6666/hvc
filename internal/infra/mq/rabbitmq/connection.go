package rabbitmq

import (
	"github.com/rabbitmq/amqp091-go"
	"hvc/internal/config"
)

// Connection 表示 RabbitMQ 连接包装。
type Connection struct {
	Engine *amqp091.Connection
}

// Open 打开 RabbitMQ 连接。
func Open(cfg config.RuntimeConfig) (*Connection, error) {
	conn, err := amqp091.Dial("amqp://guest:guest@127.0.0.1:5672/")
	if err != nil {
		return nil, err
	}
	_ = cfg
	return &Connection{Engine: conn}, nil
}
