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
	return true
}
