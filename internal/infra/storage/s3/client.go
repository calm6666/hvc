package storage

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"hvc/internal/config"
	"hvc/internal/model"
)

// Client 表示对象存储客户端。
type Client struct {
	engine     *minio.Client
	bucket     string
	basePrefix string
}

// Open 打开对象存储连接。
func Open(cfg config.StorageConfig) (*Client, error) {
	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if cfg.UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: parsed.Scheme == "https",
	})
	if err != nil {
		return nil, err
	}
	return &Client{engine: client, bucket: cfg.Bucket, basePrefix: cfg.BasePrefix}, nil
}

// BuildObjectKey 生成对象键。
func (c *Client) BuildObjectKey(segment model.Segment) string {
	return path.Join(c.basePrefix, fmt.Sprintf("job-%d", segment.JobID), segment.ObjectKey)
}

// Upload 上传对象。
func (c *Client) Upload(ctx context.Context, objectKey string, localPath string) (string, uint64, error) {
	info, err := c.engine.FPutObject(ctx, c.bucket, objectKey, localPath, minio.PutObjectOptions{})
	if err != nil {
		return "", 0, err
	}
	return info.ETag, uint64(info.Size), nil
}
