package opslog

import (
	"context"
	"encoding/json"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// Record 表示一条运行日志落库记录。
type Record struct {
	LogID      uint64
	Service    string
	Level      string
	ActionName string
	FieldsJSON string
	LoggedAt   time.Time
}

// Repository 表示运行日志仓储。
type Repository struct {
	db *mysql.DB
}

// NewRepository 创建运行日志仓储。
func NewRepository(db *mysql.DB) *Repository {
	return &Repository{db: db}
}

// Save 保存运行日志。
func (r *Repository) Save(ctx context.Context, record Record) error {
	if r == nil || r.db == nil {
		return nil
	}
	if record.LogID == 0 {
		record.LogID = idgen.Next()
	}
	if record.LoggedAt.IsZero() {
		record.LoggedAt = time.Now()
	}
	payload := map[string]any{
		"log_id":       record.LogID,
		"service_name": record.Service,
		"log_level":    record.Level,
		"action_name":  record.ActionName,
		"fields_json":  record.FieldsJSON,
		"logged_at":    record.LoggedAt,
	}
	return r.db.WithContext(ctx).Table("t_system_runtime_log").Create(payload).Error
}

// SaveStructured 按结构化日志实体保存。
func (r *Repository) SaveStructured(ctx context.Context, item logx.PersistedEntry) error {
	fieldsJSON := "{}"
	if len(item.Fields) > 0 {
		if data, err := json.Marshal(item.Fields); err == nil {
			fieldsJSON = string(data)
		}
	}
	return r.Save(ctx, Record{
		Service:    item.Service,
		Level:      item.Level,
		ActionName: item.Action,
		FieldsJSON: fieldsJSON,
		LoggedAt:   item.LoggedAt,
	})
}
