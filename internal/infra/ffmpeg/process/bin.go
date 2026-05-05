package ffmpeg

import "os/exec"

// FindFFmpeg 返回 ffmpeg 路径。
func FindFFmpeg() string {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "ffmpeg"
	}
	return path
}

// FindFFprobe 返回 ffprobe 路径。
func FindFFprobe() string {
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		return "ffprobe"
	}
	return path
}
