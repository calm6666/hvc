package gate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hvc/internal/model"
)

/*
 * 本文件是门禁的"接进运行链路"所需的两个东西：
 *   1. HTTPManifestFetcher —— 真实的清单取回器（ManifestFetcher 的 HTTP 实现）；
 *   2. RunAll / Report —— 一次把 12 项检查跑完的聚合入口，供发布路径调用。
 *
 * 与判定函数的分工：判定逻辑全在 gate.go / flow.go / delivery.go / serving.go / runtime.go 里，
 * 本文件只负责"把真实依赖喂进去、把结果收拢成一份可读的报告"。
 */

// HTTPManifestFetcher 用 HTTP 取回清单文本（ManifestFetcher 的真实实现）。
//
// baseURL 是清单服务的根地址（例如 http://127.0.0.1:8080），载荷里的清单 URL 是相对路径
// （形如 /v1/manifest/dash/9），这里拼成绝对地址再取。
type HTTPManifestFetcher struct {
	baseURL string
	client  *http.Client

	// maxBodyBytes 单个清单的体积上限：清单是文本，正常几 KB；设上限避免异常响应把内存吃满。
	maxBodyBytes int64
}

// NewHTTPManifestFetcher 创建清单取回器。timeout 为单次请求超时。
func NewHTTPManifestFetcher(baseURL string, timeout time.Duration) *HTTPManifestFetcher {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPManifestFetcher{
		baseURL:      strings.TrimRight(baseURL, "/"),
		client:       &http.Client{Timeout: timeout},
		maxBodyBytes: 4 << 20, // 4MiB
	}
}

// FetchManifest 取回一份清单文本。非 2xx 一律返回错误（不要把错误页当成清单内容）。
func (f *HTTPManifestFetcher) FetchManifest(ctx context.Context, url string) ([]byte, error) {
	if f == nil || f.client == nil {
		return nil, fmt.Errorf("清单取回器未初始化")
	}

	target := url

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = f.baseURL + "/" + strings.TrimLeft(target, "/")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}

	response, err := f.client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("清单返回 %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, f.maxBodyBytes))
	if err != nil {
		return nil, err
	}

	return body, nil
}

// Report 一次门禁运行的全部结果。
type Report struct {
	JobID   uint64
	Results []CheckResult
}

// Passed 判断是否全部通过。
//
// 语义必须严格：Skipped（该判却没判成）与 NotApplicable（本就不该判）都**不算通过** ——
// 所以"全部通过"要求每一条都是 Passed 且没有 Skipped / NotApplicable。
func (r Report) Passed() bool {
	if len(r.Results) == 0 {
		return false
	}

	for _, result := range r.Results {
		if !result.Passed || result.Skipped || result.NotApplicable {
			return false
		}
	}

	return true
}

// Blockers 返回阻止"全部通过"的条目（失败 / 跳过 / 不适用），供调用方写日志或决定是否拦发布。
func (r Report) Blockers() []CheckResult {
	blockers := make([]CheckResult, 0, len(r.Results))

	for _, result := range r.Results {
		if result.Passed && !result.Skipped && !result.NotApplicable {
			continue
		}

		blockers = append(blockers, result)
	}

	return blockers
}

// RunAllInput 跑一次全量门禁所需的全部输入。
//
// 需要外部资源的部分（对象内容、清单、运行时观测）都是可空注入：为 nil / 零值时对应检查会返回
// Skipped，而不是伪造通过。
type RunAllInput struct {
	Job         model.TranscodeJob
	Payload     model.TranscodeCompletedPayload
	Segments    []model.Segment
	OutboxEvents []model.OutboxEvent
	ProgressSnapshots []model.ProgressSnapshot
	Steps       []StepRecord
	Observation EncoderObservation
	Reader      ObjectReader
	Fetcher     ManifestFetcher
}

// RunAll 跑完 12 项检查并收拢成报告。
//
// 顺序与 gate 各文件的定义一致：交付可拉取（分片取回/清单）→ 运行时（硬编/软解 CPU）→
// 清单与分片自洽 → 回调与续跑与失败收敛与进度。
func RunAll(ctx context.Context, input RunAllInput) Report {
	results := make([]CheckResult, 0, 12)

	/* 交付层：内容能否真的拉到、清单是否可用。 */
	results = append(results, RunSegmentFetchAndDigest(ctx, input.Reader, input.Segments))
	results = append(results, RunManifestServing(ctx, input.Fetcher, input.Payload))

	/* 运行时：实际怎么跑的。 */
	results = append(results, RunHardwareEncodePreference(input.Observation))
	results = append(results, RunSoftwareDecodeCPULimit(input.Observation))

	/* 清单与分片自洽。 */
	results = append(results, RunManifestConsistency(input.Job, input.Segments, input.Payload)...)

	/* 通知、续跑、失败与进度。 */
	results = append(results, RunCallbackIdempotency(input.Job.JobID, input.OutboxEvents))
	results = append(results, RunResumeConsistency(input.Job.InputHash, input.Steps))
	results = append(results, RunFailureConvergence(input.Job))
	results = append(results, RunProgressSanity(input.ProgressSnapshots))

	return Report{JobID: input.Job.JobID, Results: results}
}
