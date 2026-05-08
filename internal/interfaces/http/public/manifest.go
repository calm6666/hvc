package public

import (
	"net/http"
	"strconv"
	"strings"

	"hvc/internal/manifest"
)

// ManifestHandler 清单动态生成处理器。
//
// 提供 DASH MPD 和 HLS m3u8 的动态生成接口。
// 支持通过查询参数控制可见清晰度列表，实现版权保护：
//   - renditions 参数：逗号分隔的清晰度名称白名单（如 1080p,720p）
//   - max_height 参数：最大允许高度（如 720 表示只返回 720p 及以下）
//   - 不传参数则返回全部清晰度
type ManifestHandler struct {
	builder *manifest.Builder
}

// NewManifestHandler 创建清单处理器。
func NewManifestHandler(builder *manifest.Builder) *ManifestHandler {
	return &ManifestHandler{builder: builder}
}

// ServeMPD 动态构建 DASH MPD 播放清单。
//
// GET /v1/manifest/dash/{job_id}.mpd
//
// 查询参数：
//   - renditions: 逗号分隔的清晰度白名单（版权保护，如 renditions=720p,480p）
//   - max_height: 最大允许高度（如 max_height=720）
func (h *ManifestHandler) ServeMPD(w http.ResponseWriter, r *http.Request) {
	jobID, err := parsePathUintWithSuffix(r, "job_id", ".mpd")
	if err != nil || jobID == 0 {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	filter := parseRenditionFilter(r)

	mpd, err := h.builder.BuildMPD(r.Context(), jobID, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/dash+xml")
	w.Header().Set("Cache-Control", "public, max-age=5")
	w.Write([]byte(mpd))
}

// ServeMasterM3U8 动态构建 HLS Master 播放清单。
//
// GET /v1/manifest/hls/{job_id}.m3u8
//
// 查询参数：
//   - renditions: 逗号分隔的清晰度白名单
//   - max_height: 最大允许高度
func (h *ManifestHandler) ServeMasterM3U8(w http.ResponseWriter, r *http.Request) {
	jobID, err := parsePathUintWithSuffix(r, "job_id", ".m3u8")
	if err != nil || jobID == 0 {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	filter := parseRenditionFilter(r)

	m3u8, err := h.builder.BuildM3U8(r.Context(), jobID, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "public, max-age=5")
	w.Write([]byte(m3u8))
}

// ServeVariantM3U8 动态构建 HLS Variant 播放清单。
//
// GET /v1/manifest/hls/{job_id}/{rendition}.m3u8
func (h *ManifestHandler) ServeVariantM3U8(w http.ResponseWriter, r *http.Request) {
	jobID, err := parsePathUintWithSuffix(r, "job_id", "")
	if err != nil || jobID == 0 {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	rendition := parsePathStringWithSuffix(r, "rendition", ".m3u8")
	if rendition == "" {
		http.Error(w, "invalid rendition", http.StatusBadRequest)
		return
	}

	m3u8, err := h.builder.BuildVariantM3U8(r.Context(), jobID, rendition)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "public, max-age=5")
	w.Write([]byte(m3u8))
}

// parseRenditionFilter 从请求参数解析清晰度过滤条件。
//
// 支持两种过滤方式：
//   - renditions=720p,480p  按名称白名单过滤
//   - max_height=720        按最大高度过滤
//
// 两者同时存在时取交集。
func parseRenditionFilter(r *http.Request) manifest.RenditionFilter {
	filter := manifest.RenditionFilter{}

	renditionsParam := r.URL.Query().Get("renditions")
	if renditionsParam != "" {
		for _, r := range strings.Split(renditionsParam, ",") {
			name := strings.TrimSpace(r)
			if name != "" {
				filter.AllowedNames = append(filter.AllowedNames, name)
			}
		}
	}

	maxHeightParam := r.URL.Query().Get("max_height")
	if maxHeightParam != "" {
		if h, err := strconv.Atoi(maxHeightParam); err == nil && h > 0 {
			filter.MaxHeight = h
		}
	}

	return filter
}

// getPathValue 从 URL 路径中获取命名参数。
func getPathValue(r *http.Request, key string) string {
	return r.PathValue(key)
}

func parsePathUintWithSuffix(r *http.Request, key string, suffix string) (uint64, error) {
	value := parsePathStringWithSuffix(r, key, suffix)
	return strconv.ParseUint(value, 10, 64)
}

func parsePathStringWithSuffix(r *http.Request, key string, suffix string) string {
	value := strings.TrimSpace(getPathValue(r, key))
	if suffix == "" {
		return value
	}
	if !strings.HasSuffix(value, suffix) {
		return ""
	}
	return strings.TrimSuffix(value, suffix)
}
