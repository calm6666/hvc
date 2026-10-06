package gate

import (
	"context"
	"fmt"
	"strings"

	"hvc/internal/model"
)

/*
 * 本文件是门禁的第四条：A8 —— 清单（MPD / m3u8）必须真的能取回、格式可识别、且覆盖每个清晰度。
 *
 * 【与 A1 的分工，避免重复实现】
 *   A1 判的是"清单**引用**的每个对象能取回且内容正确"（逐对象 GET + sha256 复算）；
 *   A8 判的是"清单**本身**可取回、可识别、且把该有的清晰度都列出来了"。
 * 两者合起来才等于"下游按清单能拉到全部正确内容"。
 *
 * 【清单 URL 挂在清晰度上，不在载荷顶层】
 * model.CompletedRendition 带三个 URL：ManifestDashURL / ManifestHLSURL / ManifestHLSVariantURL
 * （同一任务的多个清晰度会给出相同的 DASH/HLS 主清单 URL、各自不同的变体清单 URL），
 * 因此这里对 URL 去重后逐个取回，取回次数与实际清单份数一致。
 *
 * 【为什么不对 DASH 逐条拼分片 URL 去 GET】
 * 本项目的 MPD 用 SegmentTemplate 模板化（分片 URL 由模板 + 序号渲染），门禁自己去拼模板等于
 * 重新实现一遍渲染逻辑，拼错了会得到"假失败"。分片内容层面的正确性已经由 A1 用对象 Key 直接
 * 校验（那份 Key 来自分片表，不依赖模板），所以这里只判清单层面的三件事。
 */

// ManifestFetcher 门禁取回清单文本所需的最小能力。
//
// 实现方约定：HTTP 非 2xx、解析失败、超时等一律以 error 返回；不要把错误状态码当成空内容返回。
type ManifestFetcher interface {
	FetchManifest(ctx context.Context, url string) ([]byte, error)
}

// 清单容器的识别标记：DASH 是 <MPD，HLS 是 #EXTM3U。判据只要求"能识别出是这一类清单"。
const (
	dashManifestMarker = "<MPD"
	hlsManifestMarker  = "#EXTM3U"
)

// manifestKind 清单类别，决定用哪个识别标记。
type manifestKind int

const (
	manifestKindDash manifestKind = iota
	manifestKindHLS
)

// marker 返回该类别的识别标记。
func (k manifestKind) marker() string {
	if k == manifestKindDash {
		return dashManifestMarker
	}

	return hlsManifestMarker
}

// RunManifestServing A8（推导）：清单可取回、容器可识别、且每个清晰度都被列出来。
//
// 判定顺序（任何一步失败立即返回并附上具体 URL，便于定位是哪一份清单出的问题）：
//  1. 收集载荷里所有清晰度给出的清单 URL（去重），逐个取回，按类别检查容器标记；
//  2. 每个清晰度的名字必须出现在**它自己的**某一份清单里（变体清单 → HLS 主清单 → DASH 主清单
//     依次判断），否则说明清单没把这个清晰度列出来。
//
// fetcher 为 nil、或载荷里没有任何清单 URL ⇒ Skipped（不算通过）。
func RunManifestServing(ctx context.Context, fetcher ManifestFetcher, payload model.TranscodeCompletedPayload) CheckResult {
	const id = "A8"
	const title = "清单可取回、可识别且覆盖每个清晰度（推导）"

	if fetcher == nil {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "未注入清单取回器，无法判定"}
	}

	kinds := make(map[string]manifestKind)

	for _, rendition := range payload.Renditions {
		if rendition.ManifestDashURL != "" {
			kinds[rendition.ManifestDashURL] = manifestKindDash
		}

		if rendition.ManifestHLSURL != "" {
			kinds[rendition.ManifestHLSURL] = manifestKindHLS
		}

		if rendition.ManifestHLSVariantURL != "" {
			kinds[rendition.ManifestHLSVariantURL] = manifestKindHLS
		}
	}

	if len(kinds) == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "载荷里没有任何清单 URL，无法判定"}
	}

	bodies := make(map[string]string, len(kinds))

	for url, kind := range kinds {
		body, err := fetcher.FetchManifest(ctx, url)
		if err != nil {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("清单取回失败：url=%s：%v", url, err),
			}
		}

		if !strings.Contains(string(body), kind.marker()) {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("清单容器不匹配（期望含 %s）：url=%s", kind.marker(), url),
			}
		}

		bodies[url] = string(body)
	}

	for _, rendition := range payload.Renditions {
		if rendition.RenditionName == "" {
			continue
		}

		mentioned := false

		for _, url := range []string{rendition.ManifestHLSVariantURL, rendition.ManifestHLSURL, rendition.ManifestDashURL} {
			if url == "" {
				continue
			}

			if strings.Contains(bodies[url], rendition.RenditionName) {
				mentioned = true
				break
			}
		}

		if !mentioned {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("清晰度 %s 没有出现在它自己的任何一份清单里", rendition.RenditionName),
			}
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}
