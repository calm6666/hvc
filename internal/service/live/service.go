package live

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	authlive "hvc/internal/live"
	"hvc/internal/model"
	"hvc/pkg/idgen"
)

type ChannelService struct {
	currentConfig       func() config.DynamicRuntimeConfig
	channelRepository   *mysql.LiveChannelRepository
	sessionRepository   *mysql.LiveSessionRepository
	renditionRepository *mysql.LiveProfileRenditionRepository
	playbackTokenWriter playbackTokenWriter
	publishAuthLogger   publishAuthLogger
	publishSessionStore publishSessionStore
	mu                  sync.RWMutex
	fallbackChannels    map[string]model.LiveChannel
}

type playbackTokenWriter interface {
	SaveIssuedToken(ctx context.Context, channelID uint64, userToken string, viewerID string, expireAt time.Time) error
}

type publishAuthLogger interface {
	SaveAttempt(ctx context.Context, channelID uint64, streamKey string, requestIP string, allowed bool, message string) error
}

type publishSessionStore interface {
	SaveRejected(ctx context.Context, channelID uint64, streamKey string, publishIP string) error
}

type ChannelListFilter struct {
	Page       int
	PageSize   int
	Status     string
	ChannelKey string
}

type SessionListFilter struct {
	Page       int
	PageSize   int
	ChannelID  uint64
	ChannelKey string
	Status     string
}

type ChannelPatch struct {
	ChannelID             uint64
	ChannelName           *string
	ProfileID             *uint64
	EnableSourceRendition *bool
	EnableWatermark       *bool
	PlayDomain            *string
	PushDomain            *string
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

// SetPlaybackTokenWriter 注入播放令牌持久化器。
func (s *ChannelService) SetPlaybackTokenWriter(writer playbackTokenWriter) {
	s.playbackTokenWriter = writer
}

// SetPublishAuthLogger 注入推流鉴权日志记录器。
func (s *ChannelService) SetPublishAuthLogger(logger publishAuthLogger) {
	s.publishAuthLogger = logger
}

// SetPublishSessionStore 注入推流会话仓储。
func (s *ChannelService) SetPublishSessionStore(store publishSessionStore) {
	s.publishSessionStore = store
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

func (s *ChannelService) UpdateChannelContext(ctx context.Context, patch ChannelPatch) (model.LiveChannel, bool, error) {
	if patch.ChannelID == 0 {
		return model.LiveChannel{}, false, nil
	}
	channel, ok := s.GetChannelByIDContext(ctx, patch.ChannelID)
	if !ok {
		return model.LiveChannel{}, false, nil
	}
	if patch.ChannelName != nil {
		channel.ChannelName = *patch.ChannelName
	}
	if patch.ProfileID != nil {
		channel.ProfileID = *patch.ProfileID
	}
	if patch.EnableSourceRendition != nil {
		channel.EnableSourceRendition = *patch.EnableSourceRendition
	}
	if patch.EnableWatermark != nil {
		channel.EnableWatermark = *patch.EnableWatermark
	}
	if patch.PlayDomain != nil {
		channel.PlayDomain = *patch.PlayDomain
	}
	if patch.PushDomain != nil {
		channel.PushDomain = *patch.PushDomain
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

func (s *ChannelService) CurrentAuthConfig() authlive.AuthConfig {
	return authlive.NewAuthConfig(s.snapshot())
}

func (s *ChannelService) DeleteChannelContext(ctx context.Context, channelID uint64) (bool, error) {
	if channelID == 0 {
		return false, nil
	}
	channel, ok := s.GetChannelByIDContext(ctx, channelID)
	if !ok {
		return false, nil
	}
	if s.sessionRepository != nil {
		if session, found := s.sessionRepository.FindLatestActiveByChannelID(ctx, channelID); found {
			switch session.Status {
			case model.LiveSessionStatusConnecting, model.LiveSessionStatusPublishing, model.LiveSessionStatusInterruptWaitResume, model.LiveSessionStatusResumed:
				return true, fmt.Errorf("channel has active session")
			}
		}
	}
	if channel.Status == model.LiveChannelStatusStarting || channel.Status == model.LiveChannelStatusLive {
		return true, fmt.Errorf("channel is running")
	}
	if s.channelRepository != nil {
		return true, s.channelRepository.Delete(ctx, channelID)
	}
	s.mu.Lock()
	delete(s.fallbackChannels, channel.ChannelKey)
	s.mu.Unlock()
	return true, nil
}

func (s *ChannelService) ListChannelsContext(ctx context.Context, filter ChannelListFilter) ([]model.LiveChannel, int64, error) {
	if s.channelRepository != nil {
		return s.channelRepository.ListPage(ctx, mysql.LiveChannelListFilter{
			Page:       filter.Page,
			PageSize:   filter.PageSize,
			Status:     filter.Status,
			ChannelKey: filter.ChannelKey,
		})
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]model.LiveChannel, 0, len(s.fallbackChannels))
	for _, channel := range s.fallbackChannels {
		if filter.ChannelKey != "" && channel.ChannelKey != filter.ChannelKey {
			continue
		}
		if filter.Status != "" && channel.Status != filter.Status {
			continue
		}
		items = append(items, channel)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []model.LiveChannel{}, int64(len(items)), nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], int64(len(items)), nil
}

func (s *ChannelService) ListSessionsContext(ctx context.Context, filter SessionListFilter) ([]model.LiveSession, int64, error) {
	channelID := filter.ChannelID
	if channelID == 0 && filter.ChannelKey != "" {
		channel, ok := s.GetChannelByKeyContext(ctx, filter.ChannelKey)
		if !ok {
			return []model.LiveSession{}, 0, nil
		}
		channelID = channel.ChannelID
	}

	if s.sessionRepository == nil {
		return []model.LiveSession{}, 0, nil
	}

	items, total, err := s.sessionRepository.ListPage(ctx, mysql.LiveSessionListFilter{
		Page:      filter.Page,
		PageSize:  filter.PageSize,
		ChannelID: channelID,
		Status:    filter.Status,
	})
	if err != nil {
		return nil, 0, err
	}

	if filter.ChannelKey != "" {
		for i := range items {
			items[i].ChannelKey = filter.ChannelKey
		}
		return items, total, nil
	}
	for i := range items {
		channel, ok := s.GetChannelByIDContext(ctx, items[i].ChannelID)
		if ok {
			items[i].ChannelKey = channel.ChannelKey
		}
	}
	return items, total, nil
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
	playAuth := authlive.NewAuthConfig(s.snapshot())
	authValues := authlive.GeneratePlayAuthValues(playAuth, channelKey)
	playToken := authValues.Get("sign")
	expireAt, _ := strconv.ParseInt(authValues.Get("expire"), 10, 64)

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

	renditions := make([]model.LivePlaybackRendition, 0, len(renditionNames))
	for _, name := range renditionNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		hlsURL := appendQuery(fmt.Sprintf("%s/hls/%s/%s.m3u8", strings.TrimRight(playDomain, "/"), channelKey, name), authValues)
		flvName := channelKey + "_" + name
		if name == "source" {
			flvName = channelKey
		}
		flvURL := appendQuery(fmt.Sprintf("%s/flv/%s.flv", strings.TrimRight(flvDomain, "/"), flvName), authValues)
		renditions = append(renditions, model.LivePlaybackRendition{
			RenditionName: name,
			IsSource:      name == "source",
			HLSURL:        hlsURL,
			HTTPFLVURL:    flvURL,
		})
	}

	return model.LivePlaybackInfo{
		ChannelKey:     channelKey,
		Status:         status,
		PlayToken:      playToken,
		ExpireAt:       expireAt,
		MasterHLSURL:   appendQuery(fmt.Sprintf("%s/hls/%s/master.m3u8", strings.TrimRight(playDomain, "/"), channelKey), authValues),
		HTTPFLVURL:     appendQuery(fmt.Sprintf("%s/flv/%s.flv", strings.TrimRight(flvDomain, "/"), channelKey), authValues),
		RenditionNames: renditionNames,
		Renditions:     renditions,
	}
}

// RecordIssuedPlaybackToken 持久化一次已签发的播放令牌。
//
// 该方法不影响主流程返回结果；即使落库失败，也只作为审计链路失败处理。
func (s *ChannelService) RecordIssuedPlaybackToken(ctx context.Context, channelKey string, viewerID string, playToken string, expireAt int64) error {
	if s.playbackTokenWriter == nil || strings.TrimSpace(channelKey) == "" || strings.TrimSpace(playToken) == "" || expireAt <= 0 {
		return nil
	}
	channel, ok := s.GetChannelByKeyContext(ctx, channelKey)
	if !ok || channel.ChannelID == 0 {
		return nil
	}
	return s.playbackTokenWriter.SaveIssuedToken(ctx, channel.ChannelID, playToken, strings.TrimSpace(viewerID), time.Unix(expireAt, 0))
}

// RecordPublishAuthAttempt 持久化一次推流鉴权结果。
func (s *ChannelService) RecordPublishAuthAttempt(ctx context.Context, channelKey string, streamKey string, requestIP string, allowed bool, message string) error {
	if (s.publishAuthLogger == nil && s.publishSessionStore == nil) || strings.TrimSpace(channelKey) == "" {
		return nil
	}
	channel, ok := s.GetChannelByKeyContext(ctx, channelKey)
	if !ok || channel.ChannelID == 0 {
		return nil
	}
	if strings.TrimSpace(streamKey) == "" {
		streamKey = channelKey
	}
	streamKey = strings.TrimSpace(streamKey)
	requestIP = strings.TrimSpace(requestIP)
	message = strings.TrimSpace(message)

	if s.publishAuthLogger != nil {
		if err := s.publishAuthLogger.SaveAttempt(ctx, channel.ChannelID, streamKey, requestIP, allowed, message); err != nil {
			return err
		}
	}
	if !allowed && s.publishSessionStore != nil {
		if err := s.publishSessionStore.SaveRejected(ctx, channel.ChannelID, streamKey, requestIP); err != nil {
			return err
		}
	}
	return nil
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

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func appendQuery(rawURL string, values url.Values) string {
	if len(values) == 0 {
		return rawURL
	}
	return rawURL + "?" + values.Encode()
}
