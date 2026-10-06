package gate

import (
	"context"
	"errors"
	"testing"

	"hvc/internal/model"
)

/* fakeFetcher 用内存里的清单表模拟"取回清单"，覆盖取得到/取不到/内容不对三种情形。 */
type fakeFetcher struct {
	bodies  map[string]string
	errs    map[string]error
	fetches int
}

func (f *fakeFetcher) FetchManifest(_ context.Context, url string) ([]byte, error) {
	f.fetches++

	if err := f.errs[url]; err != nil {
		return nil, err
	}

	body, ok := f.bodies[url]
	if !ok {
		return nil, errors.New("404 not found")
	}

	return []byte(body), nil
}

/* 清单 URL 挂在清晰度上（CompletedRendition 的三个字段），与载荷的真实结构一致。 */
func servingPayload() model.TranscodeCompletedPayload {
	return model.TranscodeCompletedPayload{
		JobID: 9,
		Renditions: []model.CompletedRendition{
			{
				RenditionName:         "720p",
				ManifestDashURL:       "/v1/manifest/dash/9",
				ManifestHLSURL:        "/v1/manifest/hls/9",
				ManifestHLSVariantURL: "/v1/manifest/hls/9/720p",
			},
		},
	}
}

func TestManifestServingPasses(t *testing.T) {
	fetcher := &fakeFetcher{
		bodies: map[string]string{
			"/v1/manifest/dash/9":     `<?xml version="1.0"?><MPD><Period><AdaptationSet><Representation id="720p"/></AdaptationSet></Period></MPD>`,
			"/v1/manifest/hls/9":      "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000\n/v1/manifest/hls/9/720p\n",
			"/v1/manifest/hls/9/720p": "#EXTM3U\n#EXT-X-TARGETDURATION=6\n#EXTINF:6.0,\n0.m4s\n",
		},
		errs: map[string]error{},
	}

	result := RunManifestServing(context.Background(), fetcher, servingPayload())

	if !result.Passed || result.Skipped {
		t.Fatalf("应通过：passed=%v skipped=%v detail=%s", result.Passed, result.Skipped, result.Detail)
	}

	if fetcher.fetches != 3 {
		t.Fatalf("三份清单（DASH 主 / HLS 主 / 变体）各取回一次，实际 %d 次", fetcher.fetches)
	}
}

func TestManifestServingFetchErrorDetected(t *testing.T) {
	fetcher := &fakeFetcher{
		bodies: map[string]string{},
		errs:   map[string]error{"/v1/manifest/dash/9": errors.New("500 internal")},
	}

	result := RunManifestServing(context.Background(), fetcher, servingPayload())

	if result.Passed || result.Skipped {
		t.Fatalf("清单取不到必须判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestManifestServingWrongContainerDetected(t *testing.T) {
	fetcher := &fakeFetcher{
		/* DASH 路由却返回了别的容器：容器不匹配必须被判出来。 */
		bodies: map[string]string{
			"/v1/manifest/dash/9":     "<html>error page</html>",
			"/v1/manifest/hls/9":      "#EXTM3U\n",
			"/v1/manifest/hls/9/720p": "#EXTM3U\n",
		},
		errs: map[string]error{},
	}

	result := RunManifestServing(context.Background(), fetcher, servingPayload())

	if result.Passed || result.Skipped {
		t.Fatalf("容器不匹配必须判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestManifestServingRenditionNotListedDetected(t *testing.T) {
	fetcher := &fakeFetcher{
		/* 三份清单都合法，但都没提到 720p ⇒ 该清晰度没被列出来。 */
		bodies: map[string]string{
			"/v1/manifest/dash/9":     "<MPD><Period/></MPD>",
			"/v1/manifest/hls/9":      "#EXTM3U\n",
			"/v1/manifest/hls/9/720p": "#EXTM3U\n",
		},
		errs: map[string]error{},
	}

	result := RunManifestServing(context.Background(), fetcher, servingPayload())

	if result.Passed || result.Skipped {
		t.Fatalf("清晰度未出现在清单里必须判失败：passed=%v skipped=%v detail=%s",
			result.Passed, result.Skipped, result.Detail)
	}
}

func TestManifestServingWithoutFetcherSkips(t *testing.T) {
	result := RunManifestServing(context.Background(), nil, servingPayload())

	if !result.Skipped || result.Passed {
		t.Fatalf("没有取回器时必须 Skipped 且不算通过：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestManifestServingWithoutURLsSkips(t *testing.T) {
	result := RunManifestServing(context.Background(), &fakeFetcher{}, model.TranscodeCompletedPayload{JobID: 9})

	if !result.Skipped || result.Passed {
		t.Fatalf("载荷没有清单 URL 时必须 Skipped：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}
