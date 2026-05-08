package live

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

type ChannelStore interface {
	GetByID(ctx context.Context, channelID uint64) (*model.LiveChannel, error)
	GetByKey(ctx context.Context, channelKey string) (*model.LiveChannel, error)
	UpdateStatus(ctx context.Context, channelID uint64, status string, nodeID uint64, workerID string) error
}

type SessionStore interface {
	Save(ctx context.Context, session *model.LiveSession) error
	UpdateStatus(ctx context.Context, sessionID uint64, status string, stoppedAt time.Time) error
	FindLatestActiveByChannelID(ctx context.Context, channelID uint64) (*model.LiveSession, error)
}

type SessionEventWriter interface {
	SaveEvent(ctx context.Context, sessionID uint64, channelID uint64, eventType string, payload any) error
}

type PublishSessionWriter interface {
	SaveConnected(ctx context.Context, channelID uint64, sessionID uint64, streamKey string, publishIP string) error
	MarkDisconnectedLatest(ctx context.Context, channelID uint64, streamKey string) error
}

type Manager struct {
	channelStore  ChannelStore
	sessionStore  SessionStore
	eventWriter   SessionEventWriter
	publishWriter PublishSessionWriter
	mu            sync.RWMutex
	channels      map[uint64]*model.LiveChannel
	sessions      map[uint64]*model.LiveSession
}

func NewManager(channelStore ChannelStore, sessionStore SessionStore) *Manager {
	return &Manager{
		channelStore: channelStore,
		sessionStore: sessionStore,
		channels:     make(map[uint64]*model.LiveChannel),
		sessions:     make(map[uint64]*model.LiveSession),
	}
}

// SetSessionEventWriter 注入直播会话事件记录器。
func (m *Manager) SetSessionEventWriter(writer SessionEventWriter) {
	m.eventWriter = writer
}

// SetPublishSessionWriter 注入推流会话记录器。
func (m *Manager) SetPublishSessionWriter(writer PublishSessionWriter) {
	m.publishWriter = writer
}

func (m *Manager) StartChannel(ctx context.Context, channelID uint64, nodeID uint64, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, err := m.loadChannelByIDLocked(ctx, channelID)
	if err != nil {
		return err
	}
	if channel == nil {
		return fmt.Errorf("live channel %d not found", channelID)
	}
	if channel.Status == model.LiveChannelStatusStarting || channel.Status == model.LiveChannelStatusLive {
		return nil
	}

	channel.Status = model.LiveChannelStatusStarting
	channel.AssignedNodeID = nodeID
	channel.AssignedWorkerID = workerID
	channel.UpdatedAt = time.Now()
	m.channels[channelID] = channel

	if m.channelStore != nil {
		if err := m.channelStore.UpdateStatus(ctx, channelID, channel.Status, nodeID, workerID); err != nil {
			return err
		}
	}

	logx.Info("live.manager.channel_started", logx.Fields{
		"channel_id": channelID,
		"node_id":    nodeID,
		"worker_id":  workerID,
	})
	m.appendEvent(ctx, 0, channel.ChannelID, "live.channel.start_requested", map[string]any{
		"channel_key": channel.ChannelKey,
		"node_id":     nodeID,
		"worker_id":   workerID,
		"status":      channel.Status,
	})
	return nil
}

func (m *Manager) StopChannel(ctx context.Context, channelID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, err := m.loadChannelByIDLocked(ctx, channelID)
	if err != nil {
		return err
	}
	if channel == nil {
		return nil
	}

	channel.Status = model.LiveChannelStatusStopped
	channel.AssignedNodeID = 0
	channel.AssignedWorkerID = ""
	channel.UpdatedAt = time.Now()
	m.channels[channelID] = channel

	if m.channelStore != nil {
		if err := m.channelStore.UpdateStatus(ctx, channelID, channel.Status, 0, ""); err != nil {
			return err
		}
	}

	session, sessionID := m.findActiveSessionLocked(channelID)
	if session == nil && m.sessionStore != nil {
		session, err = m.sessionStore.FindLatestActiveByChannelID(ctx, channelID)
		if err != nil {
			return err
		}
		if session != nil {
			sessionID = session.SessionID
		}
	}
	if session != nil {
		session.Status = model.LiveSessionStatusStopped
		now := time.Now()
		session.StoppedAt = &now
		if m.sessionStore != nil {
			if err := m.sessionStore.UpdateStatus(ctx, sessionID, session.Status, now); err != nil {
				return err
			}
		}
		delete(m.sessions, sessionID)
	}

	logx.Info("live.manager.channel_stopped", logx.Fields{
		"channel_id": channelID,
	})
	m.appendEvent(ctx, sessionID, channel.ChannelID, "live.channel.stopped", map[string]any{
		"channel_key": channel.ChannelKey,
		"session_id":  sessionID,
		"status":      channel.Status,
	})
	return nil
}

func (m *Manager) OnPublish(ctx context.Context, channelKey string, nodeID uint64, workerID string, streamKey string, publishIP string, pushProtocol string, ingestURL string, playbackHLSURL string) (*model.LiveSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, err := m.loadChannelByKeyLocked(ctx, channelKey)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("live channel %s not found", channelKey)
	}

	if pushProtocol == "" {
		pushProtocol = "rtmp"
	}
	if streamKey == "" {
		streamKey = channelKey
	}

	session := &model.LiveSession{
		SessionID:        idgen.Next(),
		ChannelID:        channel.ChannelID,
		ChannelKey:       channelKey,
		SessionKey:       streamKey,
		Status:           model.LiveSessionStatusPublishing,
		IngestURL:        ingestURL,
		PlaybackHLSURL:   playbackHLSURL,
		PushProtocol:     pushProtocol,
		AssignedNodeID:   nodeID,
		AssignedWorkerID: workerID,
		StartedAt:        time.Now(),
	}
	if m.sessionStore != nil {
		if err := m.sessionStore.Save(ctx, session); err != nil {
			return nil, err
		}
	}
	m.sessions[session.SessionID] = session

	channel.Status = model.LiveChannelStatusLive
	channel.AssignedNodeID = nodeID
	channel.AssignedWorkerID = workerID
	channel.UpdatedAt = time.Now()
	m.channels[channel.ChannelID] = channel
	if m.channelStore != nil {
		if err := m.channelStore.UpdateStatus(ctx, channel.ChannelID, channel.Status, nodeID, workerID); err != nil {
			return nil, err
		}
	}

	logx.Info("live.manager.on_publish", logx.Fields{
		"channel_key": channelKey,
		"session_id":  session.SessionID,
		"node_id":     nodeID,
	})
	if m.publishWriter != nil {
		if err := m.publishWriter.SaveConnected(ctx, channel.ChannelID, session.SessionID, streamKey, publishIP); err != nil {
			return nil, err
		}
	}
	m.appendEvent(ctx, session.SessionID, channel.ChannelID, "live.publish.connected", map[string]any{
		"channel_key": channelKey,
		"stream_key":  streamKey,
		"publish_ip":  publishIP,
		"node_id":     nodeID,
		"worker_id":   workerID,
		"status":      session.Status,
	})
	return session, nil
}

func (m *Manager) OnUnpublish(ctx context.Context, channelKey string, streamKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, err := m.loadChannelByKeyLocked(ctx, channelKey)
	if err != nil {
		return err
	}
	if channel == nil {
		return nil
	}

	session, sessionID := m.findActiveSessionLocked(channel.ChannelID)
	if session == nil && m.sessionStore != nil {
		session, err = m.sessionStore.FindLatestActiveByChannelID(ctx, channel.ChannelID)
		if err != nil {
			return err
		}
		if session != nil {
			sessionID = session.SessionID
		}
	}
	if session != nil {
		session.Status = model.LiveSessionStatusStopped
		now := time.Now()
		session.StoppedAt = &now
		if m.sessionStore != nil {
			if err := m.sessionStore.UpdateStatus(ctx, sessionID, session.Status, now); err != nil {
				return err
			}
		}
		delete(m.sessions, sessionID)
	}

	channel.Status = model.LiveChannelStatusStopped
	channel.AssignedNodeID = 0
	channel.AssignedWorkerID = ""
	channel.UpdatedAt = time.Now()
	m.channels[channel.ChannelID] = channel
	if m.channelStore != nil {
		if err := m.channelStore.UpdateStatus(ctx, channel.ChannelID, channel.Status, 0, ""); err != nil {
			return err
		}
	}

	logx.Info("live.manager.on_unpublish", logx.Fields{
		"channel_key": channelKey,
		"session_id":  sessionID,
	})
	if m.publishWriter != nil {
		if err := m.publishWriter.MarkDisconnectedLatest(ctx, channel.ChannelID, streamKey); err != nil {
			return err
		}
	}
	m.appendEvent(ctx, sessionID, channel.ChannelID, "live.publish.disconnected", map[string]any{
		"channel_key": channelKey,
		"stream_key":  streamKey,
		"session_id":  sessionID,
		"status":      channel.Status,
	})
	return nil
}

func (m *Manager) OnInterrupt(ctx context.Context, channelKey string, streamKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, _ := m.loadChannelByKeyLocked(ctx, channelKey)
	if channel == nil {
		return
	}
	session, sessionID := m.findActiveSessionLocked(channel.ChannelID)
	if session == nil {
		if m.sessionStore != nil {
			session, _ = m.sessionStore.FindLatestActiveByChannelID(ctx, channel.ChannelID)
			if session != nil {
				sessionID = session.SessionID
			}
		}
	}
	if session != nil {
		session.Status = model.LiveSessionStatusInterruptWaitResume
		if m.sessionStore != nil {
			_ = m.sessionStore.UpdateStatus(ctx, sessionID, session.Status, time.Time{})
		}
		m.sessions[sessionID] = session
	}
	channel.Status = model.LiveChannelStatusError
	channel.UpdatedAt = time.Now()
	m.channels[channel.ChannelID] = channel
	if m.channelStore != nil {
		_ = m.channelStore.UpdateStatus(ctx, channel.ChannelID, channel.Status, channel.AssignedNodeID, channel.AssignedWorkerID)
	}
	logx.Info("live.manager.on_interrupt", logx.Fields{
		"channel_key": channelKey,
		"session_id":  sessionID,
	})
	if m.publishWriter != nil {
		_ = m.publishWriter.MarkDisconnectedLatest(ctx, channel.ChannelID, streamKey)
	}
	m.appendEvent(ctx, sessionID, channel.ChannelID, "live.stream.interrupted", map[string]any{
		"channel_key": channelKey,
		"stream_key":  streamKey,
		"session_id":  sessionID,
		"status":      channel.Status,
	})
}

func (m *Manager) GetChannel(ctx context.Context, channelID uint64) (*model.LiveChannel, bool) {
	m.mu.RLock()
	if channel, ok := m.channels[channelID]; ok {
		item := *channel
		m.mu.RUnlock()
		return &item, true
	}
	m.mu.RUnlock()

	if m.channelStore == nil {
		return nil, false
	}
	channel, err := m.channelStore.GetByID(ctx, channelID)
	if err != nil || channel == nil {
		return nil, false
	}
	return channel, true
}

func (m *Manager) GetActiveSession(ctx context.Context, channelKey string) (*model.LiveSession, bool) {
	m.mu.RLock()
	for _, session := range m.sessions {
		if session.ChannelKey == channelKey &&
			(session.Status == model.LiveSessionStatusPublishing ||
				session.Status == model.LiveSessionStatusInterruptWaitResume ||
				session.Status == model.LiveSessionStatusResumed) {
			item := *session
			m.mu.RUnlock()
			return &item, true
		}
	}
	m.mu.RUnlock()

	if m.channelStore == nil || m.sessionStore == nil {
		return nil, false
	}
	channel, err := m.channelStore.GetByKey(ctx, channelKey)
	if err != nil || channel == nil {
		return nil, false
	}
	session, err := m.sessionStore.FindLatestActiveByChannelID(ctx, channel.ChannelID)
	if err != nil || session == nil {
		return nil, false
	}
	session.ChannelKey = channelKey
	return session, true
}

func (m *Manager) ActiveChannelCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, ch := range m.channels {
		if ch.Status == model.LiveChannelStatusLive || ch.Status == model.LiveChannelStatusStarting {
			count++
		}
	}
	return count
}

func (m *Manager) ActiveSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *Manager) loadChannelByIDLocked(ctx context.Context, channelID uint64) (*model.LiveChannel, error) {
	if channel, ok := m.channels[channelID]; ok {
		return channel, nil
	}
	if m.channelStore == nil {
		return nil, nil
	}
	channel, err := m.channelStore.GetByID(ctx, channelID)
	if err != nil || channel == nil {
		return channel, err
	}
	m.channels[channelID] = channel
	return channel, nil
}

func (m *Manager) loadChannelByKeyLocked(ctx context.Context, channelKey string) (*model.LiveChannel, error) {
	for _, channel := range m.channels {
		if channel.ChannelKey == channelKey {
			return channel, nil
		}
	}
	if m.channelStore == nil {
		return nil, nil
	}
	channel, err := m.channelStore.GetByKey(ctx, channelKey)
	if err != nil || channel == nil {
		return channel, err
	}
	m.channels[channel.ChannelID] = channel
	return channel, nil
}

func (m *Manager) findActiveSessionLocked(channelID uint64) (*model.LiveSession, uint64) {
	for sessionID, session := range m.sessions {
		if session.ChannelID != channelID {
			continue
		}
		switch session.Status {
		case model.LiveSessionStatusPublishing, model.LiveSessionStatusInterruptWaitResume, model.LiveSessionStatusResumed:
			return session, sessionID
		}
	}
	return nil, 0
}

func (m *Manager) appendEvent(ctx context.Context, sessionID uint64, channelID uint64, eventType string, payload any) {
	// t_live_session_event 当前按“会话级事件”建模，session_id 为必填外键。
	// 因此在尚未创建真实直播会话前，不写入 start_requested 这类频道级事件，
	// 避免把 0 当成 session_id 落库后触发外键错误。
	if m.eventWriter == nil || sessionID == 0 || channelID == 0 || eventType == "" {
		return
	}
	if err := m.eventWriter.SaveEvent(ctx, sessionID, channelID, eventType, payload); err != nil {
		logx.Error("live.manager.append_event_failed", err, logx.Fields{
			"channel_id": channelID,
			"session_id": sessionID,
			"event_type": eventType,
		})
	}
}
