package mysql

import (
	"context"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"hvc/internal/config"
	"hvc/pkg/logx"
	"time"
)

// DB 表示数据库访问对象。
type DB struct {
	Engine *gorm.DB
}

// Open 打开数据库连接。
func Open(cfg config.MySQLConfig) (*DB, error) {
	engine, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: logx.NewGormLogger(),
	})
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

// ServerVersion 返回当前 MySQL 服务端版本号。
//
// 该信息主要用于后台观测接口，帮助快速确认实例运行环境和排障基线。
func (db *DB) ServerVersion(ctx context.Context) string {
	if db == nil || db.Engine == nil {
		return ""
	}

	var version string
	row := db.WithContext(ctx).Raw("SELECT VERSION()").Row()
	if row == nil {
		return ""
	}
	if err := row.Scan(&version); err != nil {
		return ""
	}
	return version
}
