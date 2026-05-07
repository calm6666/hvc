package audit

import (
	"context"
	"time"
)

// Item 表示审计日志查询结果。
type Item struct {
	AuditLogID        uint64    `json:"audit_log_id"`
	AdminUserID       uint64    `json:"admin_user_id"`
	Username          string    `json:"username"`
	ActionName        string    `json:"action_name"`
	TargetType        string    `json:"target_type"`
	TargetID          string    `json:"target_id"`
	RequestID         string    `json:"request_id"`
	RequestIP         string    `json:"request_ip"`
	RequestUserAgent  string    `json:"request_user_agent"`
	ResultCode        int       `json:"result_code"`
	ResultMessage     string    `json:"result_message"`
	CreatedAt         time.Time `json:"created_at"`
}

// List 返回审计日志列表。
func (r *Repository) List(ctx context.Context) []Item {
	var items []Item
	if err := r.db.WithContext(ctx).Table("t_admin_audit_log").Order("created_at desc").Find(&items).Error; err != nil {
		return nil
	}
	return items
}

// ListPaged 返回审计日志分页列表。
//
// 参数：
//   - page: 页码，从 1 开始
//   - pageSize: 每页条数
//   - actionFilter: 操作名称过滤（可选）
//   - userIDFilter: 用户 ID 过滤（可选）
//   - startTime: 起始时间过滤（可选）
//   - endTime: 结束时间过滤（可选）
func (r *Repository) ListPaged(ctx context.Context, page, pageSize int, actionFilter string, userIDFilter uint64, startTime, endTime *time.Time) ([]Item, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Table("t_admin_audit_log")
	if actionFilter != "" {
		query = query.Where("action_name = ?", actionFilter)
	}
	if userIDFilter > 0 {
		query = query.Where("admin_user_id = ?", userIDFilter)
	}
	if startTime != nil {
		query = query.Where("created_at >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", *endTime)
	}

	var total int64
	query.Count(&total)

	var items []Item
	offset := (page - 1) * pageSize
	query.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&items)

	return items, total
}

// CleanBefore 清理指定时间之前的审计日志。
func (r *Repository) CleanBefore(ctx context.Context, before time.Time) int {
	result := r.db.WithContext(ctx).Table("t_admin_audit_log").Where("created_at < ?", before).Delete(nil)
	return int(result.RowsAffected)
}
