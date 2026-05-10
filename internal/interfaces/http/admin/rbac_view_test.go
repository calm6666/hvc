package admin

import (
	"testing"
	"time"

	"hvc/internal/infra/db/mysql"
)

func TestBuildMenuTreeBuildsHierarchyAndSorts(t *testing.T) {
	items := []mysql.AdminMenuRecord{
		{MenuID: 2, ParentID: 1, MenuKey: "child-b", MenuName: "Child B", SortNo: 20, MenuType: "menu", Status: 1},
		{MenuID: 1, ParentID: 0, MenuKey: "root", MenuName: "Root", SortNo: 10, MenuType: "directory", Status: 1},
		{MenuID: 3, ParentID: 1, MenuKey: "child-a", MenuName: "Child A", SortNo: 10, MenuType: "menu", Status: 1},
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
}
