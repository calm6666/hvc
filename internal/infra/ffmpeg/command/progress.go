package command

// BuildProgressArgs 返回 ffmpeg progress 参数。
func BuildProgressArgs() []string {
	return []string{"-progress", "pipe:2", "-nostats"}
}
