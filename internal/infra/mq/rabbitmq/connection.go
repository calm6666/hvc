// Package rabbitmq 提供 RabbitMQ 连接管理。
package rabbitmq

import (
	"fmt"
	"strings"

	"github.com/rabbitmq/amqp091-go"
	"hvc/internal/config"
)

// Connection 表示 RabbitMQ 连接包装。
type Connection struct {
	Engine *amqp091.Connection
	dsn    string
}

// Open 打开 RabbitMQ 连接。
//
// 连接串统一由 Host、Port、User、Password、VHost 组合生成，
// 避免同时维护 DSN 与拆分字段两套来源导致配置语义分裂。
// 默认使用 amqp://guest:guest@127.0.0.1:5672/
func Open(cfg config.RuntimeConfig) (*Connection, error) {
	dsn := buildDSN(cfg)
	return openDSN(dsn)
}

// OpenRuntime 根据运行时 MQ 配置打开连接。
func OpenRuntime(cfg config.MQRuntimeConfig) (*Connection, error) {
	return openDSN(buildRuntimeDSN(cfg))
}

func openDSN(dsn string) (*Connection, error) {
	conn, err := amqp091.Dial(dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 RabbitMQ 失败 (%s): %w", maskDSN(dsn), err)
	}
	return &Connection{Engine: conn, dsn: dsn}, nil
}

// Channel 创建一个新的 RabbitMQ 通道。
func (c *Connection) Channel() (*amqp091.Channel, error) {
	if c.Engine == nil {
		return nil, fmt.Errorf("RabbitMQ 连接未建立")
	}
	return c.Engine.Channel()
}

// IsClosed 检查连接是否已关闭。
func (c *Connection) IsClosed() bool {
	return c.Engine == nil || c.Engine.IsClosed()
}

// Close 关闭 RabbitMQ 连接。
func (c *Connection) Close() error {
	if c.Engine == nil {
		return nil
	}
	return c.Engine.Close()
}

// DSN 返回连接使用的 DSN（脱敏后）。
func (c *Connection) DSN() string {
	return maskDSN(c.dsn)
}

// buildDSN 根据配置构建 RabbitMQ 连接字符串。
func buildDSN(cfg config.RuntimeConfig) string {
	mqCfg := cfg.MQ
	host := mqCfg.RabbitMQHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := mqCfg.RabbitMQPort
	if port == 0 {
		port = 5672
	}
	user := mqCfg.RabbitMQUser
	if user == "" {
		user = "guest"
	}
	password := mqCfg.RabbitMQPassword
	if password == "" {
		password = "guest"
	}
	vhost := mqCfg.RabbitMQVHost
	if vhost == "" {
		vhost = "/"
	}

	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s", user, password, host, port, vhost)
}

func buildRuntimeDSN(cfg config.MQRuntimeConfig) string {
	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 5672
	}
	user := cfg.Username
	if user == "" {
		user = "guest"
	}
	password := cfg.Password
	if password == "" {
		password = "guest"
	}
	vhost := cfg.VHost
	if vhost == "" {
		vhost = "/"
	}
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s", user, password, host, port, vhost)
}

// maskDSN 对 DSN 中的密码进行脱敏，防止日志泄露。
//
// 将 amqp://user:password@host:port/vhost 转换为 amqp://user:****@host:port/vhost
func maskDSN(dsn string) string {
	atIdx := strings.Index(dsn, "@")
	if atIdx < 0 {
		return dsn
	}
	prefix := dsn[:atIdx]
	colonIdx := strings.LastIndex(prefix, ":")
	if colonIdx < 0 {
		return dsn
	}
	return prefix[:colonIdx+1] + "****" + dsn[atIdx:]
}
