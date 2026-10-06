package admin

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"hvc/internal/infra/db/mysql"
)

func TestBuildMenuTreeBuildsHierarchyAndSorts(t *testing.T) {
	items := []mysql.AdminMenuRecord{
		{MenuID: 2, ParentID: 1, MenuKey: "child-b", MenuName: "Child B", SortNo: 20, MenuType: "menu", Status: 1, Component: "system/user/index"},
		{MenuID: 1, ParentID: 0, MenuKey: "root", MenuName: "Root", SortNo: 10, MenuType: "directory", Status: 1, Component: "Layout"},
		{MenuID: 3, ParentID: 1, MenuKey: "child-a", MenuName: "Child A", SortNo: 10, MenuType: "menu", Status: 1, Component: "system/role/index"},
	}

	tree := buildMenuTree(items)
	if len(tree) != 1 {
		t.Fatalf("expected one root node, got %d", len(tree))
	}
	if tree[0].MenuKey != "root" {
		t.Fatalf("unexpected root key: %s", tree[0].MenuKey)
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("expected two child nodes, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].MenuKey != "child-a" || tree[0].Children[1].MenuKey != "child-b" {
		t.Fatalf("children not sorted as expected: %+v", tree[0].Children)
	}
	if tree[0].ID != tree[0].MenuID || tree[0].Children[0].ParentID != tree[0].MenuID {
		t.Fatalf("expected menu tree nodes to expose id/parent_id, got %+v", tree[0])
	}
	if tree[0].Component != "Layout" || tree[0].Children[0].Component == "" {
		t.Fatalf("expected menu tree nodes to expose component path, got %+v", tree[0])
	}
}

func TestToAdminUserViewKeepsOnlyFrontendFields(t *testing.T) {
	lastLoginAt := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	record := mysql.AdminUserRecord{
		AdminUserID:  11,
		Username:     "admin",
		PasswordHash: "secret-hash",
		PasswordSalt: "secret-salt",
		DisplayName:  "管理员",
		Status:       1,
		LastLoginAt:  lastLoginAt,
		LastLoginIP:  "127.0.0.1",
		CreatedAt:    lastLoginAt,
		UpdatedAt:    lastLoginAt,
	}

	view := toAdminUserView(record)
	if view.AdminUserID != 11 || view.Username != "admin" {
		t.Fatalf("unexpected user view: %+v", view)
	}
	if view.LastLoginAt == nil || !view.LastLoginAt.Equal(lastLoginAt) {
		t.Fatalf("expected last_login_at to be preserved, got %+v", view.LastLoginAt)
	}
}

func TestBuildPermissionTreeGroupsByModuleAndSegments(t *testing.T) {
	items := []mysql.AdminPermissionRecord{
		{PermID: 1, PermKey: "system.user.read", PermName: "查看用户", Module: "system"},
		{PermID: 2, PermKey: "system.role.read", PermName: "查看角色", Module: "system"},
		{PermID: 3, PermKey: "cluster.read", PermName: "查看集群", Module: "cluster"},
	}

	tree := buildPermissionTree(items)
	if len(tree) != 2 {
		t.Fatalf("expected two module roots, got %d", len(tree))
	}
	if tree[0].Key != "cluster" || tree[1].Key != "system" {
		t.Fatalf("unexpected module ordering: %+v", tree)
	}
	if len(tree[1].Children) == 0 {
		t.Fatalf("expected system module to contain children, got %+v", tree[1])
	}
	if tree[0].ID == "" || tree[1].ID == "" {
		t.Fatalf("expected permission tree nodes to expose stable ids, got %+v", tree)
	}
	if tree[1].Children[0].ParentID != tree[1].ID {
		t.Fatalf("expected permission child parent_id to point to parent node id, got %+v", tree[1].Children[0])
	}
}

func TestWriteItemsResponseSupportsRoleMenuTreeUnifiedShape(t *testing.T) {
	resp := httptest.NewRecorder()
	writeItemsResponse(resp, []adminMenuNode{{MenuID: 101, MenuKey: "dashboard", MenuName: "Dashboard"}}, map[string]any{
		"tree":     true,
		"role_id":  uint64(2),
		"menu_ids": []uint64{101, 102},
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
		t.Fatalf("expected unified items field, got %+v", body.Data)
	}
	if _, ok := body.Data["menu_tree"]; ok {
		t.Fatalf("legacy menu_tree field should not exist, got %+v", body.Data)
	}
}
