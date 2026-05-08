package mysql

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm/clause"
	"hvc/pkg/idgen"
)

// LivePlaybackTokenRepository 表示直播播放令牌仓储。
//
// 这里保存的是“系统实际签发出去的播放令牌快照”，
// 主要用于排障和审计，而不是替代 URL 鉴权本身。
type LivePlaybackTokenRepository struct {
	db *DB
}

// NewLivePlaybackTokenRepository 创建直播播放令牌仓储。
func NewLivePlaybackTokenRepository(db *DB) *LivePlaybackTokenRepository {
	return &LivePlaybackTokenRepository{db: db}
}

// SaveIssuedToken 保存一次已签发的播放令牌。
func (r *LivePlaybackTokenRepository) SaveIssuedToken(ctx context.Context, channelID uint64, userToken string, viewerID string, expireAt time.Time) error {
	now := time.Now()
	record := LivePlaybackTokenRecord{
		TokenID:   idgen.Next(),
		ChannelID: channelID,
		UserToken: userToken,
		ViewerID:  viewerID,
		AllowPlay: true,
		ExpireAt:  expireAt,
		IssuedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "channel_id"},
			{Name: "user_token"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"viewer_id":  record.ViewerID,
			"allow_play": record.AllowPlay,
			"expire_at":  record.ExpireAt,
			"issued_at":  record.IssuedAt,
			"updated_at": record.UpdatedAt,
		}),
	}).Create(&record).Error
}

// LivePublishAuthLogRepository 表示直播推流鉴权日志仓储。
type LivePublishAuthLogRepository struct {
	db *DB
}

// NewLivePublishAuthLogRepository 创建直播推流鉴权日志仓储。
func NewLivePublishAuthLogRepository(db *DB) *LivePublishAuthLogRepository {
	return &LivePublishAuthLogRepository{db: db}
}

// SaveAttempt 保存一次推流鉴权结果。
func (r *LivePublishAuthLogRepository) SaveAttempt(ctx context.Context, channelID uint64, streamKey string, requestIP string, allowed bool, message string) error {
	record := LivePublishAuthLogRecord{
		AuthLogID:   idgen.Next(),
		ChannelID:   channelID,
		StreamKey:   streamKey,
		RequestIP:   requestIP,
		AuthResult:  authResultCode(allowed),
		AuthMessage: message,
		CreatedAt:   time.Now(),
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

// LiveSessionEventRepository 表示直播会话事件仓储。
type LiveSessionEventRepository struct {
	db *DB
}

// NewLiveSessionEventRepository 创建直播会话事件仓储。
func NewLiveSessionEventRepository(db *DB) *LiveSessionEventRepository {
	return &LiveSessionEventRepository{db: db}
}

// SaveEvent 保存一条直播会话事件。
//
// payload 会被序列化为 JSON 落库，方便后续审计和问题回放。
func (r *LiveSessionEventRepository) SaveEvent(ctx context.Context, sessionID uint64, channelID uint64, eventType string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var sessionRef *uint64
	if sessionID > 0 {
		sessionRef = &sessionID
	}
	record := LiveSessionEventRecord{
		EventID:          idgen.Next(),
		SessionID:        sessionRef,
		ChannelID:        channelID,
		EventType:        eventType,
		EventPayloadJSON: string(payloadJSON),
		CreatedAt:        time.Now(),
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func authResultCode(value bool) int {
	if value {
		return 1
	}
	return 2
}
