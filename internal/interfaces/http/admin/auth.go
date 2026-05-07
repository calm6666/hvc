package admin

import (
	"encoding/json"
	"net/http"
	"time"

	internalauth "hvc/internal/auth"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	authusecase "hvc/internal/usecase/auth"
	"hvc/pkg/logx"
)

// AuthHandler 处理后台认证接口。
type AuthHandler struct {
	loginUseCase    *authusecase.LoginUseCase
	adminRepository *mysql.AdminRepository
}

// NewAuthHandler 创建后台认证处理器。
func NewAuthHandler(loginUseCase *authusecase.LoginUseCase, adminRepository *mysql.AdminRepository) *AuthHandler {
	return &AuthHandler{loginUseCase: loginUseCase, adminRepository: adminRepository}
}

// Login 处理管理员登录。
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid form"})
		return
	}
	token, ok := h.loginUseCase.Execute(r)
	if !ok {
		logx.WriteJSON(w, http.StatusUnauthorized, model.Response{Code: 401, Message: "login failed"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"session_token": token}})
}

// WhoAmI 返回当前管理员会话概要。
func (h *AuthHandler) WhoAmI(w http.ResponseWriter, r *http.Request) {
	userID := internalauth.CurrentAdminUserID(r.Context(), r)
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"authenticated": userID != 0, "admin_user_id": userID}})
}

// Logout 处理管理员登出。
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		if cookie, err := r.Cookie("admin_session"); err == nil {
			token = cookie.Value
		}
	}
	if token != "" && h.adminRepository != nil {
		_ = h.adminRepository.RevokeSessionByToken(r.Context(), token)
	}
	http.SetCookie(w, &http.Cookie{Name: "admin_session", Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

var _ = json.NewDecoder
