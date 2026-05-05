package middleware

import (
	"net/http"

	"hvc/internal/auth"
	"hvc/pkg/logx"
)

// RequirePermission 创建后台权限中间件。
func RequirePermission(permissionKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequirePermission(r, permissionKey) {
			logx.Error("http.admin.permission.denied", nil, logx.Fields{
				"path":        r.URL.Path,
				"permission":  permissionKey,
			})
			logx.WriteJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "message": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
