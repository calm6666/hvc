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
	token := "admin-session-" + username + "-" + time.Now().Format("20060102150405")
	session, err := repo.CreateSession(r.Context(), user.AdminUserID, token, r.RemoteAddr, r.UserAgent(), time.Now().Add(24*time.Hour))
	if err != nil {
		return "", false
	}
	return session.SessionToken, true
}
