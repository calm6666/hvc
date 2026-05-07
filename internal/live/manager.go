// Package live 提供直播会话管理器。
//
// Manager 负责管理所有活跃的直播频道和推流会话，
// 包括频道的启动/停止、会话的状态机转换、断流恢复等。
//
// 支持集群模式和单机模式：
//   - 集群模式：频道可被调度到任意节点，会话状态通过 Redis 同步
//   - 单机模式：频道在本机执行，会话状态仅保存在内存
//
// 多 GPU 支持：
//   - 直播转码任务同样受 GPU 会话数限制
//   - 同一频道的不同清晰度可分配到不同 GPU
package live

import (
	"context"
	"sync"
	"time"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

// ChannelStore 频道存储接口，用于解耦具体仓储实现。
type ChannelStore interface {
	GetByID(ctx context.Context, channelID uint64) (*model.LiveChannel, error)
	UpdateStatus(ctx context.Context, channelID uint64, status string, nodeID uint64, workerID string) error
}

// SessionStore 会话存储接口，用于解耦具体仓储实现。
type SessionStore interface {
	Save(ctx context.Context, session *model.LiveSession) error
	UpdateStatus(ctx context.Context, sessionID uint64, status string, stoppedAt time.Time) error
}

// Manager 直播会话管理器。
type Manager struct {
	channelStore ChannelStore
	sessionStore SessionStore
	mu           sync.RWMutex
	channels     map[uint64]*model.LiveChannel
	sessions     map[uint64]*model.LiveSession
}

// NewManager 创建直播会话管理器。
func NewManager(channelStore ChannelStore, sessionStore SessionStore) *Manager {
	return &Manager{
		channelStore: channelStore,
		sessionStore: sessionStore,
		channels:     make(map[uint64]*model.LiveChannel),
		sessions:     make(map[uint64]*model.LiveSession),
	}
}

// StartChannel 启动直播频道。
func (m *Manager) StartChannel(ctx context.Context, channelID uint64, nodeID uint64, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, ok := m.channels[channelID]
	if !ok {
		if m.channelStore != nil {
			var err error
			channel, err = m.channelStore.GetByID(ctx, channelID)
			if err != nil {
				return err
			}
		}
		if channel == nil {
			channel = &model.LiveChannel{
				ChannelID: channelID,
				Status:    model.LiveChannelStatusIdle,
			}
		}
		m.channels[channelID] = channel
	}

	if channel.Status != model.LiveChannelStatusIdle && channel.Status != model.LiveChannelStatusStopped {
		logx.Info("live.manager.channel_already_active", logx.Fields{
			"channel_id": channelID,
			"status":     channel.Status,
		})
		return nil
	}

	channel.Status = model.LiveChannelStatusStarting
	channel.AssignedNodeID = nodeID
	channel.AssignedWorkerID = workerID

	if m.channelStore != nil {
		if err := m.channelStore.UpdateStatus(ctx, channelID, channel.Status, nodeID, workerID); err != nil {
			logx.Error("live.manager.start_channel_db_failed", err, logx.Fields{
				"channel_id": channelID,
			})
		}
	}

	logx.Info("live.manager.channel_started", logx.Fields{
		"channel_id": channelID,
		"node_id":    nodeID,
		"worker_id":  workerID,
	})
	return nil
}

// StopChannel 停止直播频道。
func (m *Manager) StopChannel(ctx context.Context, channelID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, ok := m.channels[channelID]
	if !ok {
		return nil
	}

	channel.Status = model.LiveChannelStatusStopped

	if m.channelStore != nil {
		if err := m.channelStore.UpdateStatus(ctx, channelID, channel.Status, 0, ""); err != nil {
			logx.Error("live.manager.stop_channel_db_failed", err, logx.Fields{
				"channel_id": channelID,
			})
		}
	}

	for sessionID, session := range m.sessions {
		if session.ChannelID == channelID {
			session.Status = model.LiveSessionStatusStopped
			now := time.Now()
			session.StoppedAt = &now
			delete(m.sessions, sessionID)
		}
	}

	logx.Info("live.manager.channel_stopped", logx.Fields{
		"channel_id": channelID,
	})
	return nil
}

// OnPublish 推流开始回调。
func (m *Manager) OnPublish(ctx context.Context, channelKey string, nodeID uint64, workerID string) (*model.LiveSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &model.LiveSession{
		ChannelKey:       channelKey,
		Status:           model.LiveSessionStatusPublishing,
		AssignedNodeID:   nodeID,
		AssignedWorkerID: workerID,
		StartedAt:        time.Now(),
	}

	if m.sessionStore != nil {
		if err := m.sessionStore.Save(ctx, session); err != nil {
			logx.Error("live.manager.on_publish_save_failed", err, logx.Fields{
				"channel_key": channelKey,
			})
		}
	}

	m.sessions[session.SessionID] = session

	logx.Info("live.manager.on_publish", logx.Fields{
		"channel_key": channelKey,
		"session_id":  session.SessionID,
		"node_id":     nodeID,
	})
	return session, nil
}

// OnUnpublish 推流结束回调。
func (m *Manager) OnUnpublish(ctx context.Context, channelKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for sessionID, session := range m.sessions {
		if session.ChannelKey == channelKey && session.Status == model.LiveSessionStatusPublishing {
			session.Status = model.LiveSessionStatusStopped
			now := time.Now()
			session.StoppedAt = &now

			if m.sessionStore != nil {
				if err := m.sessionStore.UpdateStatus(ctx, sessionID, session.Status, now); err != nil {
					logx.Error("live.manager.on_unpublish_update_failed", err, logx.Fields{
						"session_id": sessionID,
					})
				}
			}

			delete(m.sessions, sessionID)
			logx.Info("live.manager.on_unpublish", logx.Fields{
				"channel_key": channelKey,
				"session_id":  sessionID,
			})
			break
		}
	}
	return nil
}

// OnInterrupt 推流中断回调。
func (m *Manager) OnInterrupt(ctx context.Context, channelKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range m.sessions {
		if session.ChannelKey == channelKey && session.Status == model.LiveSessionStatusPublishing {
			session.Status = model.LiveSessionStatusInterruptWaitResume
			logx.Info("live.manager.on_interrupt", logx.Fields{
				"channel_key": channelKey,
				"session_id":  session.SessionID,
			})
			break
		}
	}
}

// GetChannel 获取频道信息。
func (m *Manager) GetChannel(ctx context.Context, channelID uint64) (*model.LiveChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	channel, ok := m.channels[channelID]
	return channel, ok
}

// GetActiveSession 获取频道的活跃会话。
func (m *Manager) GetActiveSession(ctx context.Context, channelKey string) (*model.LiveSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, session := range m.sessions {
		if session.ChannelKey == channelKey &&
			(session.Status == model.LiveSessionStatusPublishing ||
				session.Status == model.LiveSessionStatusInterruptWaitResume) {
			return session, true
		}
	}
	return nil, false
}

// ActiveChannelCount 返回活跃频道数。
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

// ActiveSessionCount 返回活跃会话数。
func (m *Manager) ActiveSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
