package model

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// TranscodeRendition 表示单个转码任务下的一个清晰度子任务快照。
//
// 这里持久化保存每个清晰度的稳定身份信息和输出参数，目的有两点：
// 1. 重试时复用同一份清晰度元数据，不让分片命名里的 rendition_key 漂移；
// 2. 后续生成清单、审计任务产物时，可以直接从数据库拿到每个清晰度的稳定标识。
type TranscodeRendition struct {
	RenditionID       uint64
	JobID             uint64
	RenditionName     string
	RenditionKey      string
	Status            int
	OutWidth          int
	OutHeight         int
	VideoCodec        string
	AudioCodec        string
	VideoBitrateKbps  int
	AudioBitrateKbps  int
	SegmentCountVideo int
	SegmentCountAudio int
	ProgressPermille  int
	ErrorCode         string
	ErrorMessage      string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// BuildStableRenditionKey 为单任务内的单个清晰度生成稳定短 key。
//
// 该 key 使用任务 ID 和清晰度核心参数做哈希，同一任务同一清晰度在重试时恒定不变，
// 不同任务之间即使偶然重复也不会造成冲突，因为对象名里仍然会带 job_id。
func BuildStableRenditionKey(jobID uint64, renditionName string, width, height int, videoBitrateKbps, audioBitrateKbps int, videoCodec, audioCodec string) string {
	identity := fmt.Sprintf("%d|%s|%d|%d|%d|%d|%s|%s",
		jobID,
		strings.TrimSpace(renditionName),
		width,
		height,
		videoBitrateKbps,
		audioBitrateKbps,
		strings.ToLower(strings.TrimSpace(videoCodec)),
		strings.ToLower(strings.TrimSpace(audioCodec)),
	)
	sum := sha256.Sum256([]byte(identity))
	token := base64.RawURLEncoding.EncodeToString(sum[:])
	if len(token) >= 8 {
		return token[:8]
	}
	return token
}
