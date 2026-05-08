package admin

import (
	"testing"

	"hvc/internal/infra/db/mysql"
)

func TestBuildConfigCenterBindingView(t *testing.T) {
	view := buildConfigCenterBindingView(mysql.ConfigCenterBindingRecord{
		BindingID:    1,
		BindingName:  "bootstrap-source",
		ProviderType: "nacos",
	})

	if view.ConfigScope != configCenterBindingScopeBootstrap {
		t.Fatalf("unexpected config scope: %s", view.ConfigScope)
	}
	if view.AffectsRuntime {
		t.Fatal("bootstrap config source binding must not be marked as runtime-affecting")
	}
	if len(view.BindingUsage) == 0 {
		t.Fatal("binding usage should not be empty")
	}
}
