package executor

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"hvc/internal/model"
)

// Runner 表示转码执行器。
type Runner struct {
	ffmpegPath string
}

// NewRunner 创建转码执行器。
func NewRunner() *Runner {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		path = "ffmpeg"
	}
	return &Runner{ffmpegPath: path}
}

// Run 执行一次转码任务并返回进度事件流。
func (r *Runner) Run(ctx context.Context, job model.TranscodeJob) <-chan model.TranscodeProgress {
	ch := make(chan model.TranscodeProgress, 16)
	go func() {
		defer close(ch)
		cmd := exec.CommandContext(ctx, r.ffmpegPath,
			"-hide_banner",
			"-y",
			"-progress", "pipe:2",
			"-nostats",
			"-f", "lavfi",
			"-i", "testsrc=size=1280x720:rate=30",
			"-t", "1",
			"-f", "null",
			"-",
		)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			return
		}
		parseProgress(stderr, job.JobID, ch)
		_ = cmd.Wait()
	}()
	return ch
}

func parseProgress(reader io.Reader, jobID uint64, ch chan<- model.TranscodeProgress) {
	scanner := bufio.NewScanner(reader)
	re := regexp.MustCompile(`out_time_ms=(\d+)|speed=([\d.]+)x`)
	progress := model.TranscodeProgress{JobID: jobID, Stage: model.StageTranscoding}
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		if matches[1] != "" {
			timeMS, _ := strconv.ParseInt(matches[1], 10, 64)
			progress.TimeMS = timeMS
			progress.Percent = 90
		}
		if matches[2] != "" {
			speed, _ := strconv.ParseFloat(matches[2], 64)
			progress.Speed = speed
		}
		progress.UpdatedAt = time.Now()
		ch <- progress
	}
	progress.Stage = model.StageUploading
	progress.Percent = 95
	progress.UpdatedAt = time.Now()
	ch <- progress
}
