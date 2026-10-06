package monitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsMonitorNodeOnline(t *testing.T) {
	now := time.Now()

	if !isMonitorNodeOnline(now.Add(-time.Minute), time.Time{}, now) {
		t.Fatal("expected recent heartbeat to be treated as online")
	}

	if !isMonitorNodeOnline(time.Time{}, now.Add(-30*time.Second), now) {
		t.Fatal("expected recent metrics to be treated as online")
	}

	if isMonitorNodeOnline(now.Add(-3*time.Minute), now.Add(-3*time.Minute), now) {
		t.Fatal("expected stale heartbeat and metrics to be treated as offline")
	}
}

func TestSnapshotUsesUnifiedResponseEnvelope(t *testing.T) {
	handler := NewSnapshotHandler(nil, nil, nil, nil, nil, "standalone")
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/transcode/monitor/snapshot", nil)
	rec := httptest.NewRecorder()

	handler.Snapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Mode string `json:"mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if body.Code != 0 || body.Message != "ok" {
		t.Fatalf("unexpected response envelope: %+v", body)
	}
	if body.Data.Mode != "standalone" {
		t.Fatalf("expected mode standalone, got %q", body.Data.Mode)
	}
}
