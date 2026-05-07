package uploader

import (
	"context"
	"math"
	"time"

	"hvc/internal/infra/storage/s3"
	"hvc/internal/model"
	"hvc/pkg/logx"
	"sync"
)

type WorkerPool struct {
	concurrency    int
	client         *storage.Client
	maxRetryCount  int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

func NewWorkerPool(concurrency int, client *storage.Client) *WorkerPool {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &WorkerPool{
		concurrency:    concurrency,
		client:         client,
		maxRetryCount:  3,
		retryBaseDelay: 1 * time.Second,
		retryMaxDelay:  30 * time.Second,
	}
}

// SetRetryPolicy 设置上传重试策略。
func (p *WorkerPool) SetRetryPolicy(maxRetry int, baseDelay, maxDelay time.Duration) {
	if maxRetry > 0 {
		p.maxRetryCount = maxRetry
	}
	if baseDelay > 0 {
		p.retryBaseDelay = baseDelay
	}
	if maxDelay > 0 {
		p.retryMaxDelay = maxDelay
	}
}

// Run 执行上传任务，带指数退避重试。
//
// 每个任务在独立 goroutine 中执行，通过信号量控制并发数。
// 上传失败时，按指数退避策略重试，最多重试 maxRetryCount 次。
func (p *WorkerPool) Run(ctx context.Context, tasks []model.UploadTask) []model.UploadResult {
	results := make([]model.UploadResult, 0, len(tasks))
	resultCh := make(chan model.UploadResult, len(tasks))
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(task model.UploadTask) {
			defer wg.Done()
			defer func() { <-sem }()
			result := p.uploadWithRetry(ctx, task)
			resultCh <- result
		}(task)
	}
	wg.Wait()
	close(resultCh)
	for result := range resultCh {
		results = append(results, result)
	}
	return results
}

// uploadWithRetry 带指数退避重试的上传。
//
// 退避策略：delay = baseDelay * 2^attempt，上限为 maxDelay。
// 每次重试前等待退避时间，避免对存储服务造成瞬时压力。
func (p *WorkerPool) uploadWithRetry(ctx context.Context, task model.UploadTask) model.UploadResult {
	var lastErr string
	for attempt := 0; attempt <= p.maxRetryCount; attempt++ {
		select {
		case <-ctx.Done():
			return model.UploadResult{SegmentID: task.SegmentID, Success: false, ErrorMessage: "context cancelled"}
		default:
		}

		if attempt > 0 {
			delay := p.calculateBackoff(attempt)
			logx.Info("uploader.retry.backoff", logx.Fields{
				"segment_id": task.SegmentID,
				"attempt":    attempt,
				"delay_ms":   delay.Milliseconds(),
			})
			select {
			case <-ctx.Done():
				return model.UploadResult{SegmentID: task.SegmentID, Success: false, ErrorMessage: "context cancelled during backoff"}
			case <-time.After(delay):
			}
		}

		etag, size, err := p.client.Upload(ctx, task.ObjectKey, task.LocalPath)
		if err == nil {
			return model.UploadResult{SegmentID: task.SegmentID, Success: true, ObjectETag: etag, ObjectSizeBytes: size}
		}
		lastErr = err.Error()
		logx.Info("uploader.retry.failed", logx.Fields{
			"segment_id":    task.SegmentID,
			"attempt":       attempt + 1,
			"max_retry":     p.maxRetryCount,
			"error_message": lastErr,
		})
	}
	return model.UploadResult{SegmentID: task.SegmentID, Success: false, ErrorMessage: lastErr}
}

// calculateBackoff 计算指数退避延迟。
func (p *WorkerPool) calculateBackoff(attempt int) time.Duration {
	delay := time.Duration(float64(p.retryBaseDelay) * math.Pow(2, float64(attempt-1)))
	if delay > p.retryMaxDelay {
		delay = p.retryMaxDelay
	}
	return delay
}
