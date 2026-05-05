package audit

import (
	"context"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/pkg/idgen"
)

// Record 表示审计记录。
type Record struct {
	AuditID      uint64
	Action       string
	TargetType   string
	TargetID     string
	RequestID    string
	RequestIP    string
	UserAgent    string
	ResultCode   int
	ResultMessage string
	CreatedAt    time.Time
}

// Repository 表示审计仓储。
type Repository struct {
	db *mysql.DB
}

// NewRepository 创建审计仓储。
func NewRepository(db *mysql.DB) *Repository {
	return &Repository{db: db}
}

// Save 保存审计记录。
func (r *Repository) Save(ctx context.Context, record Record) error {
	payload := map[string]any{
		"audit_log_id":    idgen.Next(),
		"action_name":     record.Action,
		"target_type":     record.TargetType,
		"target_id":       record.TargetID,
		"request_id":      record.RequestID,
		"request_ip":      record.RequestIP,
		"request_user_agent": record.UserAgent,
		"result_code":     record.ResultCode,
		"result_message":  record.ResultMessage,
		"created_at":      time.Now(),
	}
	return r.db.WithContext(ctx).Table("t_admin_audit_log").Create(payload).Error
}
