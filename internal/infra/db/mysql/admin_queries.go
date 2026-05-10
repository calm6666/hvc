package mysql

import (
	"context"
	"strings"
)

// ListUsersPage 返回管理员用户分页列表。
func (r *AdminRepository) ListUsersPage(ctx context.Context, page, pageSize int, username string, status int) ([]AdminUserRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&AdminUserRecord{})
	if username = strings.TrimSpace(username); username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []AdminUserRecord
	if err := query.Order("admin_user_id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// ListRolesPage 返回角色分页列表。
func (r *AdminRBACRepository) ListRolesPage(ctx context.Context, page, pageSize int, roleKey string, status int) ([]AdminRoleRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&AdminRoleRecord{})
	if roleKey = strings.TrimSpace(roleKey); roleKey != "" {
		query = query.Where("role_key LIKE ?", "%"+roleKey+"%")
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []AdminRoleRecord
	if err := query.Order("role_id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// ListRolesAll 返回全部角色列表。
func (r *AdminRBACRepository) ListRolesAll(ctx context.Context) []AdminRoleRecord {
	var records []AdminRoleRecord
	if err := r.db.WithContext(ctx).Order("role_id asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListPermissionsPage 返回权限点分页列表。
func (r *AdminRBACRepository) ListPermissionsPage(ctx context.Context, page, pageSize int, module string) ([]AdminPermissionRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&AdminPermissionRecord{})
	if module = strings.TrimSpace(module); module != "" {
		query = query.Where("module = ?", module)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []AdminPermissionRecord
	if err := query.Order("perm_id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *AdminRBACRepository) ListPermissions(ctx context.Context) []AdminPermissionRecord {
	items, _, err := r.ListPermissionsPage(ctx, 1, 1000, "")
	if err != nil {
		return nil
	}
	return items
}

func normalizeAdminPage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
