package mysql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// VerifyUserPassword 校验管理员密码。
//
// 当前项目初始化 SQL 使用 SHA2('admin123', 256) 生成默认管理员密码，
// 因此第一阶段先用同样的最小兼容逻辑完成后台登录链。
func (r *AdminRepository) VerifyUserPassword(ctx context.Context, username, plainPassword string) (AdminUserRecord, bool) {
	record, ok := r.FindUserByUsername(ctx, username)
	if !ok {
		return AdminUserRecord{}, false
	}
	sum := sha256.Sum256([]byte(plainPassword))
	if hex.EncodeToString(sum[:]) != record.PasswordHash {
		return AdminUserRecord{}, false
	}
	return record, true
}
