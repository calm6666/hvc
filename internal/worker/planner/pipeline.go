package planner

import (
	"hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/model"
)

// Pipeline 表示一次转码的完整管线配置。
type Pipeline struct {
	SourceURL          string
	OutputDir          string
	SegmentDurationSec int
	EnableWatermark    bool
	WatermarkPath      string
	WatermarkFilter    string
	HardwareDecode     bool
	HardwareEncode     bool
	DecodeHWAccel      string
	DecodeDevice       string
	DecodeCodec        string
	OutputFormat       string
	Renditions         []RenditionSpec

	// SegmentNaming 分片命名配置，从后台动态配置读取。
	SegmentNaming SegmentNamingConfig
}

// SegmentNamingConfig 分片命名配置。
//
// 所有模板支持以下占位符：
//   - {job_id}       任务 ID
//   - {rendition}    清晰度名称（如 1080p、720p）
//   - {rendition_id} 清晰度序号
//   - {number}       分片序号（从 1 开始）
//   - {timestamp}    分片起始时间戳（毫秒）
type SegmentNamingConfig struct {
	// SegmentTemplate 分片命名模板（init 和 media 统一）。
	// init 分片：number=0，media 分片：number=1,2,3...
	// 默认：{job_id}/{rendition}/seg-{number}.m4s
	SegmentTemplate string
	// ObjectKeyPrefix 对象存储键前缀。
	ObjectKeyPrefix string
}

// DefaultSegmentNamingConfig 返回默认分片命名配置。
func DefaultSegmentNamingConfig() SegmentNamingConfig {
	return SegmentNamingConfig{
		SegmentTemplate:  "{job_id}/{rendition}/seg-{number}.m4s",
		ObjectKeyPrefix:  "",
	}
}

// RenditionSpec 表示一个清晰度输出规格。
type RenditionSpec struct {
	Name             string
	Width            int
	Height           int
	VideoCodec       string
	VideoBitrateKbps int
	VideoMaxrateKbps int
	VideoBufsizeKbps int
	Preset           string
	GOPSize          int
	FPS              float64
	AudioCodec       string
	AudioBitrateKbps int
	AudioChannels    int
	AudioSampleRate  int
}

// BuildPlan 根据 ffprobe 结果和任务配置构建转码管线。
func BuildPlan(job model.TranscodeJob, probeResult probe.Result, executionHW string, naming SegmentNamingConfig) Pipeline {
	pipeline := Pipeline{
		SourceURL:          job.SourceURL,
		OutputDir:          "",
		SegmentDurationSec: job.SegmentDurationSec,
		EnableWatermark:    job.EnableWatermark,
		HardwareDecode:     false,
		HardwareEncode:     executionHW != model.ExecutionHWSoftware,
		OutputFormat:       "fmp4",
		SegmentNaming:      naming,
	}

	if pipeline.SegmentDurationSec <= 0 {
		pipeline.SegmentDurationSec = 6
	}

	if executionHW != model.ExecutionHWSoftware {
		if decoders, ok := hwDecoderMap[executionHW]; ok {
			if dec, ok2 := decoders[probeResult.VideoCodec]; ok2 {
				pipeline.HardwareDecode = true
				pipeline.DecodeCodec = dec
			}
		}
		if flag, ok := hwAccelFlagMap[executionHW]; ok {
			pipeline.DecodeHWAccel = flag
		}
	}

	if !pipeline.HardwareDecode && pipeline.HardwareEncode {
		pipeline.DecodeHWAccel = ""
		pipeline.DecodeCodec = ""
	}

	if job.EnableWatermark && job.WatermarkImageURL != "" {
		pipeline.WatermarkPath = job.WatermarkImageURL
		pipeline.WatermarkFilter = buildWatermarkFilter(job)
	}

	pipeline.Renditions = buildRenditions(job, probeResult, executionHW)

	return pipeline
}
