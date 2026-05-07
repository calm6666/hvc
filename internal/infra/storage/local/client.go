package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hvc/pkg/logx"
)

// Client 本地文件存储客户端。
//
// 实现与 S3 客户端相同的 Upload 接口，
// 将分片文件复制到本地指定目录。
// 通过后台配置 StorageType="local" 启用。
type Client struct {
	basePath string
}

// NewClient 创建本地存储客户端。
func NewClient(basePath string) *Client {
	if basePath == "" {
		basePath = "/data/hvc/media"
	}
	return &Client{basePath: basePath}
}

// Upload 将本地文件复制到存储路径。
//
// objectKey 作为相对路径拼接到 basePath 后面。
// 自动创建所需的目录结构。
func (c *Client) Upload(ctx context.Context, objectKey string, localPath string) (string, uint64, error) {
	targetPath := filepath.Join(c.basePath, objectKey)
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", 0, fmt.Errorf("创建目录失败 %s: %w", targetDir, err)
	}

	src, err := os.Open(localPath)
	if err != nil {
		return "", 0, fmt.Errorf("打开源文件失败 %s: %w", localPath, err)
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("获取源文件信息失败: %w", err)
	}
	_ = stat

	dst, err := os.Create(targetPath)
	if err != nil {
		return "", 0, fmt.Errorf("创建目标文件失败 %s: %w", targetPath, err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		return "", 0, fmt.Errorf("复制文件失败: %w", err)
	}

	etag := computeLocalETag(targetPath)
	logx.Info("storage.local.uploaded", logx.Fields{
		"object_key":    objectKey,
		"target_path":   targetPath,
		"size_bytes":    written,
	})

	return etag, uint64(written), nil
}

// Download 从本地存储读取文件。
func (c *Client) Download(ctx context.Context, objectKey string) ([]byte, error) {
	targetPath := filepath.Join(c.basePath, objectKey)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败 %s: %w", targetPath, err)
	}
	return data, nil
}

// Delete 删除本地存储文件。
func (c *Client) Delete(ctx context.Context, objectKey string) error {
	targetPath := filepath.Join(c.basePath, objectKey)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败 %s: %w", targetPath, err)
	}
	return nil
}

// Exists 检查文件是否存在。
func (c *Client) Exists(ctx context.Context, objectKey string) bool {
	targetPath := filepath.Join(c.basePath, objectKey)
	_, err := os.Stat(targetPath)
	return err == nil
}

// BuildObjectKey 构建对象存储键。
func (c *Client) BuildObjectKey(prefix, rendition string, segmentType string, sequenceNo int) string {
	if prefix != "" {
		return fmt.Sprintf("%s/%s/%s-%d.m4s", prefix, rendition, segmentType, sequenceNo)
	}
	return fmt.Sprintf("%s/%s-%d.m4s", rendition, segmentType, sequenceNo)
}

// BasePath 返回存储根路径。
func (c *Client) BasePath() string {
	return c.basePath
}

func computeLocalETag(path string) string {
	stat, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("local-%d", stat.Size())
}

// ResolveStorageClient 根据存储类型返回对应的存储客户端。
//
// storageType="s3" 返回 S3 客户端，storageType="local" 返回本地客户端。
// 这是后台配置驱动的存储切换入口。
func ResolveStorageClient(storageType string, s3Client interface{}, localClient *Client) Uploader {
	switch strings.ToLower(storageType) {
	case "local":
		return localClient
	default:
		if s3Uploader, ok := s3Client.(Uploader); ok {
			return s3Uploader
		}
		return localClient
	}
}

// Uploader 统一上传接口，S3 和本地存储都实现此接口。
type Uploader interface {
	Upload(ctx context.Context, objectKey string, localPath string) (string, uint64, error)
}
