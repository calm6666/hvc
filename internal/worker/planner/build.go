package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ffprobe "hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/model"
)

var hwEncoderMap = map[string]map[string]string{
	model.ExecutionHWNVIDIA:             {"h264": "h264_nvenc", "hevc": "hevc_nvenc"},
	model.ExecutionHWIntelQSV:          {"h264": "h264_qsv", "hevc": "hevc_qsv"},
	model.ExecutionHWAMDAMF:            {"h264": "h264_amf", "hevc": "hevc_amf"},
	model.ExecutionHWVAAPI:             {"h264": "h264_vaapi", "hevc": "hevc_vaapi"},
	model.ExecutionHWAppleVideoToolbox: {"h264": "h264_videotoolbox", "hevc": "hevc_videotoolbox"},
}

var hwDecoderMap = map[string]map[string]string{
	model.ExecutionHWNVIDIA:             {"h264": "h264_cuvid", "hevc": "hevc_cuvid"},
	model.ExecutionHWIntelQSV:          {"h264": "h264_qsv", "hevc": "hevc_qsv"},
	model.ExecutionHWAMDAMF:            {"h264": "h264_d3d11va", "hevc": "hevc_d3d11va"},
	model.ExecutionHWVAAPI:             {"h264": "h264_vaapi", "hevc": "hevc_vaapi"},
	model.ExecutionHWAppleVideoToolbox: {"h264": "h264_videotoolbox", "hevc": "hevc_videotoolbox"},
}

var hwAccelFlagMap = map[string]string{
	model.ExecutionHWNVIDIA:             "cuda",
	model.ExecutionHWIntelQSV:          "qsv",
	model.ExecutionHWAMDAMF:            "d3d11va",
	model.ExecutionHWVAAPI:             "vaapi",
	model.ExecutionHWAppleVideoToolbox: "videotoolbox",
}

func buildRenditions(job model.TranscodeJob, probeResult ffprobe.Result, executionHW string) []RenditionSpec {
	sourceWidth := probeResult.Width
	sourceHeight := probeResult.Height
	sourceAspect := float64(sourceWidth) / float64(sourceHeight)

	specs := []RenditionSpec{
		{
			Name:             "source",
			Width:            sourceWidth,
			Height:           sourceHeight,
			VideoCodec:       selectEncoder("h264", executionHW),
			VideoBitrateKbps: probeResult.VideoBitrateKbps,
			Preset:           selectPreset(executionHW),
			GOPSize:          probeResult.GOPSize,
			FPS:              probeResult.FPS,
			AudioCodec:       "aac",
			AudioBitrateKbps: probeResult.AudioBitrateKbps,
			AudioChannels:    probeResult.AudioChannels,
			AudioSampleRate:  probeResult.AudioSampleRate,
		},
	}

	ladderPresets := []struct {
		Name             string
		TargetHeight     int
		VideoBitrateKbps int
		AudioBitrateKbps int
	}{
		{Name: "1080p", TargetHeight: 1080, VideoBitrateKbps: 5000, AudioBitrateKbps: 128},
		{Name: "720p", TargetHeight: 720, VideoBitrateKbps: 2800, AudioBitrateKbps: 128},
		{Name: "480p", TargetHeight: 480, VideoBitrateKbps: 1400, AudioBitrateKbps: 128},
		{Name: "360p", TargetHeight: 360, VideoBitrateKbps: 800, AudioBitrateKbps: 96},
	}

	for _, preset := range ladderPresets {
		if sourceHeight < preset.TargetHeight {
			continue
		}

		targetHeight := preset.TargetHeight
		targetWidth := CalculateWidth(targetHeight, sourceAspect)

		spec := RenditionSpec{
			Name:             preset.Name,
			Width:            targetWidth,
			Height:           targetHeight,
			VideoCodec:       selectEncoder("h264", executionHW),
			VideoBitrateKbps: preset.VideoBitrateKbps,
			Preset:           selectPreset(executionHW),
			GOPSize:          0,
			FPS:              probeResult.FPS,
			AudioCodec:       "aac",
			AudioBitrateKbps: preset.AudioBitrateKbps,
			AudioChannels:    probeResult.AudioChannels,
			AudioSampleRate:  probeResult.AudioSampleRate,
		}
		specs = append(specs, spec)
	}

	if len(job.Renditions) > 0 {
		customSpecs := make([]RenditionSpec, 0, len(job.Renditions))
		for _, r := range job.Renditions {
			targetHeight := r.Height
			targetWidth := r.Width
			if targetWidth <= 0 && targetHeight > 0 {
				targetWidth = CalculateWidth(targetHeight, sourceAspect)
			}
			if targetHeight <= 0 && targetWidth > 0 {
				targetHeight = CalculateHeight(targetWidth, sourceAspect)
			}
			codec := r.VideoCodec
			if codec == "" {
				codec = selectEncoder("h264", executionHW)
			}
			customSpecs = append(customSpecs, RenditionSpec{
				Name:             r.Name,
				Width:            targetWidth,
				Height:           targetHeight,
				VideoCodec:       codec,
				VideoBitrateKbps: r.VideoBitrateKbps,
				VideoMaxrateKbps: r.VideoMaxrateKbps,
				VideoBufsizeKbps: r.VideoBufsizeKbps,
				Preset:           r.Preset,
				GOPSize:          0,
				FPS:              probeResult.FPS,
				AudioCodec:       "aac",
				AudioBitrateKbps: 128,
				AudioChannels:    probeResult.AudioChannels,
				AudioSampleRate:  probeResult.AudioSampleRate,
			})
		}
		return customSpecs
	}

	return specs
}

// calculateWidth 根据目标高度和源宽高比计算目标宽度。
//
// 宽度必须是偶数（FFmpeg 要求），因此向下取偶。
// 这样可以保证输出视频的宽高比与源视频一致，
// 不会出现拉伸或压缩变形。
func CalculateWidth(targetHeight int, sourceAspect float64) int {
	w := int(float64(targetHeight) * sourceAspect)
	if w%2 != 0 {
		w--
	}
	return w
}

// calculateHeight 根据目标宽度和源宽高比计算目标高度。
//
// 高度必须是偶数（FFmpeg 要求），因此向下取偶。
func CalculateHeight(targetWidth int, sourceAspect float64) int {
	h := int(float64(targetWidth) / sourceAspect)
	if h%2 != 0 {
		h--
	}
	return h
}

func selectEncoder(codec string, executionHW string) string {
	if executionHW == model.ExecutionHWSoftware {
		switch codec {
		case "hevc":
			return "libx265"
		default:
			return "libx264"
		}
	}
	if encoders, ok := hwEncoderMap[executionHW]; ok {
		if enc, ok2 := encoders[codec]; ok2 {
			return enc
		}
	}
	return "libx264"
}

func selectPreset(executionHW string) string {
	switch executionHW {
	case model.ExecutionHWNVIDIA:
		return "p4"
	case model.ExecutionHWIntelQSV:
		return "medium"
	case model.ExecutionHWAMDAMF:
		return "speed"
	default:
		return "medium"
	}
}

func BuildFFmpegArgs(pipeline Pipeline) []string {
	args := []string{"-hide_banner", "-y"}

	if pipeline.HardwareDecode && pipeline.DecodeHWAccel != "" {
		args = append(args, "-hwaccel", pipeline.DecodeHWAccel)
		if pipeline.DecodeDevice != "" {
			args = append(args, "-hwaccel_device", pipeline.DecodeDevice)
		}
		if pipeline.DecodeHWAccel == "cuda" {
			args = append(args, "-hwaccel_output_format", "cuda")
		}
	}

	args = append(args, "-i", pipeline.SourceURL)

	if pipeline.EnableWatermark && pipeline.WatermarkPath != "" {
		args = append(args, "-i", pipeline.WatermarkPath)
	}

	hasMultipleRenditions := len(pipeline.Renditions) > 1
	filterParts := make([]string, 0, len(pipeline.Renditions))

	for i, rend := range pipeline.Renditions {
		if hasMultipleRenditions {
			var filterChain string
			if pipeline.EnableWatermark && pipeline.WatermarkFilter != "" {
				wmFilter := pipeline.WatermarkFilter
				wmFilter = strings.Replace(wmFilter, "[0][wm]", fmt.Sprintf("[v%din][wm]", i), 1)
				wmFilter = strings.Replace(wmFilter, "[0:v]", fmt.Sprintf("[v%din]", i), 1)
				filterChain = fmt.Sprintf("[0:v]scale=%d:%d[v%din];%s[v%d]", rend.Width, rend.Height, i, wmFilter, i)
			} else {
				filterChain = fmt.Sprintf("[0:v]scale=%d:%d[v%d]", rend.Width, rend.Height, i)
			}
			filterParts = append(filterParts, filterChain)
		}
	}

	if hasMultipleRenditions && len(filterParts) > 0 {
		args = append(args, "-filter_complex", strings.Join(filterParts, ";"))
	} else if len(pipeline.Renditions) == 1 {
		rend := pipeline.Renditions[0]
		vfFilter := fmt.Sprintf("scale=%d:%d", rend.Width, rend.Height)
		if pipeline.EnableWatermark && pipeline.WatermarkFilter != "" {
			vfFilter = pipeline.WatermarkFilter
		}
		args = append(args, "-vf", vfFilter)
	}

	for i, rend := range pipeline.Renditions {
		if hasMultipleRenditions {
			args = append(args, "-map", fmt.Sprintf("[v%d]", i))
		} else {
			args = append(args, "-map", "0:v")
		}
		args = append(args, "-c:v", rend.VideoCodec)
		args = append(args, "-b:v", fmt.Sprintf("%dk", rend.VideoBitrateKbps))
		if rend.VideoMaxrateKbps > 0 {
			args = append(args, "-maxrate:v", fmt.Sprintf("%dk", rend.VideoMaxrateKbps))
		}
		if rend.VideoBufsizeKbps > 0 {
			args = append(args, "-bufsize:v", fmt.Sprintf("%dk", rend.VideoBufsizeKbps))
		}
		if rend.Preset != "" {
			args = append(args, "-preset", rend.Preset)
		}
		if rend.GOPSize > 0 {
			args = append(args, "-g", fmt.Sprintf("%d", rend.GOPSize))
		} else if pipeline.SegmentDurationSec > 0 && rend.FPS > 0 {
			gop := int(float64(pipeline.SegmentDurationSec) * rend.FPS)
			args = append(args, "-g", fmt.Sprintf("%d", gop))
		}
		args = append(args, "-keyint_min", fmt.Sprintf("%d", gopValue(rend, pipeline)))

		args = append(args, "-map", "0:a")
		args = append(args, "-c:a", rend.AudioCodec)
		args = append(args, "-b:a", fmt.Sprintf("%dk", rend.AudioBitrateKbps))
		if rend.AudioChannels > 0 {
			args = append(args, "-ac", fmt.Sprintf("%d", rend.AudioChannels))
		}
		if rend.AudioSampleRate > 0 {
			args = append(args, "-ar", fmt.Sprintf("%d", rend.AudioSampleRate))
		}
		_ = i
	}

	if len(pipeline.Renditions) > 0 {
		args = append(args, "-f", "dash")
		args = append(args, "-seg_duration", fmt.Sprintf("%d", pipeline.SegmentDurationSec))
		args = append(args, "-window_size", "0")
		args = append(args, "-extra_window_size", "0")
		args = append(args, "-remove_at_exit", "1")
		args = append(args, "-init_seg_name", renderSegmentTemplate(pipeline.SegmentNaming.SegmentTemplate, pipeline, RenditionSpec{}, 0))
		args = append(args, "-media_seg_name", renderSegmentTemplate(pipeline.SegmentNaming.SegmentTemplate, pipeline, RenditionSpec{}, -1))

		adaptationSets := "id=0,streams=v id=1,streams=a"
		args = append(args, "-adaptation_sets", adaptationSets)

		outPath := filepath.Join(pipeline.OutputDir, "manifest.mpd")
		args = append(args, outPath)
	} else {
		args = append(args, "-f", "null", "-")
	}

	return args
}

func gopValue(rend RenditionSpec, pipeline Pipeline) int {
	if rend.GOPSize > 0 {
		return rend.GOPSize
	}
	if pipeline.SegmentDurationSec > 0 && rend.FPS > 0 {
		return int(float64(pipeline.SegmentDurationSec) * rend.FPS)
	}
	return 60
}

func osTempDir() string {
	dir := os.TempDir()
	if dir == "" {
		dir = "/tmp"
	}
	return dir
}

// renderSegmentTemplate 渲染分片命名模板。
//
// 模板占位符：
//   - {job_id}       任务 ID
//   - {rendition}    清晰度名称
//   - {rendition_id} 清晰度序号
//   - {number}       分片序号（0=init, 1/2/3...=media）
//   - {timestamp}    分片起始时间戳
//
// 当 number=0 时，生成 init segment 名称（索引为 0）。
// 当 number=-1 时，生成 FFmpeg 的 $Number$ 占位符（用于 -media_seg_name）。
// 当 number>0 时，生成具体序号的 media segment 名称。
func renderSegmentTemplate(template string, pipeline Pipeline, rend RenditionSpec, number int) string {
	result := template
	result = strings.ReplaceAll(result, "{job_id}", "0")
	result = strings.ReplaceAll(result, "{rendition}", rend.Name)
	result = strings.ReplaceAll(result, "{rendition_id}", fmt.Sprintf("%d", 0))
	if number == -1 {
		result = strings.ReplaceAll(result, "{number}", "$Number$")
		result = strings.ReplaceAll(result, "{timestamp}", "$Time$")
	} else {
		result = strings.ReplaceAll(result, "{number}", fmt.Sprintf("%d", number))
		result = strings.ReplaceAll(result, "{timestamp}", "0")
	}
	return result
}

// buildWatermarkFilter 构建水印滤镜字符串。
//
// 支持特性：
//   - 保持比例缩放：按 WidthRatio 缩放宽度，高度自动按原始宽高比计算
//   - pad 填充：当水印宽高比与目标区域不匹配时，用透明色填充
//   - safe_margin 安全边距：水印与视频边缘的最小距离，防止被裁切
//   - 透明度控制：通过 colorchannelmixer 调整水印透明度
//   - 四角定位：anchor 1=左上 2=右上 3=左下 4=右下
//
// 滤镜链路：
//
//	[1] -> format=rgba -> colorchannelmixer(透明度) -> scale(保持比例缩放) -> pad(填充) -> [wm]
//	[0][wm] -> overlay(带安全边距定位)
func buildWatermarkFilter(job model.TranscodeJob) string {
	anchor := job.WatermarkAnchor
	xRatio := job.WatermarkXRatio
	yRatio := job.WatermarkYRatio
	wRatio := job.WatermarkWidthRatio
	opacity := job.WatermarkOpacity
	safeMargin := job.WatermarkSafeMarginRatio

	if opacity <= 0 {
		opacity = 1.0
	}
	if wRatio <= 0 {
		wRatio = 0.1
	}
	if safeMargin <= 0 {
		safeMargin = 0.02
	}

	targetWidthExpr := fmt.Sprintf("main_w*%f", wRatio)
	scaleFilter := fmt.Sprintf("scale=%s:-1", targetWidthExpr)

	padFilter := fmt.Sprintf("pad=iw+2:ih+2:1:1:color=0x00000000")

	var overlayExpr string
	safeX := fmt.Sprintf("main_w*%f", safeMargin)
	safeY := fmt.Sprintf("main_h*%f", safeMargin)

	switch anchor {
	case 1:
		overlayExpr = fmt.Sprintf("overlay=%s+%s:%s+%s", safeX, fmt.Sprintf("(%f*main_w)", xRatio), safeY, fmt.Sprintf("(%f*main_h)", yRatio))
	case 2:
		overlayExpr = fmt.Sprintf("overlay=main_w-overlay_w-%s-%s:%s+%s", safeX, fmt.Sprintf("(%f*main_w)", xRatio), safeY, fmt.Sprintf("(%f*main_h)", yRatio))
	case 3:
		overlayExpr = fmt.Sprintf("overlay=%s+%s:main_h-overlay_h-%s-%s", safeX, fmt.Sprintf("(%f*main_w)", xRatio), safeY, fmt.Sprintf("(%f*main_h)", yRatio))
	case 4:
		overlayExpr = fmt.Sprintf("overlay=main_w-overlay_w-%s-%s:main_h-overlay_h-%s-%s", safeX, fmt.Sprintf("(%f*main_w)", xRatio), safeY, fmt.Sprintf("(%f*main_h)", yRatio))
	default:
		overlayExpr = fmt.Sprintf("overlay=main_w-overlay_w-%s-%s:%s+%s", safeX, fmt.Sprintf("(%f*main_w)", xRatio), safeY, fmt.Sprintf("(%f*main_h)", yRatio))
	}

	return fmt.Sprintf("[1]format=rgba,colorchannelmixer=aa=%f,%s,%s[wm];[0][wm]%s", opacity, scaleFilter, padFilter, overlayExpr)
}
