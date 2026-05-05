package live

import (
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"sync"
	"time"
)

// ChannelService 表示直播频道服务。
type ChannelService struct {
	mu       sync.RWMutex
	channels map[string]model.LiveChannel
	sessions map[string]model.LivePlaybackInfo
}

// NewChannelService 创建直播频道服务。
func NewChannelService() *ChannelService {
	return &ChannelService{
		channels: make(map[string]model.LiveChannel),
		sessions: make(map[string]model.LivePlaybackInfo),
	}
}

// CreateChannel 创建直播频道。
func (s *ChannelService) CreateChannel(channelKey string, channelName string, profileID uint64) model.LiveChannel {
	now := time.Now()
	channel := model.LiveChannel{
		ChannelID:             idgen.Next(),
		ChannelKey:            channelKey,
		ChannelName:           channelName,
		ProfileID:             profileID,
		Status:                1,
		EnableSourceRendition: true,
		EnableWatermark:       false,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[channelKey] = channel
	return channel
}

// StartChannel 启动直播频道。
func (s *ChannelService) StartChannel(channelKey string) model.LivePlaybackInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	playback := model.LivePlaybackInfo{
		ChannelKey:     channelKey,
		Status:         "RUNNING",
		MasterHLSURL:   "https://live.example.com/hls/" + channelKey + "/master.m3u8",
		HTTPFLVURL:     "https://live.example.com/flv/" + channelKey + ".flv",
		RenditionNames: []string{"source", "720p", "480p"},
	}
	s.sessions[channelKey] = playback
	return playback
}

// StopChannel 停止直播频道。
func (s *ChannelService) StopChannel(channelKey string) model.LivePlaybackInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	playback := s.sessions[channelKey]
	playback.Status = "STOPPED"
	s.sessions[channelKey] = playback
	return playback
}

// GetPlaybackInfo 查询播放信息。
func (s *ChannelService) GetPlaybackInfo(channelKey string) model.LivePlaybackInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if playback, ok := s.sessions[channelKey]; ok {
		return playback
	}
	return model.LivePlaybackInfo{
		ChannelKey:     channelKey,
		Status:         "RUNNING",
		MasterHLSURL:   "https://live.example.com/hls/" + channelKey + "/master.m3u8",
		HTTPFLVURL:     "https://live.example.com/flv/" + channelKey + ".flv",
		RenditionNames: []string{"source", "720p", "480p"},
	}
}
