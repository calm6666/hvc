package mysql

import (
	"context"
	"time"
)

// SetEnabled 切换配置中心绑定启用状态。
func (r *ConfigCenterBindingRepository) SetEnabled(ctx context.Context, bindingID uint64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&ConfigCenterBindingRecord{}).
		Where("binding_id = ?", bindingID).
		Updates(map[string]any{"enabled": enabled, "updated_at": time.Now()}).Error
}
