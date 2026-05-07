// Package live 提供直播频道和会话的存储实现。
//
// 本文件提供 ChannelStore 和 SessionStore 接口的默认实现，
// 使用 MySQL 作为持久化后端。当数据库不可用时，
// 退化为内存存储，保证服务可用性。
package live

import (
	"context"
	"sync"
	"time"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

// memoryChannelStore 基于 内存 的频道存储实现。
// 当数据库不可用时作为降级方案使用。
type memoryChannelStore struct {
	mu       sync.RWMutex
	channels map[uint64]*model.LiveChannel
}

// NewMemoryChannelStore 创建内存频道存储。
func NewMemoryChannelStore() ChannelStore {
	return &memoryChannelStore{
		channels: make(map[uint64]*model.LiveChannel),
	}
}

// GetByID 根据 ID 获取频道信息。
func (s *memoryChannelStore) GetByID(ctx context.Context, channelID uint64) (*model.LiveChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ch, ok := s.channels[channelID]; ok {
		return ch, nil
	}
	return nil, nil
}

// UpdateStatus 更新频道状态。
func (s *memoryChannelStore) UpdateStatus(ctx context.Context, channelID uint64, status string, nodeID uint64, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.channels[channelID]; ok {
		ch.Status = status
		ch.AssignedNodeID = nodeID
		ch.AssignedWorkerID = workerID
		ch.UpdatedAt = time.Now()
	}
	return nil
}

// memorySessionStore 基于内存的会话存储实现。
type memorySessionStore struct {
	mu       sync.RWMutex
	sessions map[uint64]*model.LiveSession
}

// NewMemorySessionStore 创建内存会话存储。
func NewMemorySessionStore() SessionStore {
	return &memorySessionStore{
		sessions: make(map[uint64]*model.LiveSession),
	}
}

// Save 保存推流会话。
func (s *memorySessionStore) Save(ctx context.Context, session *model.LiveSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SessionID] = session
	logx.Info("live.session_store.save", logx.Fields{
		"session_id":  session.SessionID,
		"channel_key": session.ChannelKey,
	})
	return nil
}

// UpdateStatus 更新会话状态。
func (s *memorySessionStore) UpdateStatus(ctx context.Context, sessionID uint64, status string, stoppedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		session.Status = status
		session.StoppedAt = &stoppedAt
	}
	return nil
}
