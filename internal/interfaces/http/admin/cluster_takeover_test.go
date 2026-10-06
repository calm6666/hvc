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

	if rec.Code != http.StatusOK {
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

func TestListNodeMetricsReturnsEmptyPagedResultWhenRepositoryMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cluster/node/metrics?page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	handler.ListNodeMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Page     int               `json:"page"`
			PageSize int               `json:"page_size"`
			Total    int64             `json:"total"`
			Items    []json.RawMessage `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if body.Code != 0 || body.Data.Page != 1 || body.Data.PageSize != 20 || body.Data.Total != 0 || len(body.Data.Items) != 0 {
		t.Fatalf("unexpected paged payload: %+v", body)
	}
}

func TestListMembersReturnsEmptyPagedResultWhenRepositoryMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cluster/member/list?page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	handler.ListMembers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Page     int                 `json:"page"`
			PageSize int                 `json:"page_size"`
			Total    int64               `json:"total"`
			Items    []clusterMemberView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if body.Code != 0 || body.Data.Page != 1 || body.Data.PageSize != 20 || body.Data.Total != 0 || len(body.Data.Items) != 0 {
		t.Fatalf("unexpected paged payload: %+v", body)
	}
}

func TestListWorkersReturnsRequestedPageWhenRepositoryMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cluster/worker/list?page=3&page_size=15", nil)
	rec := httptest.NewRecorder()

	handler.ListWorkers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Page     int                         `json:"page"`
			PageSize int                         `json:"page_size"`
			Total    int64                       `json:"total"`
			Items    []clusterWorkerListItemView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if body.Code != 0 || body.Data.Page != 3 || body.Data.PageSize != 15 || body.Data.Total != 0 || len(body.Data.Items) != 0 {
		t.Fatalf("unexpected paged payload: %+v", body)
	}
}

func TestSetNodeEnabledReturnsServiceUnavailableWhenRepositoryMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cluster/node/enabled", strings.NewReader(`{"node_id":1,"enabled":false}`))
	rec := httptest.NewRecorder()

	handler.SetNodeEnabled(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestVersionSummaryReturnsEmptySafeDefaultsWhenDependenciesMissing(t *testing.T) {
	handler := &ClusterHandler{}

	result := handler.versionSummary(t.Context())

	if result["mysql_server_version"] != "" {
		t.Fatalf("expected empty mysql version, got %+v", result["mysql_server_version"])
	}
	if result["redis_server_version"] != "" {
		t.Fatalf("expected empty redis version, got %+v", result["redis_server_version"])
	}
	if result["runtime_config_db_version"] != uint64(0) {
		t.Fatalf("expected db version 0, got %+v", result["runtime_config_db_version"])
	}
	if result["runtime_config_cache_exists"] != false {
		t.Fatalf("expected cache exists false, got %+v", result["runtime_config_cache_exists"])
	}
}

func TestOverviewReturnsOKWhenCoreDependenciesMissing(t *testing.T) {
	handler := &ClusterHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cluster/overview", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("overview should not panic: %v", recovered)
		}
	}()

	handler.Overview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTakeoverWorkerJobsIfNeededSkipsWhenDisabled(t *testing.T) {
	handler := &ClusterHandler{}
	result, err := handler.takeoverWorkerJobsIfNeeded(t.Context(), 1, "worker-a", "manual", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil takeover result when disabled")
	}
}

func TestTakeoverWorkerJobsIfNeededRequiresRepository(t *testing.T) {
	handler := &ClusterHandler{}
	if _, err := handler.takeoverWorkerJobsIfNeeded(t.Context(), 1, "worker-a", "manual", true); err == nil {
		t.Fatal("expected missing job repository to fail")
	}
}

func TestTakeoverNodeJobsIfNeededSkipsWhenDisabled(t *testing.T) {
	handler := &ClusterHandler{}
	result, err := handler.takeoverNodeJobsIfNeeded(t.Context(), 1, "manual", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil takeover result when disabled")
	}
}

func TestTakeoverNodeJobsIfNeededRequiresRepository(t *testing.T) {
	handler := &ClusterHandler{}
	if _, err := handler.takeoverNodeJobsIfNeeded(t.Context(), 1, "manual", true); err == nil {
		t.Fatal("expected missing job repository to fail")
	}
}
