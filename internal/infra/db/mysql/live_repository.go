package mysql

import (
	"context"
	"fmt"
	"time"

	"hvc/internal/model"
)

type LiveChannelRepository struct {
	db *DB
}

func NewLiveChannelRepository(db *DB) *LiveChannelRepository {
	return &LiveChannelRepository{db: db}
}

func (r *LiveChannelRepository) Create(ctx context.Context, channel model.LiveChannel) error {
	record := toLiveChannelRecord(channel)
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *LiveChannelRepository) FindByID(ctx context.Context, channelID uint64) (model.LiveChannel, bool) {
	var record LiveChannelRecord
	if err := r.db.WithContext(ctx).Where("channel_id = ?", channelID).Take(&record).Error; err != nil {
		return model.LiveChannel{}, false
	}
	return toLiveChannelModel(record), true
}

func (r *LiveChannelRepository) FindByKey(ctx context.Context, channelKey string) (model.LiveChannel, bool) {
	var record LiveChannelRecord
	if err := r.db.WithContext(ctx).Where("channel_key = ?", channelKey).Take(&record).Error; err != nil {
		return model.LiveChannel{}, false
	}
	return toLiveChannelModel(record), true
}

func (r *LiveChannelRepository) Update(ctx context.Context, channel model.LiveChannel) error {
	record := toLiveChannelRecord(channel)
	return r.db.WithContext(ctx).Model(&LiveChannelRecord{}).Where("channel_id = ?", channel.ChannelID).Updates(map[string]any{
		"channel_name":            record.ChannelName,
		"profile_id":              record.ProfileID,
		"status":                  record.Status,
		"play_domain":             record.PlayDomain,
		"push_domain":             record.PushDomain,
		"enable_source_rendition": record.EnableSourceRendition,
		"enable_watermark":        record.EnableWatermark,
		"assigned_node_id":        record.AssignedNodeID,
		"assigned_worker_id":      record.AssignedWorkerID,
		"updated_at":              time.Now(),
	}).Error
}

func (r *LiveChannelRepository) UpdateStatus(ctx context.Context, channelID uint64, status string, nodeID uint64, workerID string) error {
	return r.db.WithContext(ctx).Model(&LiveChannelRecord{}).Where("channel_id = ?", channelID).Updates(map[string]any{
		"status":             liveChannelStatusToDB(status),
		"assigned_node_id":   nodeID,
		"assigned_worker_id": workerID,
		"updated_at":         time.Now(),
	}).Error
}

type LiveProfileRenditionRepository struct {
	db *DB
}

func NewLiveProfileRenditionRepository(db *DB) *LiveProfileRenditionRepository {
	return &LiveProfileRenditionRepository{db: db}
}

func (r *LiveProfileRenditionRepository) ListEnabledNamesByProfileID(ctx context.Context, profileID uint64) []string {
	if profileID == 0 {
		return nil
	}
	var records []LiveProfileRenditionRecord
	if err := r.db.WithContext(ctx).Where("profile_id = ? AND enabled = ?", profileID, true).Order("id ASC").Find(&records).Error; err != nil {
		return nil
	}
	items := make([]string, 0, len(records))
	for _, record := range records {
		if record.RenditionName == "" {
			continue
		}
		items = append(items, record.RenditionName)
	}
	return items
}

type LiveSessionRepository struct {
	db *DB
}

func NewLiveSessionRepository(db *DB) *LiveSessionRepository {
	return &LiveSessionRepository{db: db}
}

func (r *LiveSessionRepository) Save(ctx context.Context, session model.LiveSession) error {
	record := toLiveSessionRecord(session)
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *LiveSessionRepository) UpdateStatus(ctx context.Context, sessionID uint64, status string, stoppedAt time.Time) error {
	updates := map[string]any{
		"status":     liveSessionStatusToDB(status),
		"updated_at": time.Now(),
	}
	if !stoppedAt.IsZero() {
		updates["ended_at"] = stoppedAt
	}
	return r.db.WithContext(ctx).Model(&LiveSessionRecord{}).Where("session_id = ?", sessionID).Updates(updates).Error
}

func (r *LiveSessionRepository) FindLatestActiveByChannelID(ctx context.Context, channelID uint64) (model.LiveSession, bool) {
	var record LiveSessionRecord
	if err := r.db.WithContext(ctx).
		Where("channel_id = ? AND status IN ?", channelID, []int{
			liveSessionStatusToDB(model.LiveSessionStatusConnecting),
			liveSessionStatusToDB(model.LiveSessionStatusPublishing),
			liveSessionStatusToDB(model.LiveSessionStatusInterruptWaitResume),
			liveSessionStatusToDB(model.LiveSessionStatusResumed),
		}).
		Order("session_id DESC").
		Take(&record).Error; err != nil {
		return model.LiveSession{}, false
	}
	return toLiveSessionModel(record), true
}

func toLiveChannelRecord(channel model.LiveChannel) LiveChannelRecord {
	return LiveChannelRecord{
		ChannelID:             channel.ChannelID,
		ChannelKey:            channel.ChannelKey,
		ChannelName:           channel.ChannelName,
		ProfileID:             channel.ProfileID,
		Status:                liveChannelStatusToDB(channel.Status),
		PlayDomain:            channel.PlayDomain,
		PushDomain:            channel.PushDomain,
		EnableSourceRendition: channel.EnableSourceRendition,
		EnableWatermark:       channel.EnableWatermark,
		AssignedNodeID:        channel.AssignedNodeID,
		AssignedWorkerID:      channel.AssignedWorkerID,
		CreatedAt:             channel.CreatedAt,
		UpdatedAt:             channel.UpdatedAt,
	}
}

func toLiveChannelModel(record LiveChannelRecord) model.LiveChannel {
	return model.LiveChannel{
		ChannelID:             record.ChannelID,
		ChannelKey:            record.ChannelKey,
		ChannelName:           record.ChannelName,
		ProfileID:             record.ProfileID,
		Status:                liveChannelStatusFromDB(record.Status),
		PlayDomain:            record.PlayDomain,
		PushDomain:            record.PushDomain,
		EnableSourceRendition: record.EnableSourceRendition,
		EnableWatermark:       record.EnableWatermark,
		AssignedNodeID:        record.AssignedNodeID,
		AssignedWorkerID:      record.AssignedWorkerID,
		CreatedAt:             record.CreatedAt,
		UpdatedAt:             record.UpdatedAt,
	}
}

func toLiveSessionRecord(session model.LiveSession) LiveSessionRecord {
	record := LiveSessionRecord{
		SessionID:        session.SessionID,
		ChannelID:        session.ChannelID,
		SessionKey:       buildLiveSessionKey(session),
		Status:           liveSessionStatusToDB(session.Status),
		PushProtocol:     session.PushProtocol,
		AssignedNodeID:   session.AssignedNodeID,
		AssignedWorkerID: session.AssignedWorkerID,
		ResumeCount:      session.ResumeCount,
		StartedAt:        session.StartedAt,
		CreatedAt:        session.StartedAt,
		UpdatedAt:        session.StartedAt,
	}
	if session.StoppedAt != nil {
		record.EndedAt = *session.StoppedAt
		record.UpdatedAt = *session.StoppedAt
	}
	return record
}

func toLiveSessionModel(record LiveSessionRecord) model.LiveSession {
	session := model.LiveSession{
		SessionID:        record.SessionID,
		ChannelID:        record.ChannelID,
		Status:           liveSessionStatusFromDB(record.Status),
		PushProtocol:     record.PushProtocol,
		AssignedNodeID:   record.AssignedNodeID,
		AssignedWorkerID: record.AssignedWorkerID,
		StartedAt:        record.StartedAt,
		ResumeCount:      record.ResumeCount,
	}
	if !record.EndedAt.IsZero() {
		stoppedAt := record.EndedAt
		session.StoppedAt = &stoppedAt
	}
	return session
}

func buildLiveSessionKey(session model.LiveSession) string {
	if session.ChannelKey != "" {
		return fmt.Sprintf("%s-%d", session.ChannelKey, session.SessionID)
	}
	return fmt.Sprintf("session-%d", session.SessionID)
}

func liveChannelStatusToDB(status string) int {
	switch status {
	case model.LiveChannelStatusStarting, model.LiveChannelStatusLive:
		return 2
	case model.LiveChannelStatusStopped:
		return 3
	case model.LiveChannelStatusError:
		return 4
	default:
		return 1
	}
}

func liveChannelStatusFromDB(status int) string {
	switch status {
	case 2:
		return model.LiveChannelStatusLive
	case 3:
		return model.LiveChannelStatusStopped
	case 4:
		return model.LiveChannelStatusError
	default:
		return model.LiveChannelStatusIdle
	}
}

func liveSessionStatusToDB(status string) int {
	switch status {
	case model.LiveSessionStatusPublishing, model.LiveSessionStatusResumed:
		return 2
	case model.LiveSessionStatusStopped:
		return 3
	case model.LiveSessionStatusInterruptWaitResume:
		return 4
	case model.LiveSessionStatusRejected:
		return 5
	default:
		return 1
	}
}

func liveSessionStatusFromDB(status int) string {
	switch status {
	case 2:
		return model.LiveSessionStatusPublishing
	case 3:
		return model.LiveSessionStatusStopped
	case 4:
		return model.LiveSessionStatusInterruptWaitResume
	case 5:
		return model.LiveSessionStatusRejected
	default:
		return model.LiveSessionStatusConnecting
	}
}
