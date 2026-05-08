package auth

import (
	"net/http"
	"time"

	internalauth "hvc/internal/auth"
)

// LoginUseCase 表示登录用例。
type LoginUseCase struct{}

// Execute 执行登录。
func (u *LoginUseCase) Execute(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	repo := internalauth.AdminRepository()
	if repo == nil {
		return "", false
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		return "", false
	}
	user, ok := repo.VerifyUserPassword(r.Context(), username, password)
	if !ok || user.Status != 1 {
		return "", false
	}
	// 后台会话令牌必须使用不可预测的高强度随机值，
	// 不能再依赖“用户名 + 时间戳”这类可推测格式。
	token := internalauth.GenerateSecureToken(32)
	if token == "" {
		return "", false
	}
	session, err := repo.CreateSession(r.Context(), user.AdminUserID, token, r.RemoteAddr, r.UserAgent(), time.Now().Add(24*time.Hour))
	if err != nil {
		return "", false
	}
	return session.SessionToken, true
}
