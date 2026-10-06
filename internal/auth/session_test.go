package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractAdminSessionTokenFromBearerHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/admin/auth/session", nil)
	req.Header.Set("Authorization", "Bearer test-token-123")

	if got := ExtractAdminSessionToken(req); got != "test-token-123" {
		t.Fatalf("expected bearer token, got %q", got)
	}
}

func TestExtractAdminSessionTokenFromCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/admin/auth/session", nil)
	req.AddCookie(cookie("admin_session", "cookie-token-456"))

	if got := ExtractAdminSessionToken(req); got != "cookie-token-456" {
		t.Fatalf("expected cookie token, got %q", got)
	}
}

func TestExtractAdminSessionTokenPrefersAuthorizationHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/admin/auth/session", nil)
	req.Header.Set("Authorization", "bearer header-token")
	req.AddCookie(cookie("admin_session", "cookie-token"))

	if got := ExtractAdminSessionToken(req); got != "header-token" {
		t.Fatalf("expected header token, got %q", got)
	}
}

func cookie(name string, value string) *http.Cookie {
	return &http.Cookie{Name: name, Value: value}
}
