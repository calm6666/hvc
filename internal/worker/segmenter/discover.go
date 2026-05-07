package segmenter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"hvc/internal/model"
	"hvc/internal/worker/planner"
)

var (
	initSegRe  = regexp.MustCompile(`^init-(\d+)\.m4s$`)
	mediaSegRe = regexp.MustCompile(`^seg-(\d+)-(\d+)\.m4s$`)
)

// DiscoveredSegment 表示扫描发现的分片文件信息。
type DiscoveredSegment struct {
	RenditionName    string
	IsInit           bool
	SequenceNo       int
	RepresentationID int
	ObjectKey        string
	FilePath         string
	FileSize         int64
	MediaType        int
	MediaTypeStr     string
	Width            int
	Height           int
	VideoBitrateKbps int
	AudioBitrateKbps int
	VideoCodec       string
	QualityLabel     string
}

// Result 表示分片扫描结果。
type Result struct {
	Segments []DiscoveredSegment
}

// Discover 扫描 FFmpeg 输出目录，发现所有分片文件并按模板生成对象存储键。
//
// FFmpeg 管道输出的分片名使用简单格式（init-$RepresentationID$.m4s / seg-$RepresentationID$-$Number$.m4s），
// 不依赖后台配置的模板，以保证跨操作系统兼容性。
// 存入对象存储和数据库中的分片名必须按后台配置的模板命名，
// 因此在发现阶段将 FFmpeg 输出名映射为模板名。
func Discover(job model.TranscodeJob, pipeline planner.Pipeline) Result {
	dir := pipeline.OutputDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Result{}
	}

	var segments []DiscoveredSegment
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if m := initSegRe.FindStringSubmatch(name); m != nil {
			repID, _ := strconv.Atoi(m[1])
			rend, mediaTypeStr := planner.MapRepIDToRendition(repID, pipeline.Renditions)
			seqNo := 0
			objectKey := renderObjectKey(pipeline, job, rend, mediaTypeStr, seqNo, 0)
			segments = append(segments, DiscoveredSegment{
				RenditionName:    rend.Name,
				IsInit:           true,
				SequenceNo:       seqNo,
				RepresentationID: repID,
				ObjectKey:        objectKey,
				FilePath:         filepath.Join(dir, name),
				FileSize:         info.Size(),
				MediaType:        mediaTypeToInt(mediaTypeStr),
				MediaTypeStr:     mediaTypeStr,
				Width:            rend.Width,
				Height:           rend.Height,
				VideoBitrateKbps: rend.VideoBitrateKbps,
				AudioBitrateKbps: rend.AudioBitrateKbps,
				VideoCodec:       rend.VideoCodec,
				QualityLabel:     rend.QualityLabel,
			})
		} else if m := mediaSegRe.FindStringSubmatch(name); m != nil {
			repID, _ := strconv.Atoi(m[1])
			seqNo, _ := strconv.Atoi(m[2])
			rend, mediaTypeStr := planner.MapRepIDToRendition(repID, pipeline.Renditions)
			timestampMS := int64(seqNo * pipeline.SegmentDurationSec * 1000)
			objectKey := renderObjectKey(pipeline, job, rend, mediaTypeStr, seqNo, timestampMS)
			segments = append(segments, DiscoveredSegment{
				RenditionName:    rend.Name,
				IsInit:           false,
				SequenceNo:       seqNo,
				RepresentationID: repID,
				ObjectKey:        objectKey,
				FilePath:         filepath.Join(dir, name),
				FileSize:         info.Size(),
				MediaType:        mediaTypeToInt(mediaTypeStr),
				MediaTypeStr:     mediaTypeStr,
				Width:            rend.Width,
				Height:           rend.Height,
				VideoBitrateKbps: rend.VideoBitrateKbps,
				AudioBitrateKbps: rend.AudioBitrateKbps,
				VideoCodec:       rend.VideoCodec,
				QualityLabel:     rend.QualityLabel,
			})
		}
	}

	sort.Slice(segments, func(i, j int) bool {
		if segments[i].RenditionName != segments[j].RenditionName {
			return segments[i].RenditionName < segments[j].RenditionName
		}
		if segments[i].MediaTypeStr != segments[j].MediaTypeStr {
			return segments[i].MediaTypeStr < segments[j].MediaTypeStr
		}
		if segments[i].IsInit != segments[j].IsInit {
			return segments[i].IsInit
		}
		return segments[i].SequenceNo < segments[j].SequenceNo
	})

	return Result{Segments: segments}
}

// renderObjectKey 根据模板和清晰度信息生成对象存储键。
//
// 将后台配置的分片命名模板渲染为实际文件名，
// 再拼上前缀构成完整的对象存储键。
func renderObjectKey(pipeline planner.Pipeline, job model.TranscodeJob, rend planner.RenditionSpec, mediaType string, number int, timestampMS int64) string {
	name := planner.RenderSegmentName(
		pipeline.SegmentNaming.SegmentTemplate,
		pipeline.SegmentNaming.JobID,
		rend,
		mediaType,
		number,
		timestampMS,
	)
	prefix := pipeline.SegmentNaming.ObjectKeyPrefix
	if prefix != "" {
		return fmt.Sprintf("%s/%s", prefix, name)
	}
	return name
}

func mediaTypeToInt(mediaType string) int {
	if mediaType == "audio" {
		return 2
	}
	return 1
}
