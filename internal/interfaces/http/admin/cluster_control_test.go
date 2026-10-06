package admin

import "testing"

func TestResolveInternalGRPCTargetUsesHostIPForWildcardHost(t *testing.T) {
	target, err := resolveInternalGRPCTarget("10.0.0.8", ":19090")
	if err != nil {
		t.Fatalf("resolve target failed: %v", err)
	}
	if target != "10.0.0.8:19090" {
		t.Fatalf("unexpected target: %s", target)
	}
}

func TestResolveInternalGRPCTargetKeepsExplicitHost(t *testing.T) {
	target, err := resolveInternalGRPCTarget("10.0.0.8", "127.0.0.1:19090")
	if err != nil {
		t.Fatalf("resolve target failed: %v", err)
	}
	if target != "127.0.0.1:19090" {
		t.Fatalf("unexpected target: %s", target)
	}
}

func TestResolveInternalGRPCTargetRejectsEmptyHost(t *testing.T) {
	if _, err := resolveInternalGRPCTarget("", ":19090"); err == nil {
		t.Fatal("expected empty host ip to fail")
	}
}
