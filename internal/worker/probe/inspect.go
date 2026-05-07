package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ffprobe "hvc/internal/infra/ffmpeg/probe"
	ffmpegprocess "hvc/internal/infra/ffmpeg/process"
	"hvc/internal/infra/ffmpeg/parser"
	"os/exec"
)

type Result = ffprobe.Result

func Inspect(sourceURL string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := InspectWithContext(ctx, sourceURL)
	if err != nil {
		return Result{}
	}
	return result
}

func InspectWithContext(ctx context.Context, sourceURL string) (Result, error) {
	path := ffmpegprocess.FindFFprobe()
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_entries",
		"stream=codec_name,codec_type,width,height,r_frame_rate,pix_fmt," +
			"channels,sample_rate,has_b_frames,gop_size,bit_rate:" +
			"format=duration,bit_rate,size,format_name",
		sourceURL,
	}
	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.Output()
	if err != nil {
		return Result{}, fmt.Errorf("ffprobe command failed: %w", err)
	}
	return parseProbeOutput(output)
}

func parseProbeOutput(data []byte) (Result, error) {
	var raw struct {
		Streams []map[string]any `json:"streams"`
		Format  map[string]any  `json:"format"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Result{}, fmt.Errorf("ffprobe json unmarshal failed: %w", err)
	}
	result := Result{}
	result.ContainerFormat = strVal(raw.Format, "format_name")
	result.DurationMS = int64(parser.ParseFloat64(strVal(raw.Format, "duration")) * 1000)
	result.AvgBitrateKbps = int(parser.ParseFloat64(strVal(raw.Format, "bit_rate")) / 1000)
	for _, stream := range raw.Streams {
		codecType := strVal(stream, "codec_type")
		switch codecType {
		case "video":
			result.VideoCodec = strVal(stream, "codec_name")
			result.Width = int(parser.ParseFloat64(strVal(stream, "width")))
			result.Height = int(parser.ParseFloat64(strVal(stream, "height")))
			result.FPS = parser.ParseFPS(strVal(stream, "r_frame_rate"))
			result.PixFmt = strVal(stream, "pix_fmt")
			result.VideoBitrateKbps = int(parser.ParseFloat64(strVal(stream, "bit_rate")) / 1000)
			result.HasBFrame = parser.ParseFloat64(strVal(stream, "has_b_frames")) > 0
			gopSize := parser.ParseFloat64(strVal(stream, "gop_size"))
			result.GOPSize = int(gopSize)
			if result.FPS > 0 && gopSize > 0 {
				result.KeyintSec = gopSize / result.FPS
			}
		case "audio":
			result.AudioCodec = strVal(stream, "codec_name")
			result.AudioChannels = int(parser.ParseFloat64(strVal(stream, "channels")))
			result.AudioSampleRate = int(parser.ParseFloat64(strVal(stream, "sample_rate")))
			result.AudioBitrateKbps = int(parser.ParseFloat64(strVal(stream, "bit_rate")) / 1000)
		}
	}
	return result, nil
}

func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
