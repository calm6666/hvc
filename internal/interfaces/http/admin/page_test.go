package admin

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestParsePageParamsNormalizesValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/admin/system/user/list?page=0&page_size=500", nil)
	page, pageSize := parsePageParams(req)
	if page != 1 {
		t.Fatalf("unexpected normalized page: %d", page)
	}
	if pageSize != 100 {
		t.Fatalf("unexpected normalized page_size: %d", pageSize)
	}
}

func TestWritePageResponseUsesUnifiedPagingShape(t *testing.T) {
	resp := httptest.NewRecorder()
	writePageResponse(resp, 2, 10, 35, []map[string]any{
		{"id": 1},
	})

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Page     int                `json:"page"`
			PageSize int                `json:"page_size"`
			Total    int64              `json:"total"`
			Items    []map[string]int64 `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Code != 0 || body.Message != "ok" {
		t.Fatalf("unexpected envelope: %+v", body)
	}
	if body.Data.Page != 2 || body.Data.PageSize != 10 || body.Data.Total != 35 {
		t.Fatalf("unexpected page payload: %+v", body.Data)
	}
	if len(body.Data.Items) != 1 || body.Data.Items[0]["id"] != 1 {
		t.Fatalf("unexpected items: %+v", body.Data.Items)
	}
}

func TestWritePageResponseWithMetaMergesExtraFields(t *testing.T) {
	resp := httptest.NewRecorder()
	writePageResponseWithMeta(resp, 1, 20, 6, []int{1, 2}, map[string]any{
		"current_template": "tpl-a",
		"tree":             true,
	})

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Data["current_template"] != "tpl-a" {
		t.Fatalf("missing current_template in merged payload: %+v", body.Data)
	}
	if body.Data["tree"] != true {
		t.Fatalf("missing tree in merged payload: %+v", body.Data)
	}
}

func TestWriteItemsResponseUsesUnifiedItemsShape(t *testing.T) {
	resp := httptest.NewRecorder()
	writeItemsResponse(resp, []map[string]any{{"id": 9}}, map[string]any{"tree": true})

	var body struct {
		Code int `json:"code"`
		Data struct {
			Tree  bool             `json:"tree"`
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Code != 0 || !body.Data.Tree || len(body.Data.Items) != 1 {
		t.Fatalf("unexpected items envelope: %+v", body)
	}
}
