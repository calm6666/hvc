package auth

import "hvc/internal/infra/db/mysql"

// User 表示后台用户。
type User struct {
	UserID   uint64
	Username string
	Status   int
}

// Role 表示后台角色。
type Role struct {
	RoleID   uint64
	RoleKey  string
	RoleName string
}

// Permission 表示权限点。
type Permission struct {
	PermID   uint64
	PermKey  string
	PermName string
}

// AdminRepository 返回当前注入的管理员仓储。
func AdminRepository() *mysql.AdminRepository {
	authMu.RLock()
	defer authMu.RUnlock()
	return adminRepository
}
