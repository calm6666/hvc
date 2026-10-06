package mysql

import "testing"

func TestHashAdminPasswordMatchesBootstrapSHA256(t *testing.T) {
	if got := HashAdminPassword("admin123"); got != "240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9" {
		t.Fatalf("unexpected password hash: %s", got)
	}
}
