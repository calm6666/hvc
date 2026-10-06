package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hvc/internal/infra/db/mysql"
)

func TestResolveAdminPasswordHashPrefersPlainPassword(t *testing.T) {
	hash, salt, err := resolveAdminPasswordCredential("admin123", "legacy-hash")
	if err != nil {
		t.Fatalf("resolve password hash failed: %v", err)
	}
	if salt == "" {
		t.Fatal("expected generated salt for plain password")
	}
	if hash != mysql.HashAdminPasswordWithSalt("admin123", salt) {
		t.Fatalf("expected salted password hash, got %q", hash)
	}
}

func TestResolveAdminPasswordHashFallsBackToLegacyHash(t *testing.T) {
	hash, salt, err := resolveAdminPasswordCredential("", "legacy-hash")
	if err != nil {
		t.Fatalf("resolve password hash failed: %v", err)
	}
	if salt != "" {
		t.Fatalf("expected empty salt for legacy hash fallback, got %q", salt)
	}
	if hash != "legacy-hash" {
		t.Fatalf("expected legacy hash fallback, got %q", hash)
	}
}

func TestBindUserRoleRejectsMissingRoleID(t *testing.T) {
	handler := &WriteRBAC{}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/system/user-role/bind", strings.NewReader(`{"admin_user_id":2}`))
	rec := httptest.NewRecorder()

	handler.BindUserRole(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestBindRolePermissionRejectsMissingPermID(t *testing.T) {
	handler := &WriteRBAC{}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/system/role-permission/bind", strings.NewReader(`{"role_id":2}`))
	rec := httptest.NewRecorder()

	handler.BindRolePermission(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpsertMenuRejectsMissingMenuKey(t *testing.T) {
	handler := &WriteRBAC{}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/system/menu/upsert", strings.NewReader(`{"menu_name":"菜单","menu_type":"menu"}`))
	rec := httptest.NewRecorder()

	handler.UpsertMenu(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
