package public

import (
	"net/http"
	"strconv"

	"hvc/internal/manifest"
	"hvc/pkg/logx"
)

// ManifestHandler 清单动态构建处理器。
//
// 提供以下接口：
//   - GET /api/v1/manifest/dash/{job_id}.mpd  -- 动态构建 DASH MPD
//   - GET /api/v1/manifest/hls/{job_id}.m3u8   -- 动态构建 HLS Master m3u8
//   - GET /api/v1/manifest/hls/{job_id}/{rendition}.m3u8 -- 动态构建 HLS Variant m3u8
type ManifestHandler struct {
	builder *manifest.Builder
}

// NewManifestHandler 创建清单处理器。
func NewManifestHandler(builder *manifest.Builder) *ManifestHandler {
	return &ManifestHandler{builder: builder}
}

// ServeMPD 动态构建 DASH MPD 播放清单。
//
// GET /api/v1/manifest/dash/{job_id}.mpd
func (h *ManifestHandler) ServeMPD(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseUint(r.PathValue("job_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	if h.builder == nil {
		http.Error(w, "manifest builder not available", http.StatusServiceUnavailable)
		return
	}

	mpd, err := h.builder.BuildMPD(r.Context(), jobID)
	if err != nil {
		logx.Error("manifest.mpd.build_failed", err, logx.Fields{"job_id": jobID})
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/dash+xml")
	w.Header().Set("Cache-Control", "public, max-age=5")
	w.Write([]byte(mpd))
}

// ServeMasterM3U8 动态构建 HLS Master 播放清单。
//
// GET /api/v1/manifest/hls/{job_id}.m3u8
func (h *ManifestHandler) ServeMasterM3U8(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseUint(r.PathValue("job_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	if h.builder == nil {
		http.Error(w, "manifest builder not available", http.StatusServiceUnavailable)
		return
	}

	m3u8, err := h.builder.BuildM3U8(r.Context(), jobID)
	if err != nil {
		logx.Error("manifest.m3u8.build_failed", err, logx.Fields{"job_id": jobID})
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "public, max-age=5")
	w.Write([]byte(m3u8))
}

// ServeVariantM3U8 动态构建 HLS Variant 播放清单。
//
// GET /api/v1/manifest/hls/{job_id}/{rendition}.m3u8
func (h *ManifestHandler) ServeVariantM3U8(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseUint(r.PathValue("job_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	rendition := r.PathValue("rendition")
	if rendition == "" {
		http.Error(w, "missing rendition", http.StatusBadRequest)
		return
	}

	if h.builder == nil {
		http.Error(w, "manifest builder not available", http.StatusServiceUnavailable)
		return
	}

	m3u8, err := h.builder.BuildVariantM3U8(r.Context(), jobID, rendition)
	if err != nil {
		logx.Error("manifest.variant_m3u8.build_failed", err, logx.Fields{
			"job_id":    jobID,
			"rendition": rendition,
		})
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "public, max-age=5")
	w.Write([]byte(m3u8))
}
