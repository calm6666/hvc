package uploader

import (
	"context"
	"hvc/internal/infra/storage/s3"
	"hvc/internal/model"
	"sync"
)

// WorkerPool 表示上传工作池。
type WorkerPool struct {
	concurrency int
	client      *storage.Client
}

// NewWorkerPool 创建上传工作池。
func NewWorkerPool(concurrency int, client *storage.Client) *WorkerPool {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &WorkerPool{concurrency: concurrency, client: client}
}

// Run 执行上传任务。
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
			select {
			case <-ctx.Done():
				return
			default:
				etag, size, err := p.client.Upload(ctx, task.ObjectKey, task.LocalPath)
				if err != nil {
					resultCh <- model.UploadResult{SegmentID: task.SegmentID, Success: false, ErrorMessage: err.Error()}
					return
				}
				resultCh <- model.UploadResult{SegmentID: task.SegmentID, Success: true, ObjectETag: etag, ObjectSizeBytes: size}
			}
		}(task)
	}
	wg.Wait()
	close(resultCh)
	for result := range resultCh {
		results = append(results, result)
	}
	return results
}
