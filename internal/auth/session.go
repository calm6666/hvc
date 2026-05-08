package auth

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"hvc/internal/infra/db/mysql"
)

var (
	adminRepository *mysql.AdminRepository
	adminRBACRepo   *mysql.AdminRBACRepository
	authMu          sync.RWMutex
)

// ConfigureAdminAuth 注入管理员鉴权依赖。
//
// 当前项目的 HTTP 中间件仍然走 package-level auth 入口，
// 因此这里先通过显式注入把数据库仓储接进来，优先把第一阶段后台链路打通。
func ConfigureAdminAuth(adminRepo *mysql.AdminRepository, rbacRepo *mysql.AdminRBACRepository) {
	authMu.Lock()
	defer authMu.Unlock()
	adminRepository = adminRepo
	adminRBACRepo = rbacRepo
}

// ExtractAdminSessionToken 从 Authorization 或 Cookie 中提取后台会话令牌。
//
// 约定：
// 1. 优先读取 Authorization: Bearer <token>；
// 2. 其次读取 admin_session Cookie；
// 3. Header 支持大小写不敏感的 bearer 前缀。
func ExtractAdminSessionToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if token != "" {
		lower := strings.ToLower(token)
		if strings.HasPrefix(lower, "bearer ") {
			return strings.TrimSpace(token[7:])
		}
		return token
	}
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

// RequireAdminSession 校验后台会话。
func RequireAdminSession(r *http.Request) bool {
	token := ExtractAdminSessionToken(r)
	if token == "" {
		return false
	}
	authMu.RLock()
	repo := adminRepository
	authMu.RUnlock()
	if repo == nil {
		return false
	}
	_, ok := repo.FindActiveSession(r.Context(), token)
	return ok
}

// CurrentAdminUserID 返回当前会话对应的管理员用户ID。
func CurrentAdminUserID(ctx context.Context, r *http.Request) uint64 {
	_ = ctx
	token := ExtractAdminSessionToken(r)
	if token == "" {
		return 0
	}
	authMu.RLock()
	repo := adminRepository
	authMu.RUnlock()
	if repo == nil {
		return 0
	}
	session, ok := repo.FindActiveSession(r.Context(), token)
	if !ok {
		return 0
	}
	_ = repo.TouchSession(r.Context(), session.SessionID)
	return session.AdminUserID
}
