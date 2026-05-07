package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"hvc/pkg/logx"
)

var defaultClient = &http.Client{
	Timeout: 30 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

type DownloadResult struct {
	FilePath    string
	ContentSize int64
	StatusCode  int
}

func Fetch(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hvc-worker/1.0")
	return defaultClient.Do(req)
}

func FetchHead(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hvc-worker/1.0")
	return defaultClient.Do(req)
}

func DownloadToFile(ctx context.Context, url string, destPath string) (DownloadResult, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return DownloadResult{}, fmt.Errorf("create dest dir failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "hvc-worker/1.0")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("execute request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return DownloadResult{StatusCode: resp.StatusCode}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(destPath)
		return DownloadResult{}, fmt.Errorf("write file failed: %w", err)
	}

	logx.Info("fetcher.download.completed", logx.Fields{
		"url":          url,
		"dest_path":    destPath,
		"content_size": written,
	})

	return DownloadResult{
		FilePath:    destPath,
		ContentSize: written,
		StatusCode:  resp.StatusCode,
	}, nil
}

func DownloadRange(ctx context.Context, url string, destPath string, offset int64, length int64) (DownloadResult, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return DownloadResult{}, fmt.Errorf("create dest dir failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "hvc-worker/1.0")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))

	resp, err := defaultClient.Do(req)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("execute range request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return DownloadResult{StatusCode: resp.StatusCode}, fmt.Errorf("unexpected status code for range request: %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(destPath)
		return DownloadResult{}, fmt.Errorf("write file failed: %w", err)
	}

	return DownloadResult{
		FilePath:    destPath,
		ContentSize: written,
		StatusCode:  resp.StatusCode,
	}, nil
}

func ContentLength(ctx context.Context, url string) (int64, error) {
	resp, err := FetchHead(ctx, url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	cl := resp.ContentLength
	if cl <= 0 {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			parts := splitContentRange(cr)
			if len(parts) == 3 {
				total, _ := strconv.ParseInt(parts[2], 10, 64)
				if total > 0 {
					return total, nil
				}
			}
		}
	}
	return cl, nil
}

func splitContentRange(cr string) []string {
	for i, ch := range cr {
		if ch == ' ' {
			spec := cr[i+1:]
			parts := make([]string, 0, 3)
			current := ""
			for _, c := range spec {
				if c == '/' || c == '-' {
					parts = append(parts, current)
					current = ""
				} else {
					current += string(c)
				}
			}
			if current != "" {
				parts = append(parts, current)
			}
			return parts
		}
	}
	return nil
}
