package live

import (
	"context"
	"hvc/internal/model"
	livesvc "hvc/internal/service/live"
)

// CreateChannelUseCase 表示创建直播频道用例。
type CreateChannelUseCase struct {
	service *livesvc.ChannelService
}

// NewCreateChannelUseCase 创建创建直播频道用例。
func NewCreateChannelUseCase(service *livesvc.ChannelService) *CreateChannelUseCase {
	return &CreateChannelUseCase{service: service}
}

// Execute 创建直播频道。
func (u *CreateChannelUseCase) Execute(ctx context.Context, channelKey string, channelName string, profileID uint64) model.LiveChannel {
	channel, _ := u.service.CreateChannelContext(ctx, channelKey, channelName, profileID)
	return channel
}
