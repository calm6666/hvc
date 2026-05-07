package live

import (
	"context"
	"strings"
	"sync"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
)

type ChannelService struct {
	currentConfig       func() config.DynamicRuntimeConfig
	channelRepository   *mysql.LiveChannelRepository
	sessionRepository   *mysql.LiveSessionRepository
	renditionRepository *mysql.LiveProfileRenditionRepository
	mu                  sync.RWMutex
	fallbackChannels    map[string]model.LiveChannel
}

func NewChannelService(cfg config.DynamicRuntimeConfig, channelRepository *mysql.LiveChannelRepository, sessionRepository *mysql.LiveSessionRepository, renditionRepository *mysql.LiveProfileRenditionRepository) *ChannelService {
	return &ChannelService{
		currentConfig:       func() config.DynamicRuntimeConfig { return cfg },
		channelRepository:   channelRepository,
		sessionRepository:   sessionRepository,
		renditionRepository: renditionRepository,
		fallbackChannels:    make(map[string]model.LiveChannel),
	}
}

func (s *ChannelService) SetConfigSnapshot(snapshot func() config.DynamicRuntimeConfig) {
	if snapshot != nil {
		s.currentConfig = snapshot
	}
}

func (s *ChannelService) CreateChannel(channelKey string, channelName string, profileID uint64) model.LiveChannel {
	channel, _ := s.CreateChannelContext(context.Background(), channelKey, channelName, profileID)
	return channel
}

func (s *ChannelService) CreateChannelContext(ctx context.Context, channelKey string, channelName string, profileID uint64) (model.LiveChannel, error) {
	channelKey = strings.TrimSpace(channelKey)
	if channelKey == "" {
		return model.LiveChannel{}, nil
	}
	if s.channelRepository != nil {
		if channel, ok := s.channelRepository.FindByKey(ctx, channelKey); ok {
			return channel, nil
		}
	}

	now := time.Now()
	cfg := s.snapshot()
	channel := model.LiveChannel{
		ChannelID:             idgen.Next(),
		ChannelKey:            channelKey,
		ChannelName:           channelName,
		ProfileID:             profileID,
		Status:                model.LiveChannelStatusIdle,
		EnableSourceRendition: true,
		EnableWatermark:       false,
		PlayDomain:            cfg.Storage.PlayDomain,
		PushDomain:            cfg.Storage.BasePrefix,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if s.channelRepository != nil {
		if err := s.channelRepository.Create(ctx, channel); err != nil {
			return model.LiveChannel{}, err
		}
	} else {
		s.mu.Lock()
		s.fallbackChannels[channelKey] = channel
		s.mu.Unlock()
	}
	return channel, nil
}

func (s *ChannelService) UpdateChannelContext(ctx context.Context, patch model.LiveChannel) (model.LiveChannel, bool, error) {
	if patch.ChannelID == 0 {
		return model.LiveChannel{}, false, nil
	}
	channel, ok := s.GetChannelByIDContext(ctx, patch.ChannelID)
	if !ok {
		return model.LiveChannel{}, false, nil
	}
	if patch.ChannelName != "" {
		channel.ChannelName = patch.ChannelName
	}
	if patch.ProfileID != 0 {
		channel.ProfileID = patch.ProfileID
	}
	channel.EnableWatermark = patch.EnableWatermark
	if patch.PlayDomain != "" {
		channel.PlayDomain = patch.PlayDomain
	}
	if patch.PushDomain != "" {
		channel.PushDomain = patch.PushDomain
	}
	channel.UpdatedAt = time.Now()
	if s.channelRepository != nil {
		if err := s.channelRepository.Update(ctx, channel); err != nil {
			return model.LiveChannel{}, false, err
		}
	} else {
		s.mu.Lock()
		s.fallbackChannels[channel.ChannelKey] = channel
		s.mu.Unlock()
	}
	return channel, true, nil
}

func (s *ChannelService) GetChannelByIDContext(ctx context.Context, channelID uint64) (model.LiveChannel, bool) {
	if s.channelRepository == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, channel := range s.fallbackChannels {
			if channel.ChannelID == channelID {
				return channel, true
			}
		}
		return model.LiveChannel{}, false
	}
	return s.channelRepository.FindByID(ctx, channelID)
}

func (s *ChannelService) GetChannelByKeyContext(ctx context.Context, channelKey string) (model.LiveChannel, bool) {
	if s.channelRepository == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		channel, ok := s.fallbackChannels[channelKey]
		return channel, ok
	}
	return s.channelRepository.FindByKey(ctx, channelKey)
}

func (s *ChannelService) StartChannel(channelKey string) model.LivePlaybackInfo {
	ctx := context.Background()
	channel, ok := s.GetChannelByKeyContext(ctx, channelKey)
	if ok && s.channelRepository != nil {
		_ = s.channelRepository.UpdateStatus(ctx, channel.ChannelID, model.LiveChannelStatusLive, channel.AssignedNodeID, channel.AssignedWorkerID)
	} else if ok {
		channel.Status = model.LiveChannelStatusLive
		s.mu.Lock()
		s.fallbackChannels[channel.ChannelKey] = channel
		s.mu.Unlock()
	}
	return s.GetPlaybackInfoContext(ctx, channelKey)
}

func (s *ChannelService) StopChannel(channelKey string) model.LivePlaybackInfo {
	ctx := context.Background()
	channel, ok := s.GetChannelByKeyContext(ctx, channelKey)
	if ok && s.channelRepository != nil {
		_ = s.channelRepository.UpdateStatus(ctx, channel.ChannelID, model.LiveChannelStatusStopped, 0, "")
	} else if ok {
		channel.Status = model.LiveChannelStatusStopped
		s.mu.Lock()
		s.fallbackChannels[channel.ChannelKey] = channel
		s.mu.Unlock()
	}
	return s.GetPlaybackInfoContext(ctx, channelKey)
}

func (s *ChannelService) GetPlaybackInfo(channelKey string) model.LivePlaybackInfo {
	return s.GetPlaybackInfoContext(context.Background(), channelKey)
}

func (s *ChannelService) GetPlaybackInfoContext(ctx context.Context, channelKey string) model.LivePlaybackInfo {
	channel, _ := s.GetChannelByKeyContext(ctx, channelKey)
	playDomain, flvDomain := s.resolveDomains(channel)
	status := "IDLE"
	renditionNames := s.defaultRenditionNames()

	if channel.ProfileID != 0 && s.renditionRepository != nil {
		if items := s.renditionRepository.ListEnabledNamesByProfileID(ctx, channel.ProfileID); len(items) > 0 {
			renditionNames = items
		}
	}

	if channel.ChannelID != 0 {
		switch channel.Status {
		case model.LiveChannelStatusStarting, model.LiveChannelStatusLive:
			status = "RUNNING"
		case model.LiveChannelStatusStopped, model.LiveChannelStatusError:
			status = "STOPPED"
		}
	}
	if s.sessionRepository != nil && channel.ChannelID != 0 {
		if session, ok := s.sessionRepository.FindLatestActiveByChannelID(ctx, channel.ChannelID); ok {
			switch session.Status {
			case model.LiveSessionStatusPublishing, model.LiveSessionStatusResumed, model.LiveSessionStatusInterruptWaitResume:
				status = "RUNNING"
			}
		}
	}

	return model.LivePlaybackInfo{
		ChannelKey:     channelKey,
		Status:         status,
		MasterHLSURL:   playDomain + "/hls/" + channelKey + "/master.m3u8",
		HTTPFLVURL:     flvDomain + "/flv/" + channelKey + ".flv",
		RenditionNames: renditionNames,
	}
}

func (s *ChannelService) snapshot() config.DynamicRuntimeConfig {
	if s.currentConfig == nil {
		return config.DynamicRuntimeConfig{}
	}
	return s.currentConfig()
}

func (s *ChannelService) resolveDomains(channel model.LiveChannel) (string, string) {
	cfg := s.snapshot()
	playDomain := strings.TrimSpace(channel.PlayDomain)
	if playDomain == "" {
		playDomain = cfg.Storage.PlayDomain
	}
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
	return playDomain, flvDomain
}

func (s *ChannelService) defaultRenditionNames() []string {
	return []string{"source", "720p", "480p"}
}
