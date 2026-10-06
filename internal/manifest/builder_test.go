package manifest

import "testing"

func TestVariantM3U8URLUsesExtensionlessRoute(t *testing.T) {
	builder := &Builder{}
	got := builder.variantM3U8URL(123, "1080p")
	if got != "/v1/manifest/hls/123/1080p" {
		t.Fatalf("unexpected variant url: %s", got)
	}
}
