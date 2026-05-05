package mysql

import (
	"context"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"hvc/internal/config"
	"time"
)

// DB 表示数据库访问对象。
type DB struct {
	Engine *gorm.DB
}

// Open 打开数据库连接。
func Open(cfg config.MySQLConfig) (*DB, error) {
	engine, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := engine.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	return &DB{Engine: engine}, nil
}

// WithContext 返回绑定上下文的数据库句柄。
func (db *DB) WithContext(ctx context.Context) *gorm.DB {
	return db.Engine.WithContext(ctx)
}
