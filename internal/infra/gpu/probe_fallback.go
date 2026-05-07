package gpu

import (
	"strings"

	"hvc/internal/infra/gpu/amf"
	"hvc/internal/infra/gpu/qsv"
	"hvc/internal/model"
)

// probeFallback 基于 ffmpeg 能力输出做通用探测。
//
// 这是当前批次最重要的兜底路径：
// - 即便没有 nvidia-smi、vainfo、system_profiler 等平台专用命令；
// - 只要 ffmpeg 可用；
// - 仍然能提取出最小可用的执行模式与 codec 能力集合。
//
// 需要特别注意：
// 1. fallback 只能尽量还原“支持哪些执行路径和编码器”；
// 2. 它通常拿不到精确显存占用、驱动版本和真实最大会话数；
// 3. 因此 fallback 的重点是保证调度方向不丢失，而不是替代完整硬件监控。
func probeFallback() []ProbeResult {
	hwaccelsOutput := strings.ToLower(string(ffmpegCapabilityOutput("-hide_banner", "-hwaccels")))
	encodersOutput := strings.ToLower(string(ffmpegCapabilityOutput("-hide_banner", "-encoders")))
	decodersOutput := strings.ToLower(string(ffmpegCapabilityOutput("-hide_banner", "-decoders")))
	filtersOutput := strings.ToLower(string(ffmpegCapabilityOutput("-hide_banner", "-filters")))

	result := ProbeResult{
		GPUIndex:       0,
		MaxSessions:    0,
		SupportsFilter: strings.Contains(filtersOutput, "overlay_cuda") || strings.Contains(filtersOutput, "scale_cuda") || strings.Contains(filtersOutput, "overlay_qsv") || strings.Contains(filtersOutput, "scale_qsv") || strings.Contains(filtersOutput, "overlay_vaapi") || strings.Contains(filtersOutput, "scale_vaapi") || strings.Contains(filtersOutput, "overlay_vulkan") || strings.Contains(filtersOutput, "overlay_opencl") || strings.Contains(filtersOutput, "overlay_metal") || strings.Contains(filtersOutput, "overlay_videotoolbox"),
	}

	if strings.Contains(hwaccelsOutput, "cuda") || strings.Contains(encodersOutput, "nvenc") || strings.Contains(decodersOutput, "cuvid") {
		result.ExecutionHWTypes = append(result.ExecutionHWTypes, model.ExecutionHWNVIDIA)
		result.EncodeCodecs = append(result.EncodeCodecs, detectCodecs(encodersOutput, "nvenc")...)
		result.DecodeCodecs = append(result.DecodeCodecs, detectCodecs(decodersOutput, "cuvid")...)
	}
	if strings.Contains(hwaccelsOutput, "qsv") || strings.Contains(encodersOutput, "_qsv") || strings.Contains(decodersOutput, "_qsv") {
		result.ExecutionHWTypes = append(result.ExecutionHWTypes, qsv.HWAccelName())
		result.EncodeCodecs = append(result.EncodeCodecs, detectCodecs(encodersOutput, "_qsv")...)
		result.DecodeCodecs = append(result.DecodeCodecs, detectCodecs(decodersOutput, "_qsv")...)
	}
	if strings.Contains(hwaccelsOutput, "d3d11va") || strings.Contains(hwaccelsOutput, "amf") || strings.Contains(encodersOutput, "_amf") {
		result.ExecutionHWTypes = append(result.ExecutionHWTypes, amf.HWAccelName())
		result.EncodeCodecs = append(result.EncodeCodecs, detectCodecs(encodersOutput, "_amf")...)
		result.DecodeCodecs = append(result.DecodeCodecs, detectCodecs(decodersOutput, "_amf")...)
	}
	if strings.Contains(hwaccelsOutput, "vaapi") || strings.Contains(encodersOutput, "_vaapi") || strings.Contains(decodersOutput, "_vaapi") {
		result.ExecutionHWTypes = append(result.ExecutionHWTypes, model.ExecutionHWVAAPI)
		result.EncodeCodecs = append(result.EncodeCodecs, detectCodecs(encodersOutput, "_vaapi")...)
		result.DecodeCodecs = append(result.DecodeCodecs, detectCodecs(decodersOutput, "_vaapi")...)
	}
	if strings.Contains(hwaccelsOutput, "videotoolbox") || strings.Contains(encodersOutput, "_videotoolbox") || strings.Contains(decodersOutput, "_videotoolbox") {
		result.ExecutionHWTypes = append(result.ExecutionHWTypes, model.ExecutionHWAppleVideoToolbox)
		result.EncodeCodecs = append(result.EncodeCodecs, detectCodecs(encodersOutput, "_videotoolbox")...)
		result.DecodeCodecs = append(result.DecodeCodecs, detectCodecs(decodersOutput, "_videotoolbox")...)
	}

	if len(result.ExecutionHWTypes) == 0 && len(result.EncodeCodecs) == 0 && len(result.DecodeCodecs) == 0 {
		return nil
	}
	return []ProbeResult{result}
}

func detectCodecs(output string, marker string) []string {
	codecs := make([]string, 0, 4)
	for _, codec := range []string{"h264", "hevc", "av1", "vp9", "mpeg2", "mjpeg"} {
		if strings.Contains(output, codec+marker) {
			codecs = append(codecs, codec)
		}
	}
	return codecs
}
