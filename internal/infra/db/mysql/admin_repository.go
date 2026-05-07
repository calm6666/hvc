package mysql

import (
	"context"
	"strings"
	"time"

	"hvc/pkg/idgen"
)

// AdminRepository 表示管理员与 RBAC 读取写入仓储。
//
// 第一阶段的目标不是做抽象上的完美 RBAC，而是先把：
// 1. 管理员登录会话；
// 2. 用户-角色绑定；
// 3. 角色-权限绑定；
// 4. 权限校验查询；
// 这些后台基础公共能力真正接通。
type AdminRepository struct {
	db *DB
}

// NewAdminRepository 创建管理员仓储。
func NewAdminRepository(db *DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// FindUserByUsername 根据用户名查询管理员用户。
func (r *AdminRepository) FindUserByUsername(ctx context.Context, username string) (AdminUserRecord, bool) {
	var record AdminUserRecord
	if err := r.db.WithContext(ctx).Where("username = ?", username).Take(&record).Error; err != nil {
		return AdminUserRecord{}, false
	}
	return record, true
}

// CreateSession 创建管理员会话。
func (r *AdminRepository) CreateSession(ctx context.Context, adminUserID uint64, token, loginIP, userAgent string, expireAt time.Time) (AdminSessionRecord, error) {
	now := time.Now()
	record := AdminSessionRecord{
		SessionID:     idgen.Next(),
		AdminUserID:   adminUserID,
		SessionToken:  token,
		SessionStatus: 1,
		LoginIP:       loginIP,
		UserAgent:     userAgent,
		ExpireAt:      expireAt,
		LastSeenAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return AdminSessionRecord{}, err
	}
	return record, nil
}

// FindActiveSession 根据 session token 查询有效会话。
func (r *AdminRepository) FindActiveSession(ctx context.Context, token string) (AdminSessionRecord, bool) {
	var record AdminSessionRecord
	if err := r.db.WithContext(ctx).
		Where("session_token = ? AND session_status = ?", token, 1).
		Take(&record).Error; err != nil {
		return AdminSessionRecord{}, false
	}
	if !record.ExpireAt.IsZero() && record.ExpireAt.Before(time.Now()) {
		return AdminSessionRecord{}, false
	}
	return record, true
}

// TouchSession 更新管理员会话最近访问时间。
func (r *AdminRepository) TouchSession(ctx context.Context, sessionID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&AdminSessionRecord{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{"last_seen_at": now, "updated_at": now}).Error
}

// RevokeSession 注销管理员会话。
func (r *AdminRepository) RevokeSession(ctx context.Context, sessionID uint64) error {
	return r.db.WithContext(ctx).Model(&AdminSessionRecord{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{"session_status": 2, "updated_at": time.Now()}).Error
}

// ListPermissionKeysByUserID 查询管理员拥有的全部权限键。
func (r *AdminRepository) ListPermissionKeysByUserID(ctx context.Context, userID uint64) []string {
	rows, err := r.db.WithContext(ctx).Raw(`
SELECT p.perm_key
FROM t_admin_user_role ur
JOIN t_admin_role_permission rp ON rp.role_id = ur.role_id
JOIN t_admin_permission p ON p.perm_id = rp.perm_id
WHERE ur.user_id = ?
`, userID).Rows()
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]string, 0, 16)
	for rows.Next() {
		var permKey string
		if err := rows.Scan(&permKey); err != nil {
			continue
		}
		permKey = strings.TrimSpace(permKey)
		if permKey != "" {
			items = append(items, permKey)
		}
	}
	return items
}
