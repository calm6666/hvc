package mysql

import (
	"context"
	"errors"
	"time"

	"hvc/pkg/idgen"

	"gorm.io/gorm"
)

// LivePublishSessionRepository 表示直播推流会话仓储。
//
// 这层负责把“推流鉴权通过后真正连上来 / 断开 / 被拒绝”的链路落到 t_live_publish_session，
// 便于定位直播入口侧问题。
type LivePublishSessionRepository struct {
	db *DB
}

// NewLivePublishSessionRepository 创建直播推流会话仓储。
func NewLivePublishSessionRepository(db *DB) *LivePublishSessionRepository {
	return &LivePublishSessionRepository{db: db}
}

// SaveConnected 保存一条已接入中的推流会话记录。
func (r *LivePublishSessionRepository) SaveConnected(ctx context.Context, channelID uint64, sessionID uint64, streamKey string, publishIP string) error {
	now := time.Now()
	sessionRef := sessionID
	record := LivePublishSessionRecord{
		PublishSessionID: idgen.Next(),
		ChannelID:        channelID,
		SessionID:        &sessionRef,
		StreamKey:        streamKey,
		PublishIP:        publishIP,
		PublishStatus:    2,
		ConnectedAt:      &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

// SaveRejected 保存一条被拒绝的推流接入记录。
func (r *LivePublishSessionRepository) SaveRejected(ctx context.Context, channelID uint64, streamKey string, publishIP string) error {
	now := time.Now()
	record := LivePublishSessionRecord{
		PublishSessionID: idgen.Next(),
		ChannelID:        channelID,
		StreamKey:        streamKey,
		PublishIP:        publishIP,
		PublishStatus:    4,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

// MarkDisconnectedLatest 按频道和 stream_key 关闭最近一条仍在推流中的记录。
func (r *LivePublishSessionRepository) MarkDisconnectedLatest(ctx context.Context, channelID uint64, streamKey string) error {
	now := time.Now()
	query := r.db.WithContext(ctx).Model(&LivePublishSessionRecord{}).
		Where("channel_id = ? AND publish_status = ?", channelID, 2)
	if streamKey != "" {
		query = query.Where("stream_key = ?", streamKey)
	}

	var record LivePublishSessionRecord
	if err := query.Order("publish_session_id DESC").Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	return r.db.WithContext(ctx).Model(&LivePublishSessionRecord{}).
		Where("publish_session_id = ?", record.PublishSessionID).
		Updates(map[string]any{
			"publish_status":  3,
			"disconnected_at": now,
			"updated_at":      now,
		}).Error
}
