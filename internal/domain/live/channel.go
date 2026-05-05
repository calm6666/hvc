package live

import "time"

// Channel 表示直播频道领域对象。
type Channel struct {
	ChannelID   uint64
	ChannelKey  string
	ChannelName string
	Status      int
	ProfileID   uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Session 表示直播会话领域对象。
type Session struct {
	SessionID  uint64
	ChannelID  uint64
	Status     int
	StartedAt  time.Time
	EndedAt    time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
