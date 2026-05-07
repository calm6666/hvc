package auth

import "net/http"

// RequirePermission 校验权限。
func RequirePermission(r *http.Request, permissionKey string) bool {
	if !RequireAdminSession(r) {
		return false
	}
	if permissionKey == "" {
		return true
	}
	repo := AdminRepository()
	if repo == nil {
		return false
	}
	userID := CurrentAdminUserID(r.Context(), r)
	if userID == 0 {
		return false
	}
	permissionKeys := repo.ListPermissionKeysByUserID(r.Context(), userID)
	for _, key := range permissionKeys {
		if key == permissionKey {
			return true
		}
	}
	return false
}
