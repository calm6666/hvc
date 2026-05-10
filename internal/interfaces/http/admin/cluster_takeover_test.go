package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForceTakeoverJobsRequiresNodeOrWorker(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cluster/job/takeover", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handler.ForceTakeoverJobs(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected business code 400, got %d", resp.Code)
	}
}

func TestForceTakeoverJobsReturnsServiceUnavailableWhenRepositoryMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cluster/job/takeover", strings.NewReader(`{"node_id":1}`))
	rec := httptest.NewRecorder()

	handler.ForceTakeoverJobs(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.Code != 503 {
		t.Fatalf("expected business code 503, got %d", resp.Code)
	}
}

func TestResourceDistributionReturnsOKWhenRepositoryMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cluster/resource/distribution", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("resource distribution should not panic: %v", recovered)
		}
	}()
	handler.ResourceDistribution(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
