package public

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hvc/internal/config"
	authlive "hvc/internal/live"
	livemanager "hvc/internal/live"
	"hvc/internal/model"
	livesvc "hvc/internal/service/live"
	"hvc/pkg/idgen"
)

func TestGetPlaybackInfoReturnsSignedURLs(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := livesvc.NewChannelService(config.DynamicRuntimeConfig{
		Storage: config.StorageConfig{
			PlayDomain: "https://play.example.com",
			FLVDomain:  "https://flv.example.com",
		},
	}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)

	if _, err := service.CreateChannelContext(t.Context(), "live-3001", "room-3", 1); err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/live/channel/playback?channel_key=live-3001", nil)
	resp := httptest.NewRecorder()
	handler.GetPlaybackInfo(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"play_token\":") {
		t.Fatalf("expected play_token in body: %s", body)
	}
	if !strings.Contains(body, "master.m3u8?expire=") {
		t.Fatalf("expected signed master hls url in body: %s", body)
	}
	if !strings.Contains(body, "\"renditions\":") {
		t.Fatalf("expected rendition urls in body: %s", body)
	}
}

func TestGetPlaybackInfoReturns404ForUnknownChannel(t *testing.T) {
	handler := NewLiveHandler(nil, livesvc.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/v1/live/channel/playback?channel_key=missing", nil)
	resp := httptest.NewRecorder()
	handler.GetPlaybackInfo(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
}

func TestPlayAuthAcceptsSignedQuery(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := livesvc.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)

	if _, err := service.CreateChannelContext(t.Context(), "live-3002", "room-4", 1); err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	values := authlive.GeneratePlayAuthValues(service.CurrentAuthConfig(), "live-3002")
	req := httptest.NewRequest(http.MethodGet, "/v1/live/channel/play-auth?channel_key=live-3002&"+values.Encode(), nil)
	resp := httptest.NewRecorder()
	handler.PlayAuth(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "\"allowed\":true") {
		t.Fatalf("unexpected body: %s", resp.Body.String())
	}
}

func TestGetPlaybackInfoRecordsIssuedToken(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := livesvc.NewChannelService(config.DynamicRuntimeConfig{
		Storage: config.StorageConfig{
			PlayDomain: "https://play.example.com",
		},
	}, nil, nil, nil)
	writer := &fakePlaybackTokenWriter{}
	service.SetPlaybackTokenWriter(writer)
	handler := NewLiveHandler(nil, service)

	if _, err := service.CreateChannelContext(t.Context(), "live-3003", "room-5", 1); err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/live/channel/playback?channel_key=live-3003&viewer_id=user-1", nil)
	resp := httptest.NewRecorder()
	handler.GetPlaybackInfo(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	if writer.called != 1 {
		t.Fatalf("expected playback token writer to be called once, got %d", writer.called)
	}
	if writer.viewerID != "user-1" {
		t.Fatalf("unexpected viewer id: %s", writer.viewerID)
	}
	if writer.userToken == "" {
		t.Fatal("expected user token to be recorded")
	}
}

func TestPushAuthRecordsPublishAuthLog(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := livesvc.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	logger := &fakePublishAuthLogger{}
	service.SetPublishAuthLogger(logger)
	handler := NewLiveHandler(nil, service)

	if _, err := service.CreateChannelContext(t.Context(), "live-3004", "room-6", 1); err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	values := authlive.GeneratePushAuthValues(service.CurrentAuthConfig(), "live-3004")
	req := httptest.NewRequest(http.MethodGet, "/v1/live/channel/push-auth?channel_key=live-3004&stream_key=stream-1&"+values.Encode(), nil)
	req.RemoteAddr = "127.0.0.1:12345"
	resp := httptest.NewRecorder()
	handler.PushAuth(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	if logger.called != 1 {
		t.Fatalf("expected publish auth logger to be called once, got %d", logger.called)
	}
	if logger.streamKey != "stream-1" {
		t.Fatalf("unexpected stream key: %s", logger.streamKey)
	}
	if logger.requestIP != "127.0.0.1" {
		t.Fatalf("unexpected request ip: %s", logger.requestIP)
	}
	if !logger.allowed {
		t.Fatal("expected allowed auth attempt to be recorded")
	}
}

func TestPublishConnectedAndDisconnectedEndpoints(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)

	channelStore := newTestLiveChannelStore(model.LiveChannel{
		ChannelID:  7001,
		ChannelKey: "live-endpoint-1",
		Status:     model.LiveChannelStatusIdle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	manager := livemanager.NewManager(channelStore, newTestLiveSessionStore())
	handler := NewLiveHandler(manager, nil)

	connectBody, _ := json.Marshal(map[string]any{
		"channel_key":      "live-endpoint-1",
		"node_id":          1,
		"worker_id":        "worker-x",
		"stream_key":       "stream-x",
		"publish_ip":       "127.0.0.1",
		"push_protocol":    "rtmp",
		"ingest_url":       "rtmp://ingest/live-endpoint-1",
		"playback_hls_url": "https://play/live-endpoint-1/master.m3u8",
	})
	connectReq := httptest.NewRequest(http.MethodPost, "/v1/internal/live/publish-connected", bytes.NewReader(connectBody))
	connectResp := httptest.NewRecorder()
	handler.PublishConnected(connectResp, connectReq)
	if connectResp.Code != http.StatusOK {
		t.Fatalf("unexpected connect status code: %d, body=%s", connectResp.Code, connectResp.Body.String())
	}

	session, ok := manager.GetActiveSession(t.Context(), "live-endpoint-1")
	if !ok || session == nil {
		t.Fatal("expected active session after publish-connected")
	}

	disconnectBody, _ := json.Marshal(map[string]any{
		"channel_key": "live-endpoint-1",
		"stream_key":  "stream-x",
	})
	disconnectReq := httptest.NewRequest(http.MethodPost, "/v1/internal/live/publish-disconnected", bytes.NewReader(disconnectBody))
	disconnectResp := httptest.NewRecorder()
	handler.PublishDisconnected(disconnectResp, disconnectReq)
	if disconnectResp.Code != http.StatusOK {
		t.Fatalf("unexpected disconnect status code: %d, body=%s", disconnectResp.Code, disconnectResp.Body.String())
	}
	if _, ok := manager.GetActiveSession(t.Context(), "live-endpoint-1"); ok {
		t.Fatal("expected no active session after publish-disconnected")
	}
}

func TestPublishInterruptedEndpoint(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)

	channelStore := newTestLiveChannelStore(model.LiveChannel{
		ChannelID:  7002,
		ChannelKey: "live-endpoint-2",
		Status:     model.LiveChannelStatusIdle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	manager := livemanager.NewManager(channelStore, newTestLiveSessionStore())
	handler := NewLiveHandler(manager, nil)

	_, err := manager.OnPublish(t.Context(), "live-endpoint-2", 2, "worker-y", "stream-y", "127.0.0.1", "rtmp", "", "")
	if err != nil {
		t.Fatalf("prepare publish failed: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"channel_key": "live-endpoint-2",
		"stream_key":  "stream-y",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/internal/live/publish-interrupted", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	handler.PublishInterrupted(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected interrupt status code: %d, body=%s", resp.Code, resp.Body.String())
	}

	session, ok := manager.GetActiveSession(t.Context(), "live-endpoint-2")
	if !ok || session == nil {
		t.Fatal("expected active session after interrupt")
	}
	if session.Status != model.LiveSessionStatusInterruptWaitResume {
		t.Fatalf("unexpected session status after interrupt: %s", session.Status)
	}
}

func TestStartAndStopRequestedEndpoints(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)

	channelStore := newTestLiveChannelStore(model.LiveChannel{
		ChannelID:  7003,
		ChannelKey: "live-endpoint-3",
		Status:     model.LiveChannelStatusIdle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	manager := livemanager.NewManager(channelStore, newTestLiveSessionStore())
	handler := NewLiveHandler(manager, nil)

	startBody, _ := json.Marshal(map[string]any{
		"channel_id": 7003,
		"node_id":    9,
		"worker_id":  "worker-z",
	})
	startReq := httptest.NewRequest(http.MethodPost, "/v1/internal/live/channel/start", bytes.NewReader(startBody))
	startResp := httptest.NewRecorder()
	handler.StartRequested(startResp, startReq)
	if startResp.Code != http.StatusOK {
		t.Fatalf("unexpected start status code: %d, body=%s", startResp.Code, startResp.Body.String())
	}

	channel, ok := manager.GetChannel(t.Context(), 7003)
	if !ok || channel == nil {
		t.Fatal("expected channel to exist after start-requested")
	}
	if channel.Status != model.LiveChannelStatusStarting {
		t.Fatalf("unexpected channel status after start-requested: %s", channel.Status)
	}
	if channel.AssignedNodeID != 9 || channel.AssignedWorkerID != "worker-z" {
		t.Fatalf("unexpected channel assignment after start-requested: %+v", channel)
	}

	stopBody, _ := json.Marshal(map[string]any{"channel_id": 7003})
	stopReq := httptest.NewRequest(http.MethodPost, "/v1/internal/live/channel/stop", bytes.NewReader(stopBody))
	stopResp := httptest.NewRecorder()
	handler.StopRequested(stopResp, stopReq)
	if stopResp.Code != http.StatusOK {
		t.Fatalf("unexpected stop status code: %d, body=%s", stopResp.Code, stopResp.Body.String())
	}

	channel, ok = manager.GetChannel(t.Context(), 7003)
	if !ok || channel == nil {
		t.Fatal("expected channel to exist after stop-requested")
	}
	if channel.Status != model.LiveChannelStatusStopped {
		t.Fatalf("unexpected channel status after stop-requested: %s", channel.Status)
	}
}

type fakePlaybackTokenWriter struct {
	called    int
	channelID uint64
	userToken string
	viewerID  string
	expireAt  time.Time
}

func (f *fakePlaybackTokenWriter) SaveIssuedToken(_ context.Context, channelID uint64, userToken string, viewerID string, expireAt time.Time) error {
	f.called++
	f.channelID = channelID
	f.userToken = userToken
	f.viewerID = viewerID
	f.expireAt = expireAt
	return nil
}

type fakePublishAuthLogger struct {
	called    int
	channelID uint64
	streamKey string
	requestIP string
	allowed   bool
	message   string
}

func (f *fakePublishAuthLogger) SaveAttempt(_ context.Context, channelID uint64, streamKey string, requestIP string, allowed bool, message string) error {
	f.called++
	f.channelID = channelID
	f.streamKey = streamKey
	f.requestIP = requestIP
	f.allowed = allowed
	f.message = message
	return nil
}

type testLiveChannelStore struct {
	byID  map[uint64]model.LiveChannel
	byKey map[string]uint64
}

func newTestLiveChannelStore(channels ...model.LiveChannel) *testLiveChannelStore {
	store := &testLiveChannelStore{
		byID:  make(map[uint64]model.LiveChannel, len(channels)),
		byKey: make(map[string]uint64, len(channels)),
	}
	for _, channel := range channels {
		store.byID[channel.ChannelID] = channel
		store.byKey[channel.ChannelKey] = channel.ChannelID
	}
	return store
}

func (s *testLiveChannelStore) GetByID(_ context.Context, channelID uint64) (*model.LiveChannel, error) {
	channel, ok := s.byID[channelID]
	if !ok {
		return nil, nil
	}
	copy := channel
	return &copy, nil
}

func (s *testLiveChannelStore) GetByKey(_ context.Context, channelKey string) (*model.LiveChannel, error) {
	channelID, ok := s.byKey[channelKey]
	if !ok {
		return nil, nil
	}
	channel := s.byID[channelID]
	copy := channel
	return &copy, nil
}

func (s *testLiveChannelStore) UpdateStatus(_ context.Context, channelID uint64, status string, nodeID uint64, workerID string) error {
	channel, ok := s.byID[channelID]
	if !ok {
		return nil
	}
	channel.Status = status
	channel.AssignedNodeID = nodeID
	channel.AssignedWorkerID = workerID
	channel.UpdatedAt = time.Now()
	s.byID[channelID] = channel
	s.byKey[channel.ChannelKey] = channelID
	return nil
}

type testLiveSessionStore struct {
	items map[uint64]model.LiveSession
}

func newTestLiveSessionStore() *testLiveSessionStore {
	return &testLiveSessionStore{items: make(map[uint64]model.LiveSession)}
}

func (s *testLiveSessionStore) Save(_ context.Context, session *model.LiveSession) error {
	copy := *session
	s.items[session.SessionID] = copy
	return nil
}

func (s *testLiveSessionStore) UpdateStatus(_ context.Context, sessionID uint64, status string, stoppedAt time.Time) error {
	session, ok := s.items[sessionID]
	if !ok {
		return nil
	}
	session.Status = status
	if !stoppedAt.IsZero() {
		session.StoppedAt = &stoppedAt
	}
	s.items[sessionID] = session
	return nil
}

func (s *testLiveSessionStore) FindLatestActiveByChannelID(_ context.Context, channelID uint64) (*model.LiveSession, error) {
	var latest *model.LiveSession
	for _, session := range s.items {
		if session.ChannelID != channelID {
			continue
		}
		switch session.Status {
		case model.LiveSessionStatusPublishing, model.LiveSessionStatusInterruptWaitResume, model.LiveSessionStatusResumed:
			copy := session
			if latest == nil || copy.SessionID > latest.SessionID {
				latest = &copy
			}
		}
	}
	return latest, nil
}
