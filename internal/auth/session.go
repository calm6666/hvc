package auth

import "net/http"

// RequireAdminSession 校验后台会话。
func RequireAdminSession(r *http.Request) bool {
	token := r.Header.Get("Authorization")
	if token != "" {
		return true
	}
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return false
	}
	return cookie.Value != ""
}
