package live

import (
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"sync"
	"time"
)

// Manager 表示直播管理器。
type Manager struct {
	mu       sync.RWMutex
	channels map[string]model.LiveChannel
}

// NewManager 创建直播管理器。
func NewManager() *Manager {
	return &Manager{channels: make(map[string]model.LiveChannel)}
}

// SaveChannel 保存直播频道。
func (m *Manager) SaveChannel(channel model.LiveChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if channel.ChannelID == 0 {
		channel.ChannelID = idgen.Next()
	}
	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = time.Now()
	}
	channel.UpdatedAt = time.Now()
	m.channels[channel.ChannelKey] = channel
}

// GetChannel 获取直播频道。
func (m *Manager) GetChannel(channelKey string) (model.LiveChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	channel, ok := m.channels[channelKey]
	return channel, ok
}
