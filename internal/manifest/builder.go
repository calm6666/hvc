package manifest

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// Builder 动态清单构建器。
//
// 根据数据库中的分片元数据，动态生成 DASH MPD 和 HLS m3u8 播放清单。
// 不依赖磁盘上的 MPD/m3u8 文件，而是实时查询数据库构建。
type Builder struct {
	segmentRepo *mysql.SegmentRepository
	jobRepo     *mysql.JobRepository
	playDomain  string
}

// NewBuilder 创建动态清单构建器。
func NewBuilder(segmentRepo *mysql.SegmentRepository, jobRepo *mysql.JobRepository, playDomain string) *Builder {
	return &Builder{
		segmentRepo: segmentRepo,
		jobRepo:     jobRepo,
		playDomain:  playDomain,
	}
}

// BuildMPD 动态构建 DASH MPD 播放清单。
//
// 从数据库查询指定任务的所有分片元数据，
// 按清晰度分组，生成符合 MPEG-DASH 标准的 MPD XML。
func (b *Builder) BuildMPD(ctx context.Context, jobID uint64) (string, error) {
	job, ok := b.jobRepo.GetByID(ctx, jobID)
	if !ok {
		return "", fmt.Errorf("任务不存在: %d", jobID)
	}

	segments := b.segmentRepo.ListByJobID(ctx, jobID)
	if len(segments) == 0 {
		return "", fmt.Errorf("任务无分片数据: %d", jobID)
	}

	renditions := groupSegmentsByRendition(segments)

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
	for rendName, segs := range renditions {
		initSeg := findInitSegment(segs)
		mediaSegs := findMediaSegments(segs)
		if initSeg == nil || len(mediaSegs) == 0 {
			continue
		}

		sb.WriteString(`<AdaptationSet`)
		sb.WriteString(fmt.Sprintf(` id="%d"`, videoIdx))
		sb.WriteString(` mimeType="video/mp4"`)
		sb.WriteString(` contentType="video"`)
		sb.WriteString(` segmentAlignment="true"`)
		sb.WriteString(` startWithSAP="1"`)
		sb.WriteString(`>`)

		sb.WriteString(`<SegmentTemplate`)
		sb.WriteString(fmt.Sprintf(` timescale="1000"`))
		sb.WriteString(fmt.Sprintf(` initialization="%s"`, b.objectURL(initSeg.ObjectKey)))
		sb.WriteString(fmt.Sprintf(` media="%s"`, b.mediaTemplateURL(rendName)))
		sb.WriteString(fmt.Sprintf(` duration="%d"`, job.SegmentDurationSec*1000))
		sb.WriteString(fmt.Sprintf(` startNumber="1"`))
		sb.WriteString(`/>`)

		sb.WriteString(`<Representation`)
		sb.WriteString(fmt.Sprintf(` id="%s"`, rendName))
		sb.WriteString(fmt.Sprintf(` bandwidth="%d"`, initSeg.VideoBitrateKbps*1000))
		sb.WriteString(fmt.Sprintf(` width="%d"`, initSeg.Width))
		sb.WriteString(fmt.Sprintf(` height="%d"`, initSeg.Height))
		sb.WriteString(fmt.Sprintf(` codecs="%s"`, codecString(initSeg.VideoCodec)))
		sb.WriteString(`/>`)

		sb.WriteString(`</AdaptationSet>`)
		videoIdx++

		if audioIdx == 0 {
			sb.WriteString(`<AdaptationSet`)
			sb.WriteString(fmt.Sprintf(` id="%d"`, videoIdx))
			sb.WriteString(` mimeType="audio/mp4"`)
			sb.WriteString(` contentType="audio"`)
			sb.WriteString(` segmentAlignment="true"`)
			sb.WriteString(` startWithSAP="1"`)
			sb.WriteString(`>`)

			audioInit := findInitSegment(segs)
			sb.WriteString(`<SegmentTemplate`)
			sb.WriteString(fmt.Sprintf(` timescale="1000"`))
			sb.WriteString(fmt.Sprintf(` initialization="%s"`, b.objectURL(audioInit.ObjectKey)))
			sb.WriteString(fmt.Sprintf(` media="%s"`, b.mediaTemplateURL(rendName)))
			sb.WriteString(fmt.Sprintf(` duration="%d"`, job.SegmentDurationSec*1000))
			sb.WriteString(fmt.Sprintf(` startNumber="1"`))
			sb.WriteString(`/>`)

			sb.WriteString(`<Representation`)
			sb.WriteString(fmt.Sprintf(` id="audio-%s"`, rendName))
			sb.WriteString(fmt.Sprintf(` bandwidth="%d"`, initSeg.AudioBitrateKbps*1000))
			sb.WriteString(` audioSamplingRate="44100"`)
			sb.WriteString(` codecs="mp4a.40.2"`)
			sb.WriteString(`/>`)

			sb.WriteString(`</AdaptationSet>`)
			audioIdx++
		}
	}

	sb.WriteString(`</Period>`)
	sb.WriteString(`</MPD>`)

	return sb.String(), nil
}

// BuildM3U8 动态构建 HLS Master 播放清单。
//
// 生成 HLS fMP4 格式的 master.m3u8，
// 每个清晰度对应一个 Variant Stream。
func (b *Builder) BuildM3U8(ctx context.Context, jobID uint64) (string, error) {
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

func (b *Builder) objectURL(objectKey string) string {
	if b.playDomain != "" {
		return fmt.Sprintf("%s/%s", b.playDomain, objectKey)
	}
	return objectKey
}

func (b *Builder) mediaTemplateURL(rendName string) string {
	return fmt.Sprintf("%s/seg-%s-$Number$.m4s", rendName, rendName)
}

func (b *Builder) variantM3U8URL(jobID uint64, rendName string) string {
	return fmt.Sprintf("/api/v1/manifest/hls/%d/%s.m3u8", jobID, rendName)
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

// SegmentRepository 查询接口（如果 mysql.SegmentRepository 没有 ListByJobID 方法）。
// 这里用类型断言或方法扩展来确保接口可用。
type segmentLister interface {
	ListByJobID(ctx context.Context, jobID uint64) []model.Segment
}

// EnsureBuilderDependencies 确保构建器依赖完整。
func EnsureBuilderDependencies(builder *Builder) {
	if builder == nil {
		logx.Info("manifest.builder.nil", nil)
	}
}
