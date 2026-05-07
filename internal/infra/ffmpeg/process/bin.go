// Package process 提供 FFmpeg/FFprobe 进程管理。
//
// 本文件负责查找系统中的 FFmpeg 和 FFprobe 可执行文件路径，
// 并提供进程启动、停止和信号处理功能。
//
// 查找策略：
//  1. 优先使用 PATH 环境变量中的 ffmpeg/ffprobe
//  2. 如果 PATH 中未找到，尝试常见安装路径
//  3. Windows: C:\ffmpeg\bin\ffmpeg.exe
//  4. Linux: /usr/bin/ffmpeg, /usr/local/bin/ffmpeg
//  5. macOS: /opt/homebrew/bin/ffmpeg
//
// 多 GPU 支持：
//   - 启动 FFmpeg 进程时可通过环境变量 CUDA_VISIBLE_DEVICES
//     限制进程可见的 GPU 设备
//   - 不同转码任务可使用不同的 GPU 设备索引
package process

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"hvc/pkg/logx"
)

var (
	ffmpegPath  string
	ffprobePath string
	findOnce    sync.Once
)

// FindFFmpeg 查找 FFmpeg 可执行文件路径。
//
// 首次调用时执行查找，后续调用直接返回缓存结果。
func FindFFmpeg() string {
	findOnce.Do(findBinaries)
	return ffmpegPath
}

// FindFFprobe 查找 FFprobe 可执行文件路径。
func FindFFprobe() string {
	findOnce.Do(findBinaries)
	return ffprobePath
}

// findBinaries 查找 FFmpeg 和 FFprobe 可执行文件。
func findBinaries() {
	ffmpegPath = findBinary("ffmpeg")
	ffprobePath = findBinary("ffprobe")

	if ffmpegPath != "" {
		logx.Info("ffmpeg.process.ffmpeg_found", logx.Fields{
			"path": ffmpegPath,
		})
	} else {
		logx.Error("ffmpeg.process.ffmpeg_not_found", nil, nil)
	}

	if ffprobePath != "" {
		logx.Info("ffmpeg.process.ffprobe_found", logx.Fields{
			"path": ffprobePath,
		})
	} else {
		logx.Error("ffmpeg.process.ffprobe_not_found", nil, nil)
	}
}

// findBinary 在系统中查找指定可执行文件。
func findBinary(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	if runtime.GOOS == "windows" {
		candidates := []string{
			`C:\ffmpeg\bin\` + name + `.exe`,
			`C:\Program Files\ffmpeg\bin\` + name + `.exe`,
			`C:\ProgramData\chocolatey\bin\` + name + `.exe`,
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	} else {
		candidates := []string{
			"/usr/bin/" + name,
			"/usr/local/bin/" + name,
			"/opt/homebrew/bin/" + name,
			"/snap/bin/" + name,
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	return ""
}

// ManagedProcess 表示受管理的 FFmpeg 进程。
//
// 封装 exec.Cmd，提供超时控制、优雅停止和强制终止功能。
type ManagedProcess struct {
	cmd       *exec.Cmd
	startTime time.Time
	gpuIndex  int
}

// NewManagedProcess 创建受管理的进程。
//
// 参数：
//   - name: 可执行文件路径
//   - args: 命令行参数
//   - gpuIndex: 分配的 GPU 索引（-1 表示使用软解或所有 GPU）
func NewManagedProcess(name string, args []string, gpuIndex int) *ManagedProcess {
	cmd := exec.Command(name, args...)

	if gpuIndex >= 0 && runtime.GOOS != "windows" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CUDA_VISIBLE_DEVICES=%d", gpuIndex))
	} else if gpuIndex >= 0 && runtime.GOOS == "windows" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CUDA_VISIBLE_DEVICES=%d", gpuIndex))
	}

	return &ManagedProcess{
		cmd:      cmd,
		gpuIndex: gpuIndex,
	}
}

// SetGPUIndex 设置进程使用的 GPU 索引。
//
// 通过 CUDA_VISIBLE_DEVICES 环境变量控制进程可见的 GPU。
// 在多 GPU 服务器上，不同转码任务应使用不同的 GPU 索引，
// 避免所有任务集中在同一块 GPU 上导致过载。
func (p *ManagedProcess) SetGPUIndex(gpuIndex int) {
	p.gpuIndex = gpuIndex
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "CUDA_VISIBLE_DEVICES") {
			continue
		}
		filtered = append(filtered, e)
	}
	if gpuIndex >= 0 {
		filtered = append(filtered, fmt.Sprintf("CUDA_VISIBLE_DEVICES=%d", gpuIndex))
	}
	p.cmd.Env = filtered
}

// SetEnv 设置额外的环境变量。
func (p *ManagedProcess) SetEnv(env []string) {
	p.cmd.Env = append(p.cmd.Env, env...)
}

// SetStdout 设置标准输出。
func (p *ManagedProcess) SetStdout(w *os.File) {
	p.cmd.Stdout = w
}

// SetStderr 设置标准错误输出。
func (p *ManagedProcess) SetStderr(w *os.File) {
	p.cmd.Stderr = w
}

// Start 启动进程。
func (p *ManagedProcess) Start() error {
	p.startTime = time.Now()
	return p.cmd.Start()
}

// Wait 等待进程结束。
func (p *ManagedProcess) Wait() error {
	return p.cmd.Wait()
}

// Stop 优雅停止进程。
//
// 先发送中断信号（SIGINT/CTRL_BREAK），等待 5 秒，
// 如果进程仍未退出，则强制终止（SIGKILL/CTRL_C）。
func (p *ManagedProcess) Stop() error {
	if p.cmd.Process == nil {
		return nil
	}

	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return p.cmd.Process.Kill()
	}

	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return p.cmd.Process.Kill()
	}
}

// Kill 强制终止进程。
func (p *ManagedProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// Running 判断进程是否正在运行。
func (p *ManagedProcess) Running() bool {
	if p.cmd.Process == nil {
		return false
	}
	return p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()
}

// Elapsed 返回进程已运行时长。
func (p *ManagedProcess) Elapsed() time.Duration {
	return time.Since(p.startTime)
}

// PID 返回进程 ID。
func (p *ManagedProcess) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
