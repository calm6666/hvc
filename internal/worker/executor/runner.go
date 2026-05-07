package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"hvc/internal/model"
	"hvc/internal/worker/planner"
	ffmpegprocess "hvc/internal/infra/ffmpeg/process"
	"hvc/pkg/logx"
)

type Runner struct {
	ffmpegPath      string
	totalDurationMs int64
	cmd             *exec.Cmd
	cancelFn        context.CancelFunc
}

func NewRunner() *Runner {
	return &Runner{ffmpegPath: ffmpegprocess.FindFFmpeg()}
}

func (r *Runner) Run(ctx context.Context, job model.TranscodeJob, pipeline planner.Pipeline, totalDurationMs int64) (<-chan model.TranscodeProgress, error) {
	r.totalDurationMs = totalDurationMs

	if err := os.MkdirAll(pipeline.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir failed: %w", err)
	}

	args := planner.BuildFFmpegArgs(pipeline)
	args = append([]string{"-progress", "pipe:2", "-nostats"}, args...)

	childCtx, cancel := context.WithCancel(ctx)
	r.cancelFn = cancel

	cmd := exec.CommandContext(childCtx, r.ffmpegPath, args...)
	r.cmd = cmd

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stderr pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ffmpeg failed: %w", err)
	}

	ch := make(chan model.TranscodeProgress, 32)
	go func() {
		defer close(ch)
		r.parseProgress(stderr, job.JobID, ch)
		waitErr := cmd.Wait()
		if waitErr != nil {
			if ctx.Err() == nil {
				logx.Error("executor.ffmpeg.wait_failed", waitErr, logx.Fields{
					"job_id": job.JobID,
				})
			}
		}
	}()

	return ch, nil
}

func (r *Runner) Stop(force bool) {
	if r.cancelFn != nil {
		r.cancelFn()
	}
	if force && r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Signal(syscall.SIGKILL)
	}
}

func (r *Runner) parseProgress(reader io.Reader, jobID uint64, ch chan<- model.TranscodeProgress) {
	scanner := bufio.NewScanner(reader)
	re := regexp.MustCompile(`out_time_ms=(\d+)|speed=([\d.]+)x|fps=([\d.]+)|bitrate=([\d.]+)kbits/s|total_size=(\d+)`)

	progress := model.TranscodeProgress{JobID: jobID, Stage: model.StageTranscoding}

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		if matches[1] != "" {
			timeUS, _ := strconv.ParseInt(matches[1], 10, 64)
			progress.TimeMS = timeUS / 1000
			if r.totalDurationMs > 0 {
				progress.Percent = float64(progress.TimeMS) / float64(r.totalDurationMs) * 100.0
				if progress.Percent > 100.0 {
					progress.Percent = 100.0
				}
			}
		}
		if matches[2] != "" {
			progress.Speed, _ = strconv.ParseFloat(matches[2], 64)
		}
		if matches[3] != "" {
			progress.FPS, _ = strconv.ParseFloat(matches[3], 64)
		}
		if matches[4] != "" {
			progress.BitrateKbps, _ = strconv.ParseFloat(matches[4], 64)
		}

		progress.UpdatedAt = time.Now()
		ch <- progress
	}

	progress.Stage = model.StageUploading
	progress.Percent = 100.0
	progress.UpdatedAt = time.Now()
	ch <- progress
}
