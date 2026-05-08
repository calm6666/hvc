package manifest

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/internal/worker/planner"
	"hvc/pkg/logx"
)

// RenditionFilter 清晰度过滤条件。
//
// 用于版权保护场景，控制清单中可见的清晰度列表。
// 支持两种过滤方式：
//   - AllowedNames：按名称白名单过滤（如 720p,480p）
//   - MaxHeight：按最大高度过滤（如 720 表示只返回 720p 及以下）
//
// 两者同时存在时取交集。两者都为空时不过滤，返回全部清晰度。
type RenditionFilter struct {
	AllowedNames []string
	MaxHeight    int
}

// IsAllowed 判断指定清晰度是否通过过滤。
func (f RenditionFilter) IsAllowed(renditionName string, height int) bool {
	if len(f.AllowedNames) == 0 && f.MaxHeight <= 0 {
		return true
	}
	nameOk := len(f.AllowedNames) == 0
	for _, n := range f.AllowedNames {
		if n == renditionName {
			nameOk = true
			break
		}
	}
	heightOk := f.MaxHeight <= 0 || height <= f.MaxHeight
	return nameOk && heightOk
}

// Builder 动态清单构建器。
//
// 根据数据库中的分片元数据，动态生成 DASH MPD 和 HLS m3u8 播放清单。
// 不依赖磁盘上的 MPD/m3u8 文件，而是实时查询数据库构建。
// 分片命名使用后台配置的模板，清单中的 URL 与对象存储中的实际文件名一致。
type Builder struct {
	segmentRepo     *mysql.SegmentRepository
	jobRepo         *mysql.JobRepository
	effectiveConfig *configcenter.EffectiveConfig
}

// NewBuilder 创建动态清单构建器。
//
// segmentTemplate 为后台配置的分片命名模板，
// 用于生成清单中 SegmentTemplate 的 media 和 initialization 属性。
func NewBuilder(segmentRepo *mysql.SegmentRepository, jobRepo *mysql.JobRepository, effectiveConfig *configcenter.EffectiveConfig) *Builder {
	return &Builder{
		segmentRepo:     segmentRepo,
		jobRepo:         jobRepo,
		effectiveConfig: effectiveConfig,
	}
}

// BuildMPD 动态构建 DASH MPD 播放清单。
//
// 从数据库查询指定任务的所有分片元数据，
// 按清晰度和媒体类型分组，生成符合 MPEG-DASH 标准的 MPD XML。
// 分片 URL 使用后台配置的命名模板渲染。
func (b *Builder) BuildMPD(ctx context.Context, jobID uint64, filter RenditionFilter) (string, error) {
	job, ok := b.jobRepo.GetByID(ctx, jobID)
	if !ok {
		return "", fmt.Errorf("任务不存在: %d", jobID)
	}

	segments := b.segmentRepo.ListByJobID(ctx, jobID)
	if len(segments) == 0 {
		return "", fmt.Errorf("任务无分片数据: %d", jobID)
	}

	renditions := groupSegmentsByRenditionAndMediaType(segments)

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	durationS := float64(0)
	for _, segs := range renditions {
		for _, seg := range segs {
			if seg.SegmentType == "media" {
				durationS += float64(seg.DurationMS) / 1000.0
			}
		}
		break
	}
	durationStr := formatDuration(durationS)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	sb.WriteString(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011"`)
	sb.WriteString(fmt.Sprintf(` minBufferTime="PT%dS"`, job.SegmentDurationSec))
	sb.WriteString(` profiles="urn:mpeg:dash:profile:isoff-live:2011"`)
	sb.WriteString(fmt.Sprintf(` type="static"`))
	sb.WriteString(fmt.Sprintf(` mediaPresentationDuration="%s"`, durationStr))
	sb.WriteString(fmt.Sprintf(` availabilityStartTime="%s"`, now))
	sb.WriteString(`>`)
	sb.WriteString(`<Period>`)

	videoIdx := 0
	audioIdx := 0

	videoRenditions := make(map[string][]model.Segment)
	audioRenditions := make(map[string][]model.Segment)
	for key, segs := range renditions {
		if strings.HasSuffix(key, "-audio") {
			audioRenditions[key] = segs
		} else {
			videoRenditions[key] = segs
		}
	}

	for rendKey, segs := range videoRenditions {
		initSeg := findInitSegment(segs)
		mediaSegs := findMediaSegments(segs)
		if initSeg == nil || len(mediaSegs) == 0 {
			continue
		}

		if !filter.IsAllowed(initSeg.RenditionName, initSeg.Height) {
			continue
		}

		rend := planner.RenditionSpec{
			Name:         initSeg.RenditionName,
			RenditionKey: initSeg.RenditionKey,
			QualityLabel: planner.QualityLabelFromHeight(initSeg.Height),
			Width:        initSeg.Width,
			Height:       initSeg.Height,
		}

		sb.WriteString(`<AdaptationSet`)
		sb.WriteString(fmt.Sprintf(` id="%d"`, videoIdx))
		sb.WriteString(` mimeType="video/mp4"`)
		sb.WriteString(` contentType="video"`)
		sb.WriteString(` segmentAlignment="true"`)
		sb.WriteString(` startWithSAP="1"`)
		sb.WriteString(`>`)

		initURL := b.objectURL(initSeg.ObjectKey)
		mediaURL := b.renderMediaTemplateURL(job.SegmentTemplate, jobID, rend, "video")

		sb.WriteString(`<SegmentTemplate`)
		sb.WriteString(fmt.Sprintf(` timescale="1000"`))
		sb.WriteString(fmt.Sprintf(` initialization="%s"`, initURL))
		sb.WriteString(fmt.Sprintf(` media="%s"`, mediaURL))
		sb.WriteString(fmt.Sprintf(` duration="%d"`, job.SegmentDurationSec*1000))
		sb.WriteString(fmt.Sprintf(` startNumber="1"`))
		sb.WriteString(`/>`)

		sb.WriteString(`<Representation`)
		sb.WriteString(fmt.Sprintf(` id="%s"`, rendKey))
		sb.WriteString(fmt.Sprintf(` bandwidth="%d"`, initSeg.VideoBitrateKbps*1000))
		sb.WriteString(fmt.Sprintf(` width="%d"`, initSeg.Width))
		sb.WriteString(fmt.Sprintf(` height="%d"`, initSeg.Height))
		sb.WriteString(fmt.Sprintf(` codecs="%s"`, codecString(initSeg.VideoCodec)))
		sb.WriteString(`/>`)

		sb.WriteString(`</AdaptationSet>`)
		videoIdx++
	}

	for rendKey, segs := range audioRenditions {
		initSeg := findInitSegment(segs)
		if initSeg == nil {
			continue
		}

		if !filter.IsAllowed(initSeg.RenditionName, initSeg.Height) {
			continue
		}

		videoSegs, ok := videoRenditions[strings.TrimSuffix(rendKey, "-audio")+"-video"]
		var rend planner.RenditionSpec
		if ok {
			videoInit := findInitSegment(videoSegs)
			if videoInit != nil {
				rend = planner.RenditionSpec{
					Name:         videoInit.RenditionName,
					RenditionKey: videoInit.RenditionKey,
					QualityLabel: planner.QualityLabelFromHeight(videoInit.Height),
					Width:        videoInit.Width,
					Height:       videoInit.Height,
				}
			}
		}
		if rend.QualityLabel == "" {
			rend = planner.RenditionSpec{
				Name:         initSeg.RenditionName,
				RenditionKey: initSeg.RenditionKey,
				QualityLabel: planner.QualityLabelFromHeight(initSeg.Height),
				Width:        initSeg.Width,
				Height:       initSeg.Height,
			}
		}

		sb.WriteString(`<AdaptationSet`)
		sb.WriteString(fmt.Sprintf(` id="%d"`, videoIdx+audioIdx))
		sb.WriteString(` mimeType="audio/mp4"`)
		sb.WriteString(` contentType="audio"`)
		sb.WriteString(` segmentAlignment="true"`)
		sb.WriteString(` startWithSAP="1"`)
		sb.WriteString(`>`)

		initURL := b.objectURL(initSeg.ObjectKey)
		mediaURL := b.renderMediaTemplateURL(job.SegmentTemplate, jobID, rend, "audio")

		sb.WriteString(`<SegmentTemplate`)
		sb.WriteString(fmt.Sprintf(` timescale="1000"`))
		sb.WriteString(fmt.Sprintf(` initialization="%s"`, initURL))
		sb.WriteString(fmt.Sprintf(` media="%s"`, mediaURL))
		sb.WriteString(fmt.Sprintf(` duration="%d"`, job.SegmentDurationSec*1000))
		sb.WriteString(fmt.Sprintf(` startNumber="1"`))
		sb.WriteString(`/>`)

		sb.WriteString(`<Representation`)
		sb.WriteString(fmt.Sprintf(` id="%s"`, rendKey))
		sb.WriteString(fmt.Sprintf(` bandwidth="%d"`, initSeg.AudioBitrateKbps*1000))
		sb.WriteString(` audioSamplingRate="44100"`)
		sb.WriteString(` codecs="mp4a.40.2"`)
		sb.WriteString(`/>`)

		sb.WriteString(`</AdaptationSet>`)
		audioIdx++
	}

	sb.WriteString(`</Period>`)
	sb.WriteString(`</MPD>`)

	return sb.String(), nil
}

// BuildM3U8 动态构建 HLS Master 播放清单。
//
// 生成 HLS fMP4 格式的 master.m3u8，
// 每个清晰度对应一个 Variant Stream。
func (b *Builder) BuildM3U8(ctx context.Context, jobID uint64, filter RenditionFilter) (string, error) {
	job, ok := b.jobRepo.GetByID(ctx, jobID)
	if !ok {
		return "", fmt.Errorf("任务不存在: %d", jobID)
	}
	_ = job

	segments := b.segmentRepo.ListByJobID(ctx, jobID)
	if len(segments) == 0 {
		return "", fmt.Errorf("任务无分片数据: %d", jobID)
	}

	renditions := groupSegmentsByRendition(segments)

	var sb strings.Builder
	sb.WriteString(`#EXTM3U`)
	sb.WriteString("\n")
	sb.WriteString(`#EXT-X-VERSION:7`)
	sb.WriteString("\n")
	sb.WriteString(`#EXT-X-INDEPENDENT-SEGMENTS`)
	sb.WriteString("\n")

	for rendName, segs := range renditions {
		initSeg := findInitSegment(segs)
		if initSeg == nil {
			continue
		}

		if !filter.IsAllowed(rendName, initSeg.Height) {
			continue
		}
		bandwidth := initSeg.VideoBitrateKbps * 1000
		resolution := fmt.Sprintf("%dx%d", initSeg.Width, initSeg.Height)
		codec := codecString(initSeg.VideoCodec)

		sb.WriteString(fmt.Sprintf(`#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s,CODECS="%s,mp4a.40.2"`, bandwidth, resolution, codec))
		sb.WriteString("\n")
		sb.WriteString(b.variantM3U8URL(jobID, rendName))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// BuildVariantM3U8 动态构建 HLS Variant 播放清单。
//
// 生成单个清晰度的 fMP4 m3u8，
// 包含 init segment 和所有 media segment。
func (b *Builder) BuildVariantM3U8(ctx context.Context, jobID uint64, renditionName string) (string, error) {
	segments := b.segmentRepo.ListByJobID(ctx, jobID)
	if len(segments) == 0 {
		return "", fmt.Errorf("任务无分片数据: %d", jobID)
	}

	renditions := groupSegmentsByRendition(segments)
	segs, ok := renditions[renditionName]
	if !ok {
		return "", fmt.Errorf("清晰度不存在: %s", renditionName)
	}

	initSeg := findInitSegment(segs)
	mediaSegs := findMediaSegments(segs)
	if initSeg == nil || len(mediaSegs) == 0 {
		return "", fmt.Errorf("清晰度 %s 无分片数据", renditionName)
	}

	var sb strings.Builder
	sb.WriteString(`#EXTM3U`)
	sb.WriteString("\n")
	sb.WriteString(`#EXT-X-VERSION:7`)
	sb.WriteString("\n")
	sb.WriteString(`#EXT-X-TARGETDURATION:6`)
	sb.WriteString("\n")
	sb.WriteString(`#EXT-X-MAP:URI="`)
	sb.WriteString(b.objectURL(initSeg.ObjectKey))
	sb.WriteString(`"`)
	sb.WriteString("\n")

	for _, seg := range mediaSegs {
		durationS := float64(seg.DurationMS) / 1000.0
		sb.WriteString(fmt.Sprintf(`#EXTINF:%.3f,`, durationS))
		sb.WriteString("\n")
		sb.WriteString(b.objectURL(seg.ObjectKey))
		sb.WriteString("\n")
	}

	sb.WriteString(`#EXT-X-ENDLIST`)
	sb.WriteString("\n")

	return sb.String(), nil
}

// renderMediaTemplateURL 根据命名模板渲染 DASH SegmentTemplate 的 media 属性 URL。
//
// 使用 $Number$ 作为 FFmpeg/DASH 标准占位符，
// 其余占位符按后台配置模板渲染。
func (b *Builder) renderMediaTemplateURL(template string, jobID uint64, rend planner.RenditionSpec, mediaType string) string {
	tmpl := template
	if tmpl == "" {
		tmpl = "{job_id}-{rendition_key}-{media_type}-{number}.m4s"
	}
	result := tmpl
	result = strings.ReplaceAll(result, "{job_id}", fmt.Sprintf("%d", jobID))
	result = strings.ReplaceAll(result, "{rendition_key}", rend.RenditionKey)
	result = strings.ReplaceAll(result, "{media_type}", mediaType)
	result = strings.ReplaceAll(result, "{number}", "$Number$")
	result = strings.ReplaceAll(result, "{resolution}", rend.Resolution())
	result = strings.ReplaceAll(result, "{quality}", rend.QualityLabel)
	result = strings.ReplaceAll(result, "{timestamp}", "$Time$")
	return b.objectURL(result)
}

func (b *Builder) objectURL(objectKey string) string {
	playDomain := b.currentConfig().Storage.PlayDomain
	if playDomain != "" {
		return fmt.Sprintf("%s/%s", playDomain, objectKey)
	}
	return objectKey
}

func (b *Builder) variantM3U8URL(jobID uint64, rendName string) string {
	return fmt.Sprintf("/v1/manifest/hls/%d/%s.m3u8", jobID, rendName)
}

func groupSegmentsByRendition(segments []model.Segment) map[string][]model.Segment {
	result := make(map[string][]model.Segment)
	for _, seg := range segments {
		name := seg.RenditionName
		if name == "" {
			name = "source"
		}
		result[name] = append(result[name], seg)
	}
	return result
}

// groupSegmentsByRenditionAndMediaType 按清晰度和媒体类型分组。
//
// 键格式为 "renditionName-media-video" 或 "renditionName-media-audio"，
// 用于 MPD 生成时区分视频和音频 AdaptationSet。
func groupSegmentsByRenditionAndMediaType(segments []model.Segment) map[string][]model.Segment {
	result := make(map[string][]model.Segment)
	for _, seg := range segments {
		name := seg.RenditionName
		if name == "" {
			name = "source"
		}
		mediaTypeStr := "video"
		if seg.MediaType == 2 {
			mediaTypeStr = "audio"
		}
		key := fmt.Sprintf("%s-%s", name, mediaTypeStr)
		result[key] = append(result[key], seg)
	}
	return result
}

func findInitSegment(segments []model.Segment) *model.Segment {
	for _, seg := range segments {
		if seg.SegmentType == "init" {
			return &seg
		}
	}
	return nil
}

func findMediaSegments(segments []model.Segment) []model.Segment {
	result := make([]model.Segment, 0)
	for _, seg := range segments {
		if seg.SegmentType == "media" {
			result = append(result, seg)
		}
	}
	return result
}

func formatDuration(seconds float64) string {
	h := int(seconds / 3600)
	m := int(math.Mod(seconds, 3600) / 60)
	s := math.Mod(seconds, 60)
	return fmt.Sprintf("PT%dH%dM%.3fS", h, m, s)
}

func codecString(videoCodec string) string {
	switch strings.ToLower(videoCodec) {
	case "h264", "libx264":
		return "avc1.64001f"
	case "hevc", "libx265", "h265":
		return "hev1.1.6.L93.B0"
	default:
		return "avc1.64001f"
	}
}

type segmentLister interface {
	ListByJobID(ctx context.Context, jobID uint64) []model.Segment
}

// EnsureBuilderDependencies 确保 Builder 依赖项已正确初始化。
func EnsureBuilderDependencies(builder *Builder) {
	if builder == nil {
		logx.Info("manifest.builder.nil", nil)
	}
}

func (b *Builder) currentConfig() config.DynamicRuntimeConfig {
	if b.effectiveConfig == nil {
		return config.DynamicRuntimeConfig{}
	}
	return b.effectiveConfig.Snapshot()
}
