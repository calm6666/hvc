package mysql

import "context"

// DeliveryFailureQueueRepository 表示外部投递失败补偿记录仓储。
type DeliveryFailureQueueRepository struct {
	db *DB
}

// NewDeliveryFailureQueueRepository 创建失败补偿记录仓储。
func NewDeliveryFailureQueueRepository(db *DB) *DeliveryFailureQueueRepository {
	return &DeliveryFailureQueueRepository{db: db}
}

// Save 保存一次失败投递记录。
func (r *DeliveryFailureQueueRepository) Save(ctx context.Context, record DeliveryFailureQueueRecord) error {
	return r.db.WithContext(ctx).Create(&record).Error
}
