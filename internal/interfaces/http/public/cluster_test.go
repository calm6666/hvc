package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClusterHandler_ReportHeartbeat_AcceptsIdentityPayload(t *testing.T) {
	handler := NewClusterHandler(nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/internal/worker/heartbeat", strings.NewReader(`{"node_id":1,"worker_id":"worker-1","startup_instance_id":"worker-1-startup","machine_fingerprint":"node-worker-1"}`))
	w := httptest.NewRecorder()
	handler.ReportHeartbeat(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}
