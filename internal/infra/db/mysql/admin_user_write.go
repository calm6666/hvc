package mysql

import (
	"context"
	"fmt"
	"time"

	"hvc/pkg/idgen"
)

// SaveUser 保存或更新管理员用户。
func (r *AdminRepository) SaveUser(ctx context.Context, username, passwordHash, passwordSalt, displayName string, status int) (AdminUserRecord, error) {
	now := time.Now()
	var record AdminUserRecord
	result := r.db.WithContext(ctx).Where("username = ?", username).Limit(1).Find(&record)
	if result.Error != nil {
		return AdminUserRecord{}, result.Error
	}
	if result.RowsAffected > 0 {
		if passwordHash != "" {
			record.PasswordHash = passwordHash
			record.PasswordSalt = passwordSalt
		}
		record.DisplayName = displayName
		record.Status = status
		record.UpdatedAt = now
		return record, r.db.WithContext(ctx).Save(&record).Error
	}
	if passwordHash == "" {
		return AdminUserRecord{}, fmt.Errorf("password is required for new user")
	}
	record = AdminUserRecord{
		AdminUserID:  idgen.Next(),
		Username:     username,
		PasswordHash: passwordHash,
		PasswordSalt: passwordSalt,
		DisplayName:  displayName,
		Status:       status,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return record, r.db.WithContext(ctx).Create(&record).Error
}

// SetUserStatus 更新管理员状态。
func (r *AdminRepository) SetUserStatus(ctx context.Context, adminUserID uint64, status int) error {
	return r.db.WithContext(ctx).Model(&AdminUserRecord{}).Where("admin_user_id = ?", adminUserID).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

// RevokeSessionByToken 根据 token 注销会话。
func (r *AdminRepository) RevokeSessionByToken(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Model(&AdminSessionRecord{}).Where("session_token = ?", token).Updates(map[string]any{
		"session_status": 2,
		"updated_at":     time.Now(),
	}).Error
}
