package storage

import (
	"context"
	"path"
	"strconv"
	"strings"

	"hvc/internal/config"
	localstorage "hvc/internal/infra/storage/local"
	s3storage "hvc/internal/infra/storage/s3"
	"hvc/internal/model"
)

// Uploader 表示统一的对象上传能力。
//
// Worker 运行期不应直接绑定某一种具体存储实现，
// 而应通过这一层根据 runtime config 选择本地存储或 S3。
type Uploader interface {
	Upload(ctx context.Context, objectKey string, localPath string) (string, uint64, error)
}

// Client 表示统一的存储客户端。
//
// 该对象屏蔽了本地存储与 S3 的差异，
// 并统一维护对象键生成逻辑，避免上层继续感知具体存储类型。
type Client struct {
	uploader   Uploader
	basePrefix string
}

// Open 根据运行时配置创建存储客户端。
func Open(cfg config.StorageConfig) (*Client, error) {
	if strings.EqualFold(cfg.StorageType, "local") {
		return &Client{
			uploader:   localstorage.NewClient(cfg.LocalBasePath),
			basePrefix: cfg.BasePrefix,
		}, nil
	}

	client, err := s3storage.Open(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		uploader:   client,
		basePrefix: cfg.BasePrefix,
	}, nil
}

// BuildObjectKey 为不同存储后端生成统一对象键。
func (c *Client) BuildObjectKey(segment model.Segment) string {
	return path.Join(c.basePrefix, "job-"+strconv.FormatUint(segment.JobID, 10), segment.ObjectKey)
}

// Upload 上传对象到当前存储后端。
func (c *Client) Upload(ctx context.Context, objectKey string, localPath string) (string, uint64, error) {
	return c.uploader.Upload(ctx, objectKey, localPath)
}
