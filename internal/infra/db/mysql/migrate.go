package mysql

import "context"

// AutoMigrate 执行数据库模型迁移。
func (db *DB) AutoMigrate(ctx context.Context) error {
	return db.WithContext(ctx).AutoMigrate(&JobRecord{}, &SegmentRecord{}, &OutboxRecord{})
}
