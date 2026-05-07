package planner

import (
	"fmt"

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
//   - {job_id}       任务 ID（雪花 ID）
//   - {media_type}   媒体类型（video / audio）
//   - {number}       分片序号（0=init, 1/2/3...=media）
//   - {resolution}   视频分辨率（宽_高，如 1920_1080）
//   - {quality}      清晰度标签（如 1080p、720p、480p、360p）
//   - {timestamp}    分片起始时间戳（毫秒）
//
// 六种预置模板方案：
//   方案一：{job_id}-{media_type}-{number}.m4s
//   方案二：{job_id}-{resolution}-{media_type}-{number}.m4s
//   方案三：{job_id}-{quality}-{media_type}-{number}.m4s
//   方案四：{job_id}-{media_type}-{number}-{timestamp}.m4s
//   方案五：{job_id}-{resolution}-{media_type}-{number}-{timestamp}.m4s
//   方案六：{job_id}-{quality}-{media_type}-{number}-{timestamp}.m4s
type SegmentNamingConfig struct {
	// SegmentTemplate 分片命名模板（init 和 media 统一）。
	// init 分片：number=0，media 分片：number=1,2,3...
	// 后台管理界面可动态修改，从上述六种方案中选择或自定义。
	SegmentTemplate string
	// ObjectKeyPrefix 对象存储键前缀。
	ObjectKeyPrefix string
	// JobID 任务 ID（雪花 ID），渲染模板时填充 {job_id} 占位符。
	JobID uint64
}

// DefaultSegmentNamingConfig 返回默认分片命名配置（方案一）。
func DefaultSegmentNamingConfig() SegmentNamingConfig {
	return SegmentNamingConfig{
		SegmentTemplate: "{job_id}-{media_type}-{number}.m4s",
		ObjectKeyPrefix: "",
	}
}

// RenditionSpec 表示一个清晰度输出规格。
type RenditionSpec struct {
	Name             string
	QualityLabel     string
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

// Resolution 返回分辨率字符串（宽_高格式），用于模板 {resolution} 占位符。
func (r RenditionSpec) Resolution() string {
	return fmt.Sprintf("%d_%d", r.Width, r.Height)
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
