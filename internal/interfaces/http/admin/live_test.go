package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hvc/internal/config"
	"hvc/internal/model"
	liveservice "hvc/internal/service/live"
	"hvc/pkg/idgen"
)

func TestChannelDetailByChannelKeyReturnsChannelDetail(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := liveservice.NewChannelService(config.DynamicRuntimeConfig{
		Storage: config.StorageConfig{
			PlayDomain: "https://play.example.com",
			FLVDomain:  "https://flv.example.com",
		},
	}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)

	channel, err := service.CreateChannelContext(t.Context(), "live-1001", "room-1", 1)
	if err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/live/channel/detail?channel_key="+channel.ChannelKey, nil)
	resp := httptest.NewRecorder()
	handler.ChannelDetail(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	data := body["data"].(map[string]any)
	channelData := data["channel"].(map[string]any)
	playbackData := data["playback"].(map[string]any)
	if channelData["channel_key"] != "live-1001" {
		t.Fatalf("unexpected channel key: %v", channelData["channel_key"])
	}
	if playbackData["channel_key"] != "live-1001" {
		t.Fatalf("unexpected playback channel key: %v", playbackData["channel_key"])
	}
}

func TestUpdateChannelOnlyTouchesProvidedFields(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := liveservice.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)

	channel, err := service.CreateChannelContext(t.Context(), "live-2001", "room-2", 1)
	if err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	enabled := true
	if _, _, err := service.UpdateChannelContext(t.Context(), liveservice.ChannelPatch{
		ChannelID:       channel.ChannelID,
		EnableWatermark: &enabled,
	}); err != nil {
		t.Fatalf("preset watermark failed: %v", err)
	}

	payload := map[string]any{
		"channel_id":   channel.ChannelID,
		"channel_name": "room-2-updated",
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/live/channel/update", bytes.NewReader(raw))
	resp := httptest.NewRecorder()
	handler.UpdateChannel(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	updated, ok := service.GetChannelByIDContext(t.Context(), channel.ChannelID)
	if !ok {
		t.Fatal("channel not found after update")
	}
	if updated.ChannelName != "room-2-updated" {
		t.Fatalf("unexpected channel name: %s", updated.ChannelName)
	}
	if !updated.EnableWatermark {
		t.Fatal("enable_watermark should stay true when request omits the field")
	}
}

func TestListChannelsReturnsPagedResult(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := liveservice.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)

	for i := 0; i < 3; i++ {
		_, err := service.CreateChannelContext(t.Context(), "live-list-"+time.Now().Add(time.Duration(i)*time.Second).Format("150405"), "room", 1)
		if err != nil {
			t.Fatalf("create channel failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/live/channel/list?page=1&page_size=2", nil)
	resp := httptest.NewRecorder()
	handler.ListChannels(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var result struct {
		Data struct {
			Total int64               `json:"total"`
			Items []model.LiveChannel `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if result.Data.Total != 3 {
		t.Fatalf("unexpected total: %d", result.Data.Total)
	}
	if len(result.Data.Items) != 2 {
		t.Fatalf("unexpected page item count: %d", len(result.Data.Items))
	}
}

func TestDeleteChannelRemovesExistingChannel(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := liveservice.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)

	channel, err := service.CreateChannelContext(t.Context(), "live-delete-1", "room-delete", 1)
	if err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{"channel_id": channel.ChannelID})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/live/channel/delete", bytes.NewReader(raw))
	resp := httptest.NewRecorder()
	handler.DeleteChannel(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	if _, ok := service.GetChannelByIDContext(t.Context(), channel.ChannelID); ok {
		t.Fatal("channel should be removed after delete")
	}
}

func TestStartChannelReturnsControlDispatchMetadata(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := liveservice.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)
	handler.ConfigureInternalControl(nil, "", 1001)

	channel, err := service.CreateChannelContext(t.Context(), "live-start-1", "room-start", 1)
	if err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"channel_id": channel.ChannelID,
		"worker_id":  "worker-a",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/live/channel/start", bytes.NewReader(raw))
	resp := httptest.NewRecorder()
	handler.StartChannel(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var body struct {
		Data struct {
			ChannelID      uint64 `json:"channel_id"`
			ControlPath    string `json:"control_path"`
			TargetNodeID   uint64 `json:"target_node_id"`
			TargetWorkerID string `json:"target_worker_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Data.ChannelID != channel.ChannelID {
		t.Fatalf("unexpected channel id: %d", body.Data.ChannelID)
	}
	if body.Data.ControlPath != "local" {
		t.Fatalf("unexpected control path: %s", body.Data.ControlPath)
	}
	if body.Data.TargetNodeID != 1001 {
		t.Fatalf("unexpected target node id: %d", body.Data.TargetNodeID)
	}
	if body.Data.TargetWorkerID != "worker-a" {
		t.Fatalf("unexpected target worker id: %s", body.Data.TargetWorkerID)
	}
}

func TestStopChannelReturnsControlDispatchMetadata(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := liveservice.NewChannelService(config.DynamicRuntimeConfig{}, nil, nil, nil)
	handler := NewLiveHandler(nil, service)
	handler.ConfigureInternalControl(nil, "", 1001)

	channel, err := service.CreateChannelContext(t.Context(), "live-stop-1", "room-stop", 1)
	if err != nil {
		t.Fatalf("create channel failed: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"channel_id": channel.ChannelID,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/live/channel/stop", bytes.NewReader(raw))
	resp := httptest.NewRecorder()
	handler.StopChannel(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var body struct {
		Data struct {
			ChannelID      uint64 `json:"channel_id"`
			ControlPath    string `json:"control_path"`
			TargetNodeID   uint64 `json:"target_node_id"`
			TargetWorkerID string `json:"target_worker_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Data.ChannelID != channel.ChannelID {
		t.Fatalf("unexpected channel id: %d", body.Data.ChannelID)
	}
	if body.Data.ControlPath != "local" {
		t.Fatalf("unexpected control path: %s", body.Data.ControlPath)
	}
	if body.Data.TargetNodeID != 1001 {
		t.Fatalf("unexpected target node id: %d", body.Data.TargetNodeID)
	}
	if body.Data.TargetWorkerID != "" {
		t.Fatalf("unexpected target worker id: %s", body.Data.TargetWorkerID)
	}
}
