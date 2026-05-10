package opslog

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Item 表示运行日志查询结果。
type Item struct {
	LogID      uint64         `json:"log_id"`
	Service    string         `json:"service_name"`
	Level      string         `json:"log_level"`
	ActionName string         `json:"action_name"`
	FieldsJSON string         `json:"-"`
	Fields     map[string]any `json:"fields"`
	LoggedAt   time.Time      `json:"logged_at"`
}

// ListPaged 返回运行日志分页列表。
func (r *Repository) ListPaged(ctx context.Context, page, pageSize int, level, actionName string, startTime, endTime *time.Time) ([]Item, int64) {
	if r == nil || r.db == nil {
		return nil, 0
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Table("t_system_runtime_log")
	if level = strings.TrimSpace(level); level != "" {
		query = query.Where("log_level = ?", level)
	}
	if actionName = strings.TrimSpace(actionName); actionName != "" {
		query = query.Where("action_name = ?", actionName)
	}
	if startTime != nil {
		query = query.Where("logged_at >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("logged_at <= ?", *endTime)
	}

	var total int64
	query.Count(&total)

	var rows []struct {
		LogID      uint64    `gorm:"column:log_id"`
		Service    string    `gorm:"column:service_name"`
		Level      string    `gorm:"column:log_level"`
		ActionName string    `gorm:"column:action_name"`
		FieldsJSON string    `gorm:"column:fields_json"`
		LoggedAt   time.Time `gorm:"column:logged_at"`
	}
	offset := (page - 1) * pageSize
	query.Order("logged_at desc").Offset(offset).Limit(pageSize).Find(&rows)

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		item := Item{
			LogID:      row.LogID,
			Service:    row.Service,
			Level:      row.Level,
			ActionName: row.ActionName,
			FieldsJSON: row.FieldsJSON,
			LoggedAt:   row.LoggedAt,
			Fields:     map[string]any{},
		}
		if strings.TrimSpace(row.FieldsJSON) != "" {
			_ = json.Unmarshal([]byte(row.FieldsJSON), &item.Fields)
		}
		items = append(items, item)
	}
	return items, total
}
