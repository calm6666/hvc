package mysql

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// HashAdminPassword 对管理员明文密码做兼容旧数据的无盐哈希。
//
// 该函数只用于兼容历史种子数据或旧库中 password_salt 为空的管理员账号。
// 新创建/重置密码的管理员账号应优先使用带盐哈希。
func HashAdminPassword(plainPassword string) string {
	sum := sha256.Sum256([]byte(plainPassword))
	return hex.EncodeToString(sum[:])
}

// GenerateAdminPasswordSalt 生成管理员密码盐值。
func GenerateAdminPasswordSalt() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(raw[:])
}

// HashAdminPasswordWithSalt 对管理员密码做带盐哈希。
func HashAdminPasswordWithSalt(plainPassword string, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + plainPassword))
	return hex.EncodeToString(sum[:])
}

// VerifyUserPassword 校验管理员密码。
//
// 兼容策略：
// 1. 新账号或重置过密码的账号使用 password_salt + salted SHA-256；
// 2. 旧种子或历史存量账号如果 salt 为空，则继续走无盐 SHA-256 校验。
func (r *AdminRepository) VerifyUserPassword(ctx context.Context, username, plainPassword string) (AdminUserRecord, bool) {
	record, ok := r.FindUserByUsername(ctx, username)
	if !ok {
		return AdminUserRecord{}, false
	}
	expected := HashAdminPassword(plainPassword)
	if record.PasswordSalt != "" {
		expected = HashAdminPasswordWithSalt(plainPassword, record.PasswordSalt)
	}
	if expected != record.PasswordHash {
		return AdminUserRecord{}, false
	}
	return record, true
}
