package mysql

import "context"

// ListUsers 返回管理员用户列表。
func (r *AdminRepository) ListUsers(ctx context.Context) []AdminUserRecord {
	var records []AdminUserRecord
	if err := r.db.WithContext(ctx).Order("admin_user_id asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListRoles 返回角色列表。
func (r *AdminRBACRepository) ListRoles(ctx context.Context) []AdminRoleRecord {
	var records []AdminRoleRecord
	if err := r.db.WithContext(ctx).Order("role_id asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListPermissions 返回权限点列表。
func (r *AdminRBACRepository) ListPermissions(ctx context.Context) []AdminPermissionRecord {
	var records []AdminPermissionRecord
	if err := r.db.WithContext(ctx).Order("perm_id asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}
