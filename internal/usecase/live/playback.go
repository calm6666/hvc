package live

import (
	"context"
	"hvc/internal/model"
	livesvc "hvc/internal/service/live"
)

// PlaybackUseCase 表示直播播放信息用例。
type PlaybackUseCase struct {
	service *livesvc.ChannelService
}

// NewPlaybackUseCase 创建直播播放信息用例。
func NewPlaybackUseCase(service *livesvc.ChannelService) *PlaybackUseCase {
	return &PlaybackUseCase{service: service}
}

// Execute 查询直播播放信息。
func (u *PlaybackUseCase) Execute(ctx context.Context, channelKey string) model.LivePlaybackInfo {
	return u.service.GetPlaybackInfoContext(ctx, channelKey)
}
