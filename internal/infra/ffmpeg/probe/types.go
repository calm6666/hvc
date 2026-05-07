package probe

type Result struct {
	VideoCodec       string  `json:"video_codec"`
	AudioCodec       string  `json:"audio_codec"`
	ContainerFormat  string  `json:"container_format"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	FPS              float64 `json:"fps"`
	DurationMS       int64   `json:"duration_ms"`
	VideoBitrateKbps int     `json:"video_bitrate_kbps"`
	AudioBitrateKbps int     `json:"audio_bitrate_kbps"`
	AvgBitrateKbps   int     `json:"avg_bitrate_kbps"`
	PixFmt           string  `json:"pix_fmt"`
	ColorRange       string  `json:"color_range"`
	AudioChannels    int     `json:"audio_channels"`
	AudioSampleRate  int     `json:"audio_sample_rate"`
	HasBFrame        bool    `json:"has_b_frame"`
	GOPSize          int     `json:"gop_size"`
	KeyintSec        float64 `json:"keyint_sec"`
	PTSMonotonic     bool    `json:"pts_monotonic"`
	DTSMonotonic     bool    `json:"dts_monotonic"`
}
