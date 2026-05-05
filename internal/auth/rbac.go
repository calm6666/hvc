package auth

import "hvc/pkg/idgen"

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

// NewUser 创建后台用户。
func NewUser(username string) User {
	return User{UserID: idgen.Next(), Username: username, Status: 1}
}

// NewRole 创建后台角色。
func NewRole(roleKey string, roleName string) Role {
	return Role{RoleID: idgen.Next(), RoleKey: roleKey, RoleName: roleName}
}

// NewPermission 创建权限点。
func NewPermission(permKey string, permName string) Permission {
	return Permission{PermID: idgen.Next(), PermKey: permKey, PermName: permName}
}
