package live

import (
	"context"
	"testing"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"
)

func TestStartChannelSkipsZeroSessionEvent(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)

	channelStore := &memoryChannelStore{
		channels:   map[uint64]*model.LiveChannel{},
		channelKey: map[string]uint64{},
	}
	channelStore.channels[1001] = &model.LiveChannel{
		ChannelID:  1001,
		ChannelKey: "live-zero-event",
		Status:     model.LiveChannelStatusIdle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	channelStore.channelKey["live-zero-event"] = 1001

	eventWriter := &fakeSessionEventWriter{}
	manager := NewManager(channelStore, NewMemorySessionStore())
	manager.SetSessionEventWriter(eventWriter)

	if err := manager.StartChannel(context.Background(), 1001, 11, "worker-a"); err != nil {
		t.Fatalf("start channel failed: %v", err)
	}
	if eventWriter.called != 0 {
		t.Fatalf("expected no session event to be stored before session exists, got %d", eventWriter.called)
	}
}

func TestPublishLifecyclePersistsOnlySessionBoundEvents(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)

	channelStore := &memoryChannelStore{
		channels:   map[uint64]*model.LiveChannel{},
		channelKey: map[string]uint64{},
	}
	channelStore.channels[1002] = &model.LiveChannel{
		ChannelID:  1002,
		ChannelKey: "live-publish-1",
		Status:     model.LiveChannelStatusIdle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	channelStore.channelKey["live-publish-1"] = 1002

	eventWriter := &fakeSessionEventWriter{}
	publishWriter := &fakePublishSessionWriter{}
	manager := NewManager(channelStore, NewMemorySessionStore())
	manager.SetSessionEventWriter(eventWriter)
	manager.SetPublishSessionWriter(publishWriter)

	session, err := manager.OnPublish(context.Background(), "live-publish-1", 22, "worker-b", "stream-a", "127.0.0.1", "rtmp", "rtmp://ingest/live", "https://play/live.m3u8")
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if session == nil || session.SessionID == 0 {
		t.Fatal("expected session to be created")
	}
	if publishWriter.connected != 1 {
		t.Fatalf("expected SaveConnected to be called once, got %d", publishWriter.connected)
	}
	if len(eventWriter.sessionIDs) != 1 || eventWriter.sessionIDs[0] != session.SessionID {
		t.Fatalf("expected one publish-connected event bound to session %d, got %+v", session.SessionID, eventWriter.sessionIDs)
	}

	if err := manager.OnUnpublish(context.Background(), "live-publish-1", "stream-a"); err != nil {
		t.Fatalf("unpublish failed: %v", err)
	}
	if publishWriter.disconnected != 1 {
		t.Fatalf("expected MarkDisconnectedLatest to be called once, got %d", publishWriter.disconnected)
	}
	if len(eventWriter.sessionIDs) != 2 || eventWriter.sessionIDs[1] != session.SessionID {
		t.Fatalf("expected disconnect event bound to same session %d, got %+v", session.SessionID, eventWriter.sessionIDs)
	}
	if manager.ActiveSessionCount() != 0 {
		t.Fatalf("expected no active sessions after unpublish, got %d", manager.ActiveSessionCount())
	}
}

type fakeSessionEventWriter struct {
	called     int
	sessionIDs []uint64
}

func (f *fakeSessionEventWriter) SaveEvent(_ context.Context, sessionID uint64, _ uint64, _ string, _ any) error {
	f.called++
	f.sessionIDs = append(f.sessionIDs, sessionID)
	return nil
}

type fakePublishSessionWriter struct {
	connected    int
	disconnected int
}

func (f *fakePublishSessionWriter) SaveConnected(_ context.Context, _ uint64, _ uint64, _ string, _ string) error {
	f.connected++
	return nil
}

func (f *fakePublishSessionWriter) MarkDisconnectedLatest(_ context.Context, _ uint64, _ string) error {
	f.disconnected++
	return nil
}
