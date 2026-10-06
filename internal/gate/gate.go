// Package gate 提供转码交付的自动门禁：把验收判据做成可 go test 跑的 Go 判定。
//
// 【判据来源必须如实交代】
// 本包最初的 12 条（A1–A12）来自需求描述里"转码设计 §3.4 的 A1–A12 验收判据"，
// 但该文档在本仓库与相邻目录（hvc、转码脚本、video_conversion、vod_live_transcoding、
// hvc/.claude 及各 agent worktree）中都不存在（sql/107 引用的 docs/TRANSCODE-SERVICE-DESIGN.md
// 也没有随仓库提交）。因此本包现有判据是**从设计与现有实现推导**出来的，逐条在 Title 里标注
// "推导"；拿到原文后按条替换即可 —— 每条判定都是独立的纯函数，替换不影响其它条。
//
// 【判定原则】
//   - 只做判定，不修数据：门禁的输出是"通过/不通过 + 具体原因"，不替业务改状态。
//   - 需要外部资源（对象存储 GET、HTTP 拉清单、硬件信息）的判据一律通过参数注入客户端；
//     客户端缺失时返回 Skipped（明确不算通过），避免"没验=通过"。
//   - 纯数据判据（分片行、回调载荷、任务行）不依赖任何外部资源，可在单测里直接构造。
package gate

import (
	"fmt"

	"hvc/internal/model"
)

// CheckResult 单条判据的判定结果。
type CheckResult struct {
	// ID 判据编号（A1、A4…）。
	ID string
	// Title 判据内容；凡是从设计推导而来、而非引自原文的，标题里会带"（推导）"。
	Title string
	// Passed 是否通过。Skipped 或 NotApplicable 为真时本字段无意义。
	Passed bool
	// Skipped 表示本次运行**无法判定**（缺外部资源/缺数据），**不计入通过**。
	Skipped bool
	// NotApplicable 表示这条判据**在本次情形下不适用**（例如 A3 只管软解，硬解路径天然不适用）。
	// 与 Skipped 的区别必须分清：Skipped 是"该判却没判成"，NotApplicable 是"本就不该判"。
	// 两者都不计入通过，但报告时语义不同 —— 全部 NotApplicable 也不能宣称"已验收"。
	NotApplicable bool
	// Detail 失败/跳过/不适用时的具体原因（含定位信息），通过时可为空。
	Detail string
}

// RunManifestConsistency 判定"清单与分片是否自洽"这一组判据（A4/A5/A6/A7）。
//
// 输入：
//   - job      任务行：提供切片时长基准（A4）与任务号（用于报错定位）
//   - segments 该任务的全部分片行（t_transcode_segment）
//   - payload  完成回调载荷：清单对外声明的分片与摘要（A7 用它比对）
//
// 为什么把这四条放在一起：它们判定的是同一份事实的不同侧面 —— "清单里说的"与"库里存的"、
// "库里存的"与"对象存储收到的"是否一致。四条都只需要纯数据，可在单测里构造，不需要外部资源。
func RunManifestConsistency(job model.TranscodeJob, segments []model.Segment, payload model.TranscodeCompletedPayload) []CheckResult {
	results := []CheckResult{
		checkSegmentDurationWithinManifest(job, segments),
		checkSegmentSequenceContinuous(segments),
		checkSegmentUploadEvidence(segments),
		checkPayloadDigestsMatchRows(segments, payload),
	}

	return results
}

// checkSegmentDurationWithinManifest A4（推导）：每片时长应与任务约定的切片时长一致。
//
// 判据口径：允许 ±1 秒偏差（编码器按关键帧对齐会带来不足一片的尾片与轻微漂移），
// 尾片允许短于约定时长（源就剩这么多）。超出即为清单与配置不一致。
func checkSegmentDurationWithinManifest(job model.TranscodeJob, segments []model.Segment) CheckResult {
	const id = "A4"
	const title = "切片时长与约定一致（推导）"

	if job.SegmentDurationSec <= 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "任务未约定切片时长，无法判定"}
	}

	expectedMS := job.SegmentDurationSec * 1000
	toleranceMS := 1000

	for _, segment := range segments {
		if segment.IsInitSegment || segment.DurationMS <= 0 {
			continue
		}

		/* 尾片（最后一片）短于约定时长是正常的：源剩余不足一片。 */
		if segment.DurationMS < expectedMS-toleranceMS {
			/* 只有"非尾片偏短"才算问题，这里用序号判断是否还有后续片。 */
			if hasSegmentAfter(segments, segment.SequenceNo) {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("分片 seq=%d 时长 %dms 明显短于约定 %dms（且不是尾片）",
						segment.SequenceNo, segment.DurationMS, expectedMS),
				}
			}

			continue
		}

		if segment.DurationMS > expectedMS+toleranceMS {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片 seq=%d 时长 %dms 超出约定 %dms（容差 %dms）",
					segment.SequenceNo, segment.DurationMS, expectedMS, toleranceMS),
			}
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// checkSegmentSequenceContinuous A5（推导）：同一清晰度内媒体分片的序号必须连续、无缺口。
//
// 缺口意味着清单会少片（播放到那里会卡住或跳），所以必须挡住；init 段不参与编号判定。
func checkSegmentSequenceContinuous(segments []model.Segment) CheckResult {
	const id = "A5"
	const title = "分片序号连续无缺口（推导）"

	byRendition := make(map[string][]int)

	for _, segment := range segments {
		if segment.IsInitSegment {
			continue
		}

		key := segment.RenditionName
		if key == "" {
			key = fmt.Sprintf("rid=%d", segment.RenditionID)
		}

		byRendition[key] = append(byRendition[key], segment.SequenceNo)
	}

	if len(byRendition) == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有媒体分片，无法判定"}
	}

	for rendition, numbers := range byRendition {
		if len(numbers) == 0 {
			continue
		}

		/* 用出现过的序号最小值..最大值做连续性检查：重复序号同样视为不连续（会写坏清单）。 */
		seen := make(map[int]struct{}, len(numbers))
		minSeq, maxSeq := numbers[0], numbers[0]

		for _, number := range numbers {
			if _, duplicated := seen[number]; duplicated {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("清晰度 %s 的分片序号重复：seq=%d", rendition, number),
				}
			}

			seen[number] = struct{}{}

			if number < minSeq {
				minSeq = number
			}

			if number > maxSeq {
				maxSeq = number
			}
		}

		for expected := minSeq; expected <= maxSeq; expected++ {
			if _, ok := seen[expected]; !ok {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("清晰度 %s 的分片序号缺口：缺少 seq=%d（范围 %d..%d）",
						rendition, expected, minSeq, maxSeq),
				}
			}
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// checkSegmentUploadEvidence A6（推导）：每片都必须带上对象存储的回执（状态、Key、ETag、大小）。
//
// 为什么把"有回执"当判据：MarkUploaded 只在对象存储 PUT 成功返回后被调用，ETag/大小就是对象存储
// 对**这一片**的确认。缺任何一项都意味着"这一片是否真的传上去了"无法证明。
func checkSegmentUploadEvidence(segments []model.Segment) CheckResult {
	const id = "A6"
	const title = "分片上传有对象存储回执（推导）"

	if len(segments) == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有分片行，无法判定"}
	}

	for _, segment := range segments {
		if segment.UploadStatus != model.SegmentUploaded {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片 seq=%d（%s）上传状态为 %d，不是已上传",
					segment.SequenceNo, segment.RenditionName, segment.UploadStatus),
			}
		}

		if segment.ObjectKey == "" || segment.ObjectETag == "" || segment.ObjectSizeBytes == 0 {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片 seq=%d（%s）缺少对象回执：key=%q etag=%q size=%d",
					segment.SequenceNo, segment.RenditionName, segment.ObjectKey,
					segment.ObjectETag, segment.ObjectSizeBytes),
			}
		}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// checkPayloadDigestsMatchRows A7（推导）：回调载荷里声明的每片摘要，必须与分片行里存的一致。
//
// 为什么需要这条：载荷是下游/门禁用来复算校验的依据（A1 的"sha256 通过"就靠它）。
// 如果载荷里的摘要与库里不符（漏填、错配、被改），下游的校验就失去了意义。
func checkPayloadDigestsMatchRows(segments []model.Segment, payload model.TranscodeCompletedPayload) CheckResult {
	const id = "A7"
	const title = "回调载荷的分片摘要与库中一致（推导）"

	rowDigests := make(map[string]string, len(segments))

	for _, segment := range segments {
		if segment.ObjectKey == "" {
			continue
		}

		rowDigests[segment.ObjectKey] = segment.SHA256
	}

	declared := 0

	for _, rendition := range payload.Renditions {
		for _, digest := range rendition.Segments {
			declared++

			rowDigest, found := rowDigests[digest.ObjectKey]
			if !found {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("载荷声明的分片在库中不存在：key=%q（清晰度 %s）",
						digest.ObjectKey, rendition.RenditionName),
				}
			}

			if digest.SHA256 == "" {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("载荷未带摘要：key=%q（清晰度 %s）",
						digest.ObjectKey, rendition.RenditionName),
				}
			}

			if rowDigest == "" {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("库中该分片没有摘要：key=%q（清晰度 %s）",
						digest.ObjectKey, rendition.RenditionName),
				}
			}

			if rowDigest != digest.SHA256 {
				return CheckResult{
					ID: id, Title: title,
					Detail: fmt.Sprintf("摘要不一致：key=%q 载荷=%s 库中=%s",
						digest.ObjectKey, digest.SHA256, rowDigest),
				}
			}
		}
	}

	if declared == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "载荷没有声明任何分片摘要，无法判定"}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}

// hasSegmentAfter 判断同一清晰度里是否还存在序号更大的媒体分片（用于识别"尾片"）。
func hasSegmentAfter(segments []model.Segment, sequenceNo int) bool {
	for _, segment := range segments {
		if segment.IsInitSegment {
			continue
		}

		if segment.SequenceNo > sequenceNo {
			return true
		}
	}

	return false
}
