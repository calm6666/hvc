// Package live 提供直播频道服务。
//
// ChannelService 管理直播频道的创建、启停和播放信息查询。
// 播放域名从配置中读取，不再硬编码。
package live

import (
	"hvc/internal/config"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"sync"
	"time"
)

// ChannelService 表示直播频道服务。
type ChannelService struct {
	mu          sync.RWMutex
	channels    map[string]model.LiveChannel
	sessions    map[string]model.LivePlaybackInfo
	playDomain  string
	flvDomain   string
}

// NewChannelService 创建直播频道服务。
//
// 参数：
//   - cfg: 动态运行配置，从中读取播放域名
func NewChannelService(cfg config.DynamicRuntimeConfig) *ChannelService {
	playDomain := cfg.Storage.PlayDomain
	if playDomain == "" {
		playDomain = cfg.Storage.BasePrefix
	}
	if playDomain == "" {
		playDomain = "http://localhost:8080"
	}
	flvDomain := cfg.Storage.FLVDomain
	if flvDomain == "" {
		flvDomain = playDomain
	}
	return &ChannelService{
		channels:   make(map[string]model.LiveChannel),
		sessions:   make(map[string]model.LivePlaybackInfo),
		playDomain: playDomain,
		flvDomain:  flvDomain,
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
		Status:                model.LiveChannelStatusIdle,
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
		MasterHLSURL:   s.playDomain + "/hls/" + channelKey + "/master.m3u8",
		HTTPFLVURL:     s.flvDomain + "/flv/" + channelKey + ".flv",
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
		Status:         "IDLE",
		MasterHLSURL:   s.playDomain + "/hls/" + channelKey + "/master.m3u8",
		HTTPFLVURL:     s.flvDomain + "/flv/" + channelKey + ".flv",
		RenditionNames: []string{"source", "720p", "480p"},
	}
}
