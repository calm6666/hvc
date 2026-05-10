package middleware

import (
	"net/http"

	"hvc/internal/auth"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// RequireAdminSession 创建后台会话中间件。
func RequireAdminSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireAdminSession(r) {
			logx.WriteJSON(w, http.StatusUnauthorized, model.Response{Code: 401, Message: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePermission 创建后台权限中间件。
func RequirePermission(permissionKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequirePermission(r, permissionKey) {
			logx.Error("http.admin.permission.denied", nil, logx.Fields{
				"path":       r.URL.Path,
				"permission": permissionKey,
			})
			logx.WriteJSON(w, http.StatusUnauthorized, model.Response{Code: 401, Message: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
