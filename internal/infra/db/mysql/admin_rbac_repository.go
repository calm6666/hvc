package mysql

import (
	"context"
	"time"

	"hvc/pkg/idgen"
)

// AdminRBACRepository 表示 RBAC 仓储。
//
// 这层负责维护角色、权限以及用户-角色、角色-权限两类绑定关系，
// 是第一阶段后台权限体系真正可用的基础数据入口。
type AdminRBACRepository struct {
	db *DB
}

// NewAdminRBACRepository 创建 RBAC 仓储。
func NewAdminRBACRepository(db *DB) *AdminRBACRepository {
	return &AdminRBACRepository{db: db}
}

// EnsureRole 保存或更新角色。
func (r *AdminRBACRepository) EnsureRole(ctx context.Context, roleKey, roleName, roleDesc string, status int) (AdminRoleRecord, error) {
	now := time.Now()
	var record AdminRoleRecord
	result := r.db.WithContext(ctx).Where("role_key = ?", roleKey).Limit(1).Find(&record)
	if result.Error != nil {
		return AdminRoleRecord{}, result.Error
	}
	if result.RowsAffected > 0 {
		record.RoleName = roleName
		record.RoleDesc = roleDesc
		record.Status = status
		record.UpdatedAt = now
		return record, r.db.WithContext(ctx).Save(&record).Error
	}
	record = AdminRoleRecord{
		RoleID:    idgen.Next(),
		RoleKey:   roleKey,
		RoleName:  roleName,
		RoleDesc:  roleDesc,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return record, r.db.WithContext(ctx).Create(&record).Error
}

// EnsurePermission 保存或更新权限点。
func (r *AdminRBACRepository) EnsurePermission(ctx context.Context, permKey, permName, permDesc, module string) (AdminPermissionRecord, error) {
	now := time.Now()
	var record AdminPermissionRecord
	result := r.db.WithContext(ctx).Where("perm_key = ?", permKey).Limit(1).Find(&record)
	if result.Error != nil {
		return AdminPermissionRecord{}, result.Error
	}
	if result.RowsAffected > 0 {
		record.PermName = permName
		record.PermDesc = permDesc
		record.Module = module
		return record, r.db.WithContext(ctx).Save(&record).Error
	}
	record = AdminPermissionRecord{
		PermID:    idgen.Next(),
		PermKey:   permKey,
		PermName:  permName,
		PermDesc:  permDesc,
		Module:    module,
		CreatedAt: now,
	}
	return record, r.db.WithContext(ctx).Create(&record).Error
}

// BindUserRole 绑定管理员用户与角色。
func (r *AdminRBACRepository) BindUserRole(ctx context.Context, userID, roleID uint64) error {
	var record AdminUserRoleRecord
	result := r.db.WithContext(ctx).Where("user_id = ? AND role_id = ?", userID, roleID).Limit(1).Find(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&AdminUserRoleRecord{
		ID:        idgen.Next(),
		UserID:    userID,
		RoleID:    roleID,
		CreatedAt: time.Now(),
	}).Error
}

// BindRolePermission 绑定角色与权限点。
func (r *AdminRBACRepository) BindRolePermission(ctx context.Context, roleID, permID uint64) error {
	var record AdminRolePermissionRecord
	result := r.db.WithContext(ctx).Where("role_id = ? AND perm_id = ?", roleID, permID).Limit(1).Find(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&AdminRolePermissionRecord{
		ID:        idgen.Next(),
		RoleID:    roleID,
		PermID:    permID,
		CreatedAt: time.Now(),
	}).Error
}
