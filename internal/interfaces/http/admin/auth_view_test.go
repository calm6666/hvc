package admin

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

func TestWhoAmICompatibleMenuTreeShape(t *testing.T) {
	resp := httptest.NewRecorder()
	menuTree := []adminMenuNode{{ID: 100, MenuID: 100, ParentID: 0, MenuKey: "system", MenuName: "系统管理", Component: "Layout"}}
	logx.WriteJSON(resp, 200, model.Response{
		Code:    0,
		Message: "ok",
		Data: map[string]any{
			"authenticated": true,
			"admin_user_id": uint64(1),
			"tree":          true,
			"items":         menuTree,
			"menu_tree":     menuTree,
		},
	})

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Data["tree"] != true {
		t.Fatalf("expected tree marker, got %+v", body.Data)
	}
	if _, ok := body.Data["items"]; !ok {
		t.Fatalf("expected items field, got %+v", body.Data)
	}
	if _, ok := body.Data["menu_tree"]; !ok {
		t.Fatalf("expected legacy menu_tree compatibility field, got %+v", body.Data)
	}
}
