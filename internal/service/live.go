// Package service 提供业务服务层入口。
//
// 本文件提供直播频道服务的便捷访问入口，
// 实际实现委托给 service/live 子包。
package service

import (
	"hvc/internal/config"
	liveservice "hvc/internal/service/live"
	"hvc/internal/model"
)

// LiveChannelService 是对 service/live.ChannelService 的类型别名，
// 保持向后兼容。
type LiveChannelService = liveservice.ChannelService

// NewLiveChannelService 创建直播频道服务。
func NewLiveChannelService(cfg config.DynamicRuntimeConfig) *LiveChannelService {
	return liveservice.NewChannelService(cfg)
}

// CreateLiveChannel 创建直播频道的便捷函数。
func CreateLiveChannel(svc *LiveChannelService, channelKey string, channelName string, profileID uint64) model.LiveChannel {
	return svc.CreateChannel(channelKey, channelName, profileID)
}
