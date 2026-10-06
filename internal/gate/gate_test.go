package gate

import (
	"testing"

	"hvc/internal/model"
)

/* 下面这些构造只用一个清晰度的两片媒体 + 一个 init 段，足够覆盖 A4/A5/A6/A7 的判定分支。 */

func sampleJob() model.TranscodeJob {
	return model.TranscodeJob{JobID: 101, SegmentDurationSec: 6}
}

func sampleSegments() []model.Segment {
	return []model.Segment{
		{
			JobID: 101, RenditionName: "720p", SegmentType: "init", IsInitSegment: true,
			ObjectKey: "out/720p/init.mp4", ObjectSizeBytes: 800, ObjectETag: "etag-init",
			SHA256: "aaaa", UploadStatus: model.SegmentUploaded,
		},
		{
			JobID: 101, RenditionName: "720p", SequenceNo: 1, DurationMS: 6000,
			ObjectKey: "out/720p/1.m4s", ObjectSizeBytes: 1024, ObjectETag: "etag-1",
			SHA256: "bbbb", UploadStatus: model.SegmentUploaded,
		},
		{
			JobID: 101, RenditionName: "720p", SequenceNo: 2, DurationMS: 4200,
			ObjectKey: "out/720p/2.m4s", ObjectSizeBytes: 900, ObjectETag: "etag-2",
			SHA256: "cccc", UploadStatus: model.SegmentUploaded,
		},
	}
}

func samplePayload() model.TranscodeCompletedPayload {
	return model.TranscodeCompletedPayload{
		JobID:      101,
		Renditions: []model.CompletedRendition{
			{
				RenditionName: "720p",
				SegmentCount:  2,
				Segments: []model.CompletedSegmentDigest{
					{ObjectKey: "out/720p/1.m4s", SHA256: "bbbb", SizeBytes: 1024},
					{ObjectKey: "out/720p/2.m4s", SHA256: "cccc", SizeBytes: 900},
				},
			},
		},
	}
}

func resultByID(results []CheckResult, id string) CheckResult {
	for _, result := range results {
		if result.ID == id {
			return result
		}
	}

	return CheckResult{ID: id, Detail: "判据未运行"}
}

func TestManifestConsistencyAllPass(t *testing.T) {
	results := RunManifestConsistency(sampleJob(), sampleSegments(), samplePayload())

	for _, result := range results {
		if result.Skipped {
			t.Fatalf("%s 不应被跳过：%s", result.ID, result.Detail)
		}

		if !result.Passed {
			t.Fatalf("%s 应该通过，实际失败：%s", result.ID, result.Detail)
		}
	}

	if len(results) != 4 {
		t.Fatalf("期望 4 条判据（A4/A5/A6/A7），实际 %d 条", len(results))
	}
}

func TestSequenceGapDetected(t *testing.T) {
	segments := sampleSegments()
	/* 把第 2 片的序号改成 3：出现缺口 seq=2。 */
	segments[2].SequenceNo = 3

	result := resultByID(RunManifestConsistency(sampleJob(), segments, samplePayload()), "A5")

	if result.Passed || result.Skipped {
		t.Fatalf("A5 应该报缺口，实际：passed=%v skipped=%v detail=%s",
			result.Passed, result.Skipped, result.Detail)
	}
}

func TestUploadEvidenceMissingDetected(t *testing.T) {
	segments := sampleSegments()
	/* 抹掉第 1 片的 ETag：等于"对象存储没给回执"，A6 必须挡住。 */
	segments[1].ObjectETag = ""

	result := resultByID(RunManifestConsistency(sampleJob(), segments, samplePayload()), "A6")

	if result.Passed || result.Skipped {
		t.Fatalf("A6 应该报缺回执，实际：passed=%v skipped=%v detail=%s",
			result.Passed, result.Skipped, result.Detail)
	}
}

func TestDigestMismatchDetected(t *testing.T) {
	payload := samplePayload()
	payload.Renditions[0].Segments[0].SHA256 = "not-the-same"

	result := resultByID(RunManifestConsistency(sampleJob(), sampleSegments(), payload), "A7")

	if result.Passed || result.Skipped {
		t.Fatalf("A7 应该报摘要不一致，实际：passed=%v skipped=%v detail=%s",
			result.Passed, result.Skipped, result.Detail)
	}
}

func TestShortNonTailSegmentDetected(t *testing.T) {
	segments := sampleSegments()
	/* 第 1 片只有 2 秒，而它后面还有第 2 片 ⇒ 不是尾片 ⇒ A4 必须报。 */
	segments[1].DurationMS = 2000

	result := resultByID(RunManifestConsistency(sampleJob(), segments, samplePayload()), "A4")

	if result.Passed || result.Skipped {
		t.Fatalf("A4 应该报非尾片过短，实际：passed=%v skipped=%v detail=%s",
			result.Passed, result.Skipped, result.Detail)
	}
}

func TestShortTailSegmentAllowed(t *testing.T) {
	segments := sampleSegments()
	/* 尾片（最后一片）短于约定时长是正常的，不能被 A4 判失败。 */
	segments[2].DurationMS = 1500

	result := resultByID(RunManifestConsistency(sampleJob(), segments, samplePayload()), "A4")

	if !result.Passed {
		t.Fatalf("尾片过短应被允许，实际：%s", result.Detail)
	}
}

func TestMissingExpectationSkipsInsteadOfPassing(t *testing.T) {
	/* 任务没有约定切片时长 ⇒ A4 必须 Skipped（明确不算通过），而不是默认通过。 */
	job := sampleJob()
	job.SegmentDurationSec = 0

	result := resultByID(RunManifestConsistency(job, sampleSegments(), samplePayload()), "A4")

	if !result.Skipped {
		t.Fatalf("缺少判定依据时应 Skipped，实际：passed=%v detail=%s", result.Passed, result.Detail)
	}

	if result.Passed {
		t.Fatal("Skipped 的结果不能被当成通过")
	}
}
