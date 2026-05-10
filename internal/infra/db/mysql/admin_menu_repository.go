package mysql

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"hvc/pkg/idgen"
)

// EnsureMenu 保存或更新后台菜单。
func (r *AdminRBACRepository) EnsureMenu(ctx context.Context, record AdminMenuRecord) (AdminMenuRecord, error) {
	now := time.Now()
	if record.MenuID == 0 {
		record.MenuID = idgen.Next()
	}
	record.MenuKey = strings.TrimSpace(record.MenuKey)
	record.MenuName = strings.TrimSpace(record.MenuName)
	record.RoutePath = strings.TrimSpace(record.RoutePath)
	record.ComponentName = strings.TrimSpace(record.ComponentName)
	record.IconName = strings.TrimSpace(record.IconName)
	record.MenuType = strings.TrimSpace(record.MenuType)
	record.PermissionKey = strings.TrimSpace(record.PermissionKey)

	var existing AdminMenuRecord
	result := r.db.WithContext(ctx).Where("menu_key = ?", record.MenuKey).Limit(1).Find(&existing)
	if result.Error != nil {
		return AdminMenuRecord{}, result.Error
	}
	if result.RowsAffected > 0 {
		record.MenuID = existing.MenuID
		record.CreatedAt = existing.CreatedAt
		record.UpdatedAt = now
		return record, r.db.WithContext(ctx).Model(&AdminMenuRecord{}).
			Where("menu_id = ?", existing.MenuID).
			Updates(map[string]any{
				"parent_id":      record.ParentID,
				"menu_name":      record.MenuName,
				"route_path":     record.RoutePath,
				"component_name": record.ComponentName,
				"icon_name":      record.IconName,
				"menu_type":      record.MenuType,
				"permission_key": record.PermissionKey,
				"sort_no":        record.SortNo,
				"hidden":         record.Hidden,
				"status":         record.Status,
				"updated_at":     now,
			}).Error
	}

	record.CreatedAt = now
	record.UpdatedAt = now
	return record, r.db.WithContext(ctx).Create(&record).Error
}

// ListMenus 返回全部菜单，按层级和排序号稳定排序。
func (r *AdminRBACRepository) ListMenus(ctx context.Context, enabledOnly bool) []AdminMenuRecord {
	query := r.db.WithContext(ctx).Model(&AdminMenuRecord{})
	if enabledOnly {
		query = query.Where("status = ?", 1)
	}
	var records []AdminMenuRecord
	if err := query.Order("parent_id asc, sort_no asc, menu_id asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListMenusByUserID 返回用户可见菜单集合。
func (r *AdminRepository) ListMenusByUserID(ctx context.Context, userID uint64) []AdminMenuRecord {
	if userID == 0 {
		return nil
	}
	var records []AdminMenuRecord
	if err := r.db.WithContext(ctx).Raw(`
SELECT DISTINCT
  m.menu_id, m.parent_id, m.menu_key, m.menu_name, m.route_path, m.component_name, m.icon_name,
  m.menu_type, m.permission_key, m.sort_no, m.hidden, m.status, m.created_at, m.updated_at
FROM t_admin_user_role ur
JOIN t_admin_role_menu rm ON rm.role_id = ur.role_id
JOIN t_admin_menu m ON m.menu_id = rm.menu_id
WHERE ur.user_id = ? AND m.status = 1
ORDER BY m.parent_id ASC, m.sort_no ASC, m.menu_id ASC
`, userID).Scan(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListMenuIDsByRoleID 返回角色当前已绑定的菜单 ID 列表。
func (r *AdminRBACRepository) ListMenuIDsByRoleID(ctx context.Context, roleID uint64) []uint64 {
	if roleID == 0 {
		return nil
	}
	rows, err := r.db.WithContext(ctx).Raw(`
SELECT menu_id
FROM t_admin_role_menu
WHERE role_id = ?
ORDER BY menu_id ASC
`, roleID).Rows()
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]uint64, 0, 16)
	for rows.Next() {
		var menuID uint64
		if err := rows.Scan(&menuID); err != nil {
			continue
		}
		items = append(items, menuID)
	}
	return items
}

// ReplaceRoleMenus 用新菜单集合覆盖角色菜单绑定。
func (r *AdminRBACRepository) ReplaceRoleMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	if roleID == 0 {
		return nil
	}
	uniqueMenuIDs := dedupeUint64(menuIDs)
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&AdminRoleMenuRecord{}).Error; err != nil {
			return err
		}
		for _, menuID := range uniqueMenuIDs {
			if menuID == 0 {
				continue
			}
			record := AdminRoleMenuRecord{
				ID:        idgen.Next(),
				RoleID:    roleID,
				MenuID:    menuID,
				CreatedAt: now,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteMenu 删除菜单及其角色绑定；若仍存在子菜单则拒绝删除。
func (r *AdminRBACRepository) DeleteMenu(ctx context.Context, menuID uint64) error {
	if menuID == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var childCount int64
		if err := tx.Model(&AdminMenuRecord{}).Where("parent_id = ?", menuID).Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return gorm.ErrInvalidData
		}
		if err := tx.Where("menu_id = ?", menuID).Delete(&AdminRoleMenuRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("menu_id = ?", menuID).Delete(&AdminMenuRecord{}).Error
	})
}

func dedupeUint64(items []uint64) []uint64 {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(items))
	result := make([]uint64, 0, len(items))
	for _, item := range items {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
