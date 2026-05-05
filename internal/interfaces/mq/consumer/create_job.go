package consumer

import (
	"context"
	"encoding/json"
	"hvc/internal/model"
	transcodeusecase "hvc/internal/usecase/transcode"
	"github.com/rabbitmq/amqp091-go"
)

// CreateJobConsumer 表示创建任务消费者。
type CreateJobConsumer struct {
	createJobUseCase *transcodeusecase.CreateJobUseCase
}

// NewCreateJobConsumer 创建创建任务消费者。
func NewCreateJobConsumer(createJobUseCase *transcodeusecase.CreateJobUseCase) *CreateJobConsumer {
	return &CreateJobConsumer{createJobUseCase: createJobUseCase}
}

// Consume 处理一条消息。
func (c *CreateJobConsumer) Consume(ctx context.Context, delivery amqp091.Delivery) error {
	var req model.CreateJobRequest
	if err := json.Unmarshal(delivery.Body, &req); err != nil {
		return err
	}
	_, err := c.createJobUseCase.Execute(req)
	return err
}
