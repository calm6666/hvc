// Package command 提供 FFmpeg 命令行参数构建辅助函数。
//
// 本文件提供构建 FFmpeg 命令行参数的辅助函数，
// 包括进度输出参数、硬件加速参数、编码参数等。
//
// FFmpeg 命令行结构：
//
//	ffmpeg [全局选项] [输入选项] -i 输入 [输出选项] 输出
//
// 全局选项示例：
//   - -hide_banner: 隐藏编译信息
//   - -y: 覆盖输出文件
//   - -progress pipe:2: 将进度信息输出到 stderr
//   - -nostats: 禁止输出统计信息
package command

import (
	"fmt"
	"path/filepath"
)

// ProgressArgs 返回 FFmpeg 进度输出相关的命令行参数。
//
// -progress pipe:2 将进度信息以 key=value 格式输出到 stderr，
// Worker 通过解析 stderr 获取实时转码进度。
//
// -nostats 禁止 FFmpeg 在转码结束时输出统计摘要，
// 避免与进度信息混合导致解析错误。
func ProgressArgs() []string {
	return []string{"-progress", "pipe:2", "-nostats"}
}

// GlobalArgs 返回 FFmpeg 全局默认参数。
//
// -hide_banner: 隐藏 FFmpeg 版本和编译选项信息
// -y: 自动覆盖输出文件，不提示确认
func GlobalArgs() []string {
	return []string{"-hide_banner", "-y"}
}

// HWAccelArgs 返回硬件加速相关的命令行参数。
//
// 根据 GPU 类型返回不同的参数组合：
//   - NVIDIA: -hwaccel cuda -hwaccel_output_format cuda
//   - Intel QSV: -hwaccel qsv
//   - AMD AMF: 不需要 hwaccel 参数（仅编码使用 h264_amf/hevc_amf）
//   - VAAPI: -hwaccel vaapi -vaapi_device /dev/dri/renderD128
//   - VideoToolbox: -hwaccel videotoolbox
func HWAccelArgs(hwType string, deviceIndex int) []string {
	switch hwType {
	case "nvidia":
		args := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
		if deviceIndex > 0 {
			args = append(args, "-hwaccel_device", fmt.Sprintf("%d", deviceIndex))
		}
		return args
	case "qsv":
		return []string{"-hwaccel", "qsv"}
	case "vaapi":
		return []string{"-hwaccel", "vaapi", "-vaapi_device", "/dev/dri/renderD128"}
	case "videotoolbox":
		return []string{"-hwaccel", "videotoolbox"}
	default:
		return nil
	}
}

// VideoEncodeArgs 返回视频编码相关的命令行参数。
//
// 根据 GPU 类型选择对应的硬件编码器：
//   - NVIDIA: h264_nvenc / hevc_nvenc
//   - Intel QSV: h264_qsv / hevc_qsv
//   - AMD AMF: h264_amf / hevc_amf
//   - VAAPI: h264_vaapi / hevc_vaapi
//   - VideoToolbox: h264_videotoolbox / hevc_videotoolbox
//   - 软解: libx264 / libx265
func VideoEncodeArgs(hwType string, codec string, bitrateKbps int, maxrateKbps int, bufsizeKbps int, gopSize int) []string {
	encoder := selectEncoder(hwType, codec)
	args := []string{"-c:v", encoder}

	if bitrateKbps > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", bitrateKbps))
	}
	if maxrateKbps > 0 {
		args = append(args, "-maxrate", fmt.Sprintf("%dk", maxrateKbps))
	}
	if bufsizeKbps > 0 {
		args = append(args, "-bufsize", fmt.Sprintf("%dk", bufsizeKbps))
	}
	if gopSize > 0 {
		args = append(args, "-g", fmt.Sprintf("%d", gopSize))
	}

	return args
}

// AudioEncodeArgs 返回音频编码相关的命令行参数。
//
// 默认使用 AAC 编码，码率 128k。
func AudioEncodeArgs(codec string, bitrateKbps int, channels int, sampleRate int) []string {
	args := []string{"-c:a", "aac"}
	if codec != "" {
		args[1] = codec
	}
	if bitrateKbps > 0 {
		args = append(args, "-b:a", fmt.Sprintf("%dk", bitrateKbps))
	} else {
		args = append(args, "-b:a", "128k")
	}
	if channels > 0 {
		args = append(args, "-ac", fmt.Sprintf("%d", channels))
	}
	if sampleRate > 0 {
		args = append(args, "-ar", fmt.Sprintf("%d", sampleRate))
	}
	return args
}

// DASHOutputArgs 返回 DASH 输出相关的命令行参数。
//
// 生成 CMAF 兼容的分片文件，同时支持 DASH 和 HLS 播放。
func DASHOutputArgs(outputDir string, segmentDuration int, initSegmentName string, mediaSegmentName string) []string {
	args := []string{
		"-f", "dash",
		"-seg_duration", fmt.Sprintf("%d", segmentDuration),
		"-dash_segment_type", "mp4",
	}
	if initSegmentName != "" {
		args = append(args, "-init_seg_name", initSegmentName)
	}
	if mediaSegmentName != "" {
		args = append(args, "-media_seg_name", mediaSegmentName)
	}
	args = append(args, filepath.Join(outputDir, "manifest.mpd"))
	return args
}

// selectEncoder 根据硬件类型和编解码器选择编码器名称。
func selectEncoder(hwType string, codec string) string {
	switch hwType {
	case "nvidia":
		if codec == "hevc" || codec == "h265" {
			return "hevc_nvenc"
		}
		return "h264_nvenc"
	case "qsv":
		if codec == "hevc" || codec == "h265" {
			return "hevc_qsv"
		}
		return "h264_qsv"
	case "amf":
		if codec == "hevc" || codec == "h265" {
			return "hevc_amf"
		}
		return "h264_amf"
	case "vaapi":
		if codec == "hevc" || codec == "h265" {
			return "hevc_vaapi"
		}
		return "h264_vaapi"
	case "videotoolbox":
		if codec == "hevc" || codec == "h265" {
			return "hevc_videotoolbox"
		}
		return "h264_videotoolbox"
	default:
		if codec == "hevc" || codec == "h265" {
			return "libx265"
		}
		return "libx264"
	}
}
