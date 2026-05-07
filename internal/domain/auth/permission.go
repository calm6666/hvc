// Package auth 提供权限领域对象。
//
// Permission 表示系统中的一个权限点，采用 "模块.资源.动作" 命名规范。
// 例如：transcode.job.create、cluster.node.quarantine、live.channel.start
//
// 权限体系设计：
//   - 每个权限点对应一个具体的操作能力
//   - 权限通过角色间接授予用户
//   - 支持权限的层级继承（如 transcode.job.* 包含所有 job 操作）
package auth

import "strings"

// Permission 表示权限领域对象。
type Permission struct {
	ID          uint64
	Code        string
	Description string
	Module      string
	Resource    string
	Action      string
}

// ParseCode 从权限编码解析出模块、资源和动作。
//
// 权限编码格式：模块.资源.动作
// 例如：transcode.job.create → Module=transcode, Resource=job, Action=create
func (p *Permission) ParseCode() {
	parts := strings.SplitN(p.Code, ".", 3)
	if len(parts) >= 1 {
		p.Module = parts[0]
	}
	if len(parts) >= 2 {
		p.Resource = parts[1]
	}
	if len(parts) >= 3 {
		p.Action = parts[2]
	}
}

// IsWildcard 判断权限是否为通配符权限。
//
// 通配符权限可以匹配该层级下的所有操作：
//   - transcode.* 匹配 transcode 模块下所有操作
//   - transcode.job.* 匹配 transcode.job 下所有操作
func (p *Permission) IsWildcard() bool {
	return strings.HasSuffix(p.Code, ".*")
}

// Matches 判断当前权限是否能匹配目标权限编码。
//
// 通配符匹配规则：
//   - transcode.* 匹配 transcode.job.create
//   - transcode.job.* 匹配 transcode.job.create
//   - transcode.job.create 只匹配 transcode.job.create
func (p *Permission) Matches(targetCode string) bool {
	if p.Code == targetCode {
		return true
	}
	if p.IsWildcard() {
		prefix := strings.TrimSuffix(p.Code, "*")
		return strings.HasPrefix(targetCode, prefix)
	}
	return false
}

// PermissionSet 表示权限集合，用于快速判断权限。
type PermissionSet struct {
	permissions map[string]*Permission
}

// NewPermissionSet 创建权限集合。
func NewPermissionSet(permissions []*Permission) *PermissionSet {
	ps := &PermissionSet{
		permissions: make(map[string]*Permission, len(permissions)),
	}
	for _, p := range permissions {
		ps.permissions[p.Code] = p
	}
	return ps
}

// Has 判断权限集合中是否包含指定权限。
//
// 支持通配符匹配：如果集合中有 transcode.*，
// 则 Has("transcode.job.create") 返回 true。
func (ps *PermissionSet) Has(code string) bool {
	if _, ok := ps.permissions[code]; ok {
		return true
	}
	for _, p := range ps.permissions {
		if p.Matches(code) {
			return true
		}
	}
	return false
}

// Add 向权限集合中添加权限。
func (ps *PermissionSet) Add(p *Permission) {
	ps.permissions[p.Code] = p
}

// Remove 从权限集合中移除权限。
func (ps *PermissionSet) Remove(code string) {
	delete(ps.permissions, code)
}

// Codes 返回所有权限编码。
func (ps *PermissionSet) Codes() []string {
	codes := make([]string, 0, len(ps.permissions))
	for code := range ps.permissions {
		codes = append(codes, code)
	}
	return codes
}
