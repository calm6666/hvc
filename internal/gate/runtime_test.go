package gate

import "testing"

/* ---------- A2 硬编优先（运行时观测） ---------- */

func TestHardwareEncodePreferenceHardwareEncoderPasses(t *testing.T) {
	observation := EncoderObservation{JobID: 11, Encoder: "h264_nvenc", CPUPeakPercent: 20, CPUSamples: 5}

	result := RunHardwareEncodePreference(observation)

	if !result.Passed || result.Skipped || result.NotApplicable {
		t.Fatalf("硬件编码器应通过：passed=%v skipped=%v na=%v detail=%s",
			result.Passed, result.Skipped, result.NotApplicable, result.Detail)
	}
}

func TestHardwareEncodePreferenceAllHardwareMarkers(t *testing.T) {
	/* 覆盖各平台的硬件编码器名，避免白名单漏掉某个平台。 */
	for _, encoder := range []string{"hevc_qsv", "h264_amf", "h264_vaapi", "h264_videotoolbox", "h264_mediacodec", "h264_oh_avcodec", "h264_v4l2m2m"} {
		if !isHardwareEncoder(encoder) {
			t.Fatalf("%s 应被认定为硬件编码器", encoder)
		}
	}

	if isHardwareEncoder("libx264") || isHardwareEncoder("libx265") {
		t.Fatal("软件编码器不能被认定为硬件编码器")
	}
}

func TestHardwareEncodePreferenceSoftwareWithReasonPasses(t *testing.T) {
	observation := EncoderObservation{
		JobID: 11, Encoder: "libx264",
		DecodeFallbackReason: "源视频无可用硬解设备（V100 无 NVENC 会话）",
	}

	result := RunHardwareEncodePreference(observation)

	if !result.Passed {
		t.Fatalf("有记录理由的软编例外应通过：%s", result.Detail)
	}

	if result.Detail == "" {
		t.Fatal("放行例外时必须在 Detail 里写明观测到的编码器与理由，便于审计")
	}
}

func TestHardwareEncodePreferenceSoftwareWithoutReasonFails(t *testing.T) {
	observation := EncoderObservation{JobID: 11, Encoder: "libx264"}

	result := RunHardwareEncodePreference(observation)

	if result.Passed || result.Skipped || result.NotApplicable {
		t.Fatalf("无理由的软编必须判失败：passed=%v skipped=%v na=%v",
			result.Passed, result.Skipped, result.NotApplicable)
	}
}

func TestHardwareEncodePreferenceNoObservationSkips(t *testing.T) {
	result := RunHardwareEncodePreference(EncoderObservation{JobID: 11})

	if !result.Skipped || result.Passed {
		t.Fatalf("没有观测数据必须 Skipped 且不算通过：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

/* ---------- A3 软解 CPU 上限（运行时观测） ---------- */

func TestSoftwareDecodeCPULimitUnderLimitPasses(t *testing.T) {
	observation := EncoderObservation{JobID: 12, HardwareDecode: false, CPUPeakPercent: 42.5, CPUSamples: 30}

	result := RunSoftwareDecodeCPULimit(observation)

	if !result.Passed || result.Skipped || result.NotApplicable {
		t.Fatalf("软解峰值 42.5%% 应通过：passed=%v skipped=%v na=%v detail=%s",
			result.Passed, result.Skipped, result.NotApplicable, result.Detail)
	}
}

func TestSoftwareDecodeCPULimitOverLimitFails(t *testing.T) {
	observation := EncoderObservation{JobID: 12, HardwareDecode: false, CPUPeakPercent: 68.0, CPUSamples: 30}

	result := RunSoftwareDecodeCPULimit(observation)

	if result.Passed || result.Skipped || result.NotApplicable {
		t.Fatalf("软解峰值 68%% 必须判失败：passed=%v skipped=%v na=%v",
			result.Passed, result.Skipped, result.NotApplicable)
	}
}

func TestSoftwareDecodeCPULimitExactlyAtLimitPasses(t *testing.T) {
	/* 边界：判据是"不得超过 50%"，恰好 50% 应放行。 */
	observation := EncoderObservation{JobID: 12, HardwareDecode: false, CPUPeakPercent: 50.0, CPUSamples: 10}

	result := RunSoftwareDecodeCPULimit(observation)

	if !result.Passed {
		t.Fatalf("恰好 50%% 应通过（判据是「不得超过」，边界值放行）：%s", result.Detail)
	}
}

func TestSoftwareDecodeCPULimitHardwareDecodeNotApplicable(t *testing.T) {
	observation := EncoderObservation{JobID: 12, HardwareDecode: true, CPUPeakPercent: 95.0, CPUSamples: 30}

	result := RunSoftwareDecodeCPULimit(observation)

	if !result.NotApplicable {
		t.Fatalf("硬解路径必须标记为不适用：na=%v passed=%v detail=%s",
			result.NotApplicable, result.Passed, result.Detail)
	}

	if result.Passed {
		t.Fatal("不适用不能被当成通过")
	}
}

func TestSoftwareDecodeCPULimitNoSamplesSkips(t *testing.T) {
	observation := EncoderObservation{JobID: 12, HardwareDecode: false, CPUPeakPercent: 0, CPUSamples: 0}

	result := RunSoftwareDecodeCPULimit(observation)

	if !result.Skipped || result.Passed || result.NotApplicable {
		t.Fatalf("没有采样必须 Skipped（不是不适用、也不是通过）：passed=%v skipped=%v na=%v",
			result.Passed, result.Skipped, result.NotApplicable)
	}
}
