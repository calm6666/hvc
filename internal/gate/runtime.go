package gate

import (
	"fmt"
	"strings"
)

/*
 * 本文件是门禁的最后两条：A2 硬编优先、A3 软解 CPU ≤ 50%。
 *
 * 【口径：判运行时观测】—— 按项目方的选择，这两条不看"配置里写了什么/调度时选了什么"，
 * 只看**执行期间实际观测到的编码器与 CPU 占用**。理由：配置和调度决策都可能与真实执行不一致
 * （回退到软编、软解设备不可用等），而这两条判据关心的正是"最终到底是怎么跑的"。
 *
 * 观测数据由执行侧采集：编码器名取 ffmpeg 实际使用的编码器（-c:v 的实际取值），
 * CPU 占用取任务执行窗口内的采样峰值。采集与上报不在门禁里实现 —— 门禁只做判定。
 */

/* 硬件编码器名特征：ffmpeg 的硬件编码器名都以这些后缀区分（h264_nvenc、hevc_qsv、h264_amf…）。
 * 用"包含"而不是精确列表：编码器名会随 ffmpeg 版本与平台变化，精确列表会漏掉新名字。 */
var hardwareEncoderMarkers = []string{
	"nvenc",      // NVIDIA
	"qsv",        // Intel
	"amf",        // AMD
	"vaapi",      // Linux VA-API
	"videotoolbox", // Apple
	"mediacodec", // Android
	"oh_avcodec", // HarmonyOS
	"v4l2m2m",    // 树莓派等嵌入式硬编
}

// softwareDecodeCPUPeakLimitPercent 软解时的 CPU 峰值上限（百分比）。
// 出处：项目总原则"软解 CPU 总占用默认不得超过 50%"。
const softwareDecodeCPUPeakLimitPercent = 50.0

// EncoderObservation 一次任务执行的运行时观测事实。
//
// 采集侧（worker/执行器）负责填这些字段；门禁只读不写。
// 字段为零值时的语义要分清：Encoder 为空表示"没观测到"，而不是"观测到软编"。
type EncoderObservation struct {
	JobID uint64
	// Encoder 实际使用的视频编码器名（如 h264_nvenc / h264_qsv / libx264）。
	Encoder string
	// HardwareDecode 本次执行是否走硬解 —— 决定 A3 是否适用。
	HardwareDecode bool
	// DecodeFallbackReason 走软解的记录理由（源不支持硬解、无可用硬解设备等）；空表示没有记录理由。
	// 硬解时该字段应为空。
	DecodeFallbackReason string
	// CPUPeakPercent 执行窗口内观测到的 CPU 峰值占用（0..100）。
	CPUPeakPercent float64
	// CPUSamples 采样条数；0 表示没有采集到运行时 CPU 数据。
	CPUSamples int
}

// isHardwareEncoder 判断观测到的编码器名是不是硬件编码器。
func isHardwareEncoder(encoder string) bool {
	lowered := strings.ToLower(encoder)

	for _, marker := range hardwareEncoderMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}

	return false
}

// RunHardwareEncodePreference A2（推导，运行时口径）：实际使用的编码器必须是硬件编码器。
//
// 判定：
//   - 没有观测到编码器名 ⇒ Skipped（不算通过）；
//   - 观测到的编码器命中硬件白名单 ⇒ 通过；
//   - 观测到的是软编：若有**记录在案的**软解/软编理由（DecodeFallbackReason 非空）⇒ 通过，
//     并在 Detail 里写清"观测到的编码器 + 理由"，便于审计这是例外而不是默认；
//     没有记录理由 ⇒ 失败。
func RunHardwareEncodePreference(observation EncoderObservation) CheckResult {
	const id = "A2"
	const title = "实际使用硬件编码器（推导，运行时观测）"

	if strings.TrimSpace(observation.Encoder) == "" {
		return CheckResult{
			ID: id, Title: title, Skipped: true,
			Detail: fmt.Sprintf("任务 %d 没有观测到实际使用的编码器，无法判定", observation.JobID),
		}
	}

	if isHardwareEncoder(observation.Encoder) {
		return CheckResult{ID: id, Title: title, Passed: true}
	}

	if strings.TrimSpace(observation.DecodeFallbackReason) != "" {
		return CheckResult{
			ID: id, Title: title, Passed: true,
			Detail: fmt.Sprintf("实际使用软编 %q，依据记录的例外理由放行：%s",
				observation.Encoder, observation.DecodeFallbackReason),
		}
	}

	return CheckResult{
		ID: id, Title: title,
		Detail: fmt.Sprintf("任务 %d 实际使用软编 %q，且没有记录任何例外理由",
			observation.JobID, observation.Encoder),
	}
}

// RunSoftwareDecodeCPULimit A3（推导，运行时口径）：软解路径下 CPU 峰值不得超过 50%。
//
// 判定：
//   - 本次走硬解 ⇒ **不适用**（NotApplicable，不是通过也不是跳过）：这条判据只约束软解；
//   - 走软解但没有 CPU 采样 ⇒ Skipped（该判却没判成，不算通过）；
//   - 峰值 ≤ 50% ⇒ 通过；超过 ⇒ 失败，并带上峰值与采样条数（采样越多，结论越可信）。
func RunSoftwareDecodeCPULimit(observation EncoderObservation) CheckResult {
	const id = "A3"
	const title = "软解路径 CPU 峰值不超过 50%（推导，运行时观测）"

	if observation.HardwareDecode {
		return CheckResult{
			ID: id, Title: title, NotApplicable: true,
			Detail: fmt.Sprintf("任务 %d 走硬解，软解 CPU 上限不适用", observation.JobID),
		}
	}

	if observation.CPUSamples <= 0 {
		return CheckResult{
			ID: id, Title: title, Skipped: true,
			Detail: fmt.Sprintf("任务 %d 走软解但没有 CPU 采样，无法判定", observation.JobID),
		}
	}

	if observation.CPUPeakPercent > softwareDecodeCPUPeakLimitPercent {
		return CheckResult{
			ID: id, Title: title,
			Detail: fmt.Sprintf("任务 %d 软解期间 CPU 峰值 %.1f%%，超过上限 %.0f%%（采样 %d 次）",
				observation.JobID, observation.CPUPeakPercent, softwareDecodeCPUPeakLimitPercent, observation.CPUSamples),
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}
