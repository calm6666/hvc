// Package config 提供配置领域对象。
//
// Effective 表示当前生效的运行配置快照。
// 配置系统采用"草稿/发布"双态模型：
//   - 草稿状态：管理员可修改，不影响线上运行
//   - 发布状态：发布后成为当前生效配置，立即热更新到所有节点
//
// 配置优先级（从高到低）：
//  1. 任务级覆盖参数（t_transcode_job_request_override）
//  2. 当前生效配置快照（Effective）
//  3. 代码内置默认值
package config

import "time"

// Effective 表示当前生效的运行配置快照。
type Effective struct {
	Version       int64
	PublishedAt   time.Time
	PublishedBy   string
	ConfigHash    string
	ConfigJSON    string
}

// IsValid 判断配置快照是否有效。
//
// 有效条件：
//  1. 版本号 > 0
//  2. 配置内容不为空
func (e *Effective) IsValid() bool {
	return e.Version > 0 && e.ConfigJSON != ""
}

// IsStale 判断配置快照是否过期。
//
// 如果配置快照的发布时间早于指定时间，则认为已过期。
func (e *Effective) IsStale(maxAge time.Duration) bool {
	if e.PublishedAt.IsZero() {
		return true
	}
	return time.Since(e.PublishedAt) > maxAge
}
