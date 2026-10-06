package admin

import (
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
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "登录表单格式无效"})
		return
	}
	token, ok := h.loginUseCase.Execute(r)
	if !ok {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 401, Message: "用户名或密码错误"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"authenticated": true,
		"token_transport": map[string]any{
			"type":        "cookie",
			"cookie_name": "admin_session",
			"http_only":   true,
		},
	}})
}

// WhoAmI 返回当前管理员会话概要。
func (h *AuthHandler) WhoAmI(w http.ResponseWriter, r *http.Request) {
	userID := internalauth.CurrentAdminUserID(r.Context(), r)
	data := map[string]any{
		"authenticated": userID != 0,
		"admin_user_id": userID,
	}
	if userID != 0 && h.adminRepository != nil {
		if user, ok := h.adminRepository.FindUserByID(r.Context(), userID); ok {
			data["user"] = toAdminUserView(user)
		}
		data["permission_keys"] = h.adminRepository.ListPermissionKeysByUserID(r.Context(), userID)
		menuTree := buildMenuTree(h.adminRepository.ListMenusByUserID(r.Context(), userID))
		data["tree"] = true
		data["items"] = menuTree
		data["menu_tree"] = menuTree
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: data})
}

// Logout 处理管理员登出。
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := internalauth.ExtractAdminSessionToken(r)
	if token != "" && h.adminRepository != nil {
		_ = h.adminRepository.RevokeSessionByToken(r.Context(), token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}
