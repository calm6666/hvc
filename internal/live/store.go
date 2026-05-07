package live

import (
	"context"
	"sync"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

type memoryChannelStore struct {
	mu         sync.RWMutex
	channels   map[uint64]*model.LiveChannel
	channelKey map[string]uint64
}

func NewMemoryChannelStore() ChannelStore {
	return &memoryChannelStore{
		channels:   make(map[uint64]*model.LiveChannel),
		channelKey: make(map[string]uint64),
	}
}

func (s *memoryChannelStore) GetByID(ctx context.Context, channelID uint64) (*model.LiveChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ch, ok := s.channels[channelID]; ok {
		item := *ch
		return &item, nil
	}
	return nil, nil
}

func (s *memoryChannelStore) GetByKey(ctx context.Context, channelKey string) (*model.LiveChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channelID, ok := s.channelKey[channelKey]
	if !ok {
		return nil, nil
	}
	if ch, ok := s.channels[channelID]; ok {
		item := *ch
		return &item, nil
	}
	return nil, nil
}

func (s *memoryChannelStore) UpdateStatus(ctx context.Context, channelID uint64, status string, nodeID uint64, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.channels[channelID]; ok {
		ch.Status = status
		ch.AssignedNodeID = nodeID
		ch.AssignedWorkerID = workerID
		ch.UpdatedAt = time.Now()
		s.channelKey[ch.ChannelKey] = channelID
	}
	return nil
}

type repositoryChannelStore struct {
	repo *mysql.LiveChannelRepository
}

func NewRepositoryChannelStore(repo *mysql.LiveChannelRepository) ChannelStore {
	return &repositoryChannelStore{repo: repo}
}

func (s *repositoryChannelStore) GetByID(ctx context.Context, channelID uint64) (*model.LiveChannel, error) {
	if s.repo == nil {
		return nil, nil
	}
	channel, ok := s.repo.FindByID(ctx, channelID)
	if !ok {
		return nil, nil
	}
	return &channel, nil
}

func (s *repositoryChannelStore) GetByKey(ctx context.Context, channelKey string) (*model.LiveChannel, error) {
	if s.repo == nil {
		return nil, nil
	}
	channel, ok := s.repo.FindByKey(ctx, channelKey)
	if !ok {
		return nil, nil
	}
	return &channel, nil
}

func (s *repositoryChannelStore) UpdateStatus(ctx context.Context, channelID uint64, status string, nodeID uint64, workerID string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.UpdateStatus(ctx, channelID, status, nodeID, workerID)
}

type memorySessionStore struct {
	mu       sync.RWMutex
	sessions map[uint64]*model.LiveSession
}

func NewMemorySessionStore() SessionStore {
	return &memorySessionStore{
		sessions: make(map[uint64]*model.LiveSession),
	}
}

func (s *memorySessionStore) Save(ctx context.Context, session *model.LiveSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copySession := *session
	s.sessions[session.SessionID] = &copySession
	logx.Info("live.session_store.save", logx.Fields{
		"session_id":  session.SessionID,
		"channel_key": session.ChannelKey,
	})
	return nil
}

func (s *memorySessionStore) UpdateStatus(ctx context.Context, sessionID uint64, status string, stoppedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		session.Status = status
		if !stoppedAt.IsZero() {
			session.StoppedAt = &stoppedAt
		}
	}
	return nil
}

func (s *memorySessionStore) FindLatestActiveByChannelID(ctx context.Context, channelID uint64) (*model.LiveSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *model.LiveSession
	for _, session := range s.sessions {
		if session.ChannelID != channelID {
			continue
		}
		switch session.Status {
		case model.LiveSessionStatusConnecting, model.LiveSessionStatusPublishing, model.LiveSessionStatusInterruptWaitResume, model.LiveSessionStatusResumed:
			if latest == nil || session.SessionID > latest.SessionID {
				copySession := *session
				latest = &copySession
			}
		}
	}
	return latest, nil
}

type repositorySessionStore struct {
	repo *mysql.LiveSessionRepository
}

func NewRepositorySessionStore(repo *mysql.LiveSessionRepository) SessionStore {
	return &repositorySessionStore{repo: repo}
}

func (s *repositorySessionStore) Save(ctx context.Context, session *model.LiveSession) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Save(ctx, *session)
}

func (s *repositorySessionStore) UpdateStatus(ctx context.Context, sessionID uint64, status string, stoppedAt time.Time) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.UpdateStatus(ctx, sessionID, status, stoppedAt)
}

func (s *repositorySessionStore) FindLatestActiveByChannelID(ctx context.Context, channelID uint64) (*model.LiveSession, error) {
	if s.repo == nil {
		return nil, nil
	}
	session, ok := s.repo.FindLatestActiveByChannelID(ctx, channelID)
	if !ok {
		return nil, nil
	}
	return &session, nil
}
