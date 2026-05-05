package producer

import (
	"context"
	"encoding/json"
	"hvc/internal/model"
	"github.com/rabbitmq/amqp091-go"
)

// Publisher 表示 MQ 生产者。
type Publisher struct {
	channel *amqp091.Channel
	exchange string
	routingKey string
}

// NewPublisher 创建 MQ 生产者。
func NewPublisher(channel *amqp091.Channel, exchange string, routingKey string) *Publisher {
	return &Publisher{channel: channel, exchange: exchange, routingKey: routingKey}
}

// PublishOutbox 发布 outbox 事件。
func (p *Publisher) PublishOutbox(ctx context.Context, event model.OutboxEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.channel.PublishWithContext(ctx, p.exchange, p.routingKey, false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body: payload,
	})
}
