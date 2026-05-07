package consumer

import (
	"context"
	"encoding/json"
	"github.com/rabbitmq/amqp091-go"
	"hvc/internal/model"
	transcodesvc "hvc/internal/service/transcode"
)

// CreateJobConsumer 表示创建任务消费者。
type CreateJobConsumer struct {
	service *transcodesvc.Service
}

// NewCreateJobConsumer 创建创建任务消费者。
func NewCreateJobConsumer(service *transcodesvc.Service) *CreateJobConsumer {
	return &CreateJobConsumer{service: service}
}

// Consume 处理一条消息。
func (c *CreateJobConsumer) Consume(ctx context.Context, delivery amqp091.Delivery) error {
	var req model.CreateJobRequest
	if err := json.Unmarshal(delivery.Body, &req); err != nil {
		return err
	}
	_, err := c.service.CreateJob(ctx, req)
	return err
}
