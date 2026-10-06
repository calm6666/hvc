package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"hvc/internal/model"
)

// jobInputHashVersion 是指纹的算法版本：拼接口径变了就改它，这样老指纹天然对不上、
// 会走"整任务重跑"而不是按已经变形的口径续跑。
const jobInputHashVersion = "v1"

// computeJobInputHash 计算"源 + 输出规格"的指纹（B2 断点续跑的判据）。
//
// 为什么指纹包含这些字段：能不能续跑，完全取决于"这一版输入与上一版是不是同一份"。
//   - 源：只用 SourceURL。刻意不把源文件大小/ETag 放进来：那两项在网络抖动时可能拿不到，
//     把它们算进指纹会让"这次没取到大小"被误判成"输入变了"，从而白白全量重跑。
//     源内容真的变了（同 URL 换了内容）由上传侧的 ETag/大小逐片校验兜住。
//   - 规格：清晰度集合（名字/宽高/编码/码率/预设）、切片时长、DASH/HLS 开关、水印参数、
//     缩略图参数、输出存储与前缀 —— 任何一项变化都会改变中间产物的形态，必须重跑。
//
// 返回 64 位小写十六进制，与 t_transcode_job.input_hash、t_transcode_job_step.input_hash 同口径比较。
func computeJobInputHash(job model.TranscodeJob) string {
	parts := []string{
		jobInputHashVersion,
		"url=" + strings.TrimSpace(job.SourceURL),
		fmt.Sprintf("seg_duration=%d", job.SegmentDurationSec),
		fmt.Sprintf("segment_template=%s", job.SegmentTemplate),
		fmt.Sprintf("dash=%t", job.SupportDash),
		fmt.Sprintf("hls=%t", job.SupportHLS),
		fmt.Sprintf("output=%d|%s", job.OutputStorageID, job.OutputBasePrefix),
		fmt.Sprintf("watermark=%t|%s|%d|%.6f|%.6f|%.6f|%.6f",
			job.EnableWatermark, job.WatermarkImageURL, job.WatermarkAnchor,
			job.WatermarkXRatio, job.WatermarkYRatio, job.WatermarkWidthRatio, job.WatermarkOpacity),
		fmt.Sprintf("thumb_sprite=%t|%d|%d|%d|%d|%d|%s|%s",
			job.EnableThumbnailSprite, job.ThumbRows, job.ThumbCols, job.ThumbIntervalSec,
			job.ThumbWidth, job.ThumbHeight, job.ThumbImageFormat, job.ThumbStoragePrefix),
		fmt.Sprintf("thumb_binary=%t|%s|%d",
			job.EnableThumbnailBinaryIndex, job.ThumbBinaryStoragePrefix, job.ThumbBinaryMaxSizeBytes),
	}

	/* 清晰度的先后顺序不影响产物，按名字排序后再拼：否则"顺序变了"会被误判成"输入变了"。 */
	names := make([]string, 0, len(job.Renditions))
	byName := make(map[string]model.RenditionOption, len(job.Renditions))

	for _, rendition := range job.Renditions {
		names = append(names, rendition.Name)
		byName[rendition.Name] = rendition
	}

	sort.Strings(names)

	for _, name := range names {
		rendition := byName[name]
		parts = append(parts, fmt.Sprintf("rendition=%s|%d|%d|%s|%d|%d|%d|%s",
			rendition.Name, rendition.Width, rendition.Height, rendition.VideoCodec,
			rendition.VideoBitrateKbps, rendition.VideoMaxrateKbps, rendition.VideoBufsizeKbps,
			rendition.Preset))
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))

	return hex.EncodeToString(sum[:])
}
