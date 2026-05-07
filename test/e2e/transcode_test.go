package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hvc/internal/interfaces/http/public"
)

func TestCreateJobE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	handler := public.NewTranscodeHandler(nil, nil, nil, nil)

	reqBody := map[string]any{
		"request_id": "e2e-test-001",
		"source_url": "https://example.com/test.mp4",
		"segment_options": map[string]any{
			"segment_duration_sec": 6,
			"support_dash":         true,
			"support_hls":          true,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/job", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateJob(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestQueryProgressE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	handler := public.NewTranscodeHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/progress?request_id=e2e-test-001", nil)
	w := httptest.NewRecorder()

	handler.QueryProgress(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
