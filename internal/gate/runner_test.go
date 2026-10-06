package gate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hvc/internal/model"
)

/* ---------- HTTPManifestFetcher ---------- */

func TestHTTPManifestFetcherJoinsRelativeURL(t *testing.T) {
	var seenPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer server.Close()

	fetcher := NewHTTPManifestFetcher(server.URL, 2*time.Second)
	body, err := fetcher.FetchManifest(context.Background(), "/v1/manifest/hls/9")

	if err != nil {
		t.Fatalf("取回失败：%v", err)
	}

	if seenPath != "/v1/manifest/hls/9" {
		t.Fatalf("相对路径拼接后应请求 %s，实际 %s", "/v1/manifest/hls/9", seenPath)
	}

	if string(body) != "#EXTM3U\n" {
		t.Fatalf("返回内容不符：%q", string(body))
	}
}

func TestHTTPManifestFetcherRejectsNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewHTTPManifestFetcher(server.URL, 2*time.Second)

	if _, err := fetcher.FetchManifest(context.Background(), "/v1/manifest/hls/9"); err == nil {
		t.Fatal("非 2xx 必须返回错误（不能把错误页当清单内容）")
	}
}

func TestHTTPManifestFetcherKeepsAbsoluteURL(t *testing.T) {
	var seenHost string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		_, _ = w.Write([]byte("<MPD/>"))
	}))
	defer server.Close()

	fetcher := NewHTTPManifestFetcher("http://example.invalid", 2*time.Second)
	body, err := fetcher.FetchManifest(context.Background(), server.URL+"/absolute.mpd")

	if err != nil {
		t.Fatalf("绝对地址应直接使用：%v", err)
	}

	if string(body) != "<MPD/>" || seenHost == "" {
		t.Fatalf("绝对地址取回异常：body=%q host=%q", string(body), seenHost)
	}
}

/* ---------- RunAll / Report ---------- */

/*
 * runnerSegments 是聚合用例**自带**的完整夹具。
 *
 * 为什么不复用 delivery_test 里的 fetchSegments：那份夹具只服务 A1 一条判据，是"最小可用"的
 * （没有 ETag 等 A6 需要的字段）。聚合入口要同时满足 12 项检查，复用最小夹具会让聚合用例
 * 依赖别的测试文件的内部细节 —— 已经因此连错三次（硬解标记、init 段 ETag、媒体分片 ETag）。
 */
func runnerSegments() []model.Segment {
	return []model.Segment{
		{
			RenditionName: "720p", SegmentType: "init", IsInitSegment: true, SequenceNo: 0,
			ObjectKey: "out/720p/init.mp4", ObjectSizeBytes: uint64(len("INIT")),
			ObjectETag: "etag-init", SHA256: digestOf("INIT"),
			UploadStatus: model.SegmentUploaded,
		},
		{
			RenditionName: "720p", SequenceNo: 1, DurationMS: 6000,
			ObjectKey: "out/720p/1.m4s", ObjectSizeBytes: uint64(len("SEG1")),
			ObjectETag: "etag-1", SHA256: digestOf("SEG1"),
			UploadStatus: model.SegmentUploaded,
		},
	}
}

/* runAllInput 构造一份"全部检查都能判定"的输入：本地假读取器 + 本地假取回器 + 完整观测。 */
func runAllInput() RunAllInput {
	segments := runnerSegments()

	payload := model.TranscodeCompletedPayload{
		JobID:      9,
		Renditions: []model.CompletedRendition{{RenditionName: "720p", SegmentCount: 1}},
	}
	payload.Renditions[0].Segments = []model.CompletedSegmentDigest{
		{ObjectKey: "out/720p/1.m4s", SHA256: digestOf("SEG1"), SizeBytes: uint64(len("SEG1"))},
	}

	return RunAllInput{
		Job:      model.TranscodeJob{JobID: 9, Status: model.JobStatusPublished, SegmentDurationSec: 6, InputHash: "h1"},
		Payload:  payload,
		Segments: segments,
		OutboxEvents: []model.OutboxEvent{
			{EventID: 1, EventType: "transcode.completed", JobID: 9},
		},
		ProgressSnapshots: []model.ProgressSnapshot{
			{JobID: 9, Status: model.JobStatusPublished, ProgressPermille: 1000},
		},
		Steps: []StepRecord{
			{Step: "PROBE", State: StepDone, InputHash: "h1"},
			{Step: "UPLOAD", State: StepDone, InputHash: "h1"},
		},
		Observation: EncoderObservation{
			/* 软解路径 + 峰值远低于上限：A2（硬编）与 A3（软解 CPU 上限）都应真正通过。
			   注意不能写 HardwareDecode=true —— 那会让 A3 判为"不适用"，
			   而"不适用"按设计不算通过（见 TestRunAllNotApplicableIsNotPassed）。 */
			JobID: 9, Encoder: "h264_nvenc", HardwareDecode: false,
			DecodeFallbackReason: "源不支持硬解，已记录例外",
			CPUPeakPercent:       22.5, CPUSamples: 8,
		},
		Reader: &fakeReader{
			objects: map[string][]byte{"out/720p/init.mp4": []byte("INIT"), "out/720p/1.m4s": []byte("SEG1")},
			errs:    map[string]error{},
		},
		Fetcher: &fakeFetcher{
			bodies: map[string]string{
				"/v1/manifest/dash/9":     `<MPD><Representation id="720p"/></MPD>`,
				"/v1/manifest/hls/9":      "#EXTM3U\n720p\n",
				"/v1/manifest/hls/9/720p": "#EXTM3U\n",
			},
			errs: map[string]error{},
		},
	}
}

func TestRunAllPassesWhenEverythingIsSound(t *testing.T) {
	input := runAllInput()
	/* 清单 URL 挂在清晰度上（与载荷真实结构一致）。 */
	input.Payload.Renditions[0].ManifestDashURL = "/v1/manifest/dash/9"
	input.Payload.Renditions[0].ManifestHLSURL = "/v1/manifest/hls/9"
	input.Payload.Renditions[0].ManifestHLSVariantURL = "/v1/manifest/hls/9/720p"

	report := RunAll(context.Background(), input)

	if !report.Passed() {
		for _, blocker := range report.Blockers() {
			t.Errorf("未通过：%s %s -> %s", blocker.ID, blocker.Title, blocker.Detail)
		}

		t.Fatal("全部条件齐备时应整体通过")
	}
}

func TestRunAllBlocksWhenSoftwareEncodeWithoutReason(t *testing.T) {
	input := runAllInput()
	input.Payload.Renditions[0].ManifestDashURL = "/v1/manifest/dash/9"
	input.Payload.Renditions[0].ManifestHLSURL = "/v1/manifest/hls/9"
	input.Payload.Renditions[0].ManifestHLSVariantURL = "/v1/manifest/hls/9/720p"
	input.Observation.Encoder = "libx264"
	input.Observation.HardwareDecode = false
	input.Observation.DecodeFallbackReason = ""
	input.Observation.CPUSamples = 8

	report := RunAll(context.Background(), input)

	if report.Passed() {
		t.Fatal("无理由软编必须拦住整体通过")
	}

	blockers := report.Blockers()

	foundHardware := false

	for _, blocker := range blockers {
		if blocker.ID == "A2" {
			foundHardware = true
		}
	}

	if !foundHardware {
		t.Fatalf("Blockers 里应包含硬件编码那一条，实际：%+v", blockers)
	}
}

func TestRunAllNotApplicableIsNotPassed(t *testing.T) {
	input := runAllInput()
	input.Payload.Renditions[0].ManifestDashURL = "/v1/manifest/dash/9"
	input.Payload.Renditions[0].ManifestHLSURL = "/v1/manifest/hls/9"
	input.Payload.Renditions[0].ManifestHLSVariantURL = "/v1/manifest/hls/9/720p"

	report := RunAll(context.Background(), input)

	/* 把观测改成硬解路径：A3 变成"不适用" ⇒ 即便其它检查都过，整体也不算通过。 */
	input.Observation.HardwareDecode = true

	report = RunAll(context.Background(), input)

	if report.Passed() {
		t.Fatal("存在 NotApplicable 时不能判为整体通过（否则等于宣称已验收）")
	}

	blockedByA3 := false

	for _, blocker := range report.Blockers() {
		if blocker.ID == "A3" && blocker.NotApplicable {
			blockedByA3 = true
		}
	}

	if !blockedByA3 {
		t.Fatal("A3 在硬解路径下应作为不适用出现在 Blockers 里")
	}
}

func TestReportWithoutResultsIsNotPassed(t *testing.T) {
	if (Report{JobID: 1}).Passed() {
		t.Fatal("空报告不能算通过")
	}
}
