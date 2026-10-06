package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"hvc/pkg/logx"
)

// computeFileSHA256 计算本地文件的 SHA-256（小写十六进制）。
//
// 用途（B3）：分片在转码产出后就把内容摘要算出来随分片行落库，这样
//   - 发布前的逐片校验有**内容级**凭据（而不只是对象存储的 ETag/大小回执）；
//   - 清单可以把每片摘要一起物化出去，下游据此校验拉到的分片内容。
//
// 为什么在生产侧算一次而不是发布前重算：分片产出后内容不会再变，而输出目录在任务收尾时会被
// 清理（cleanupOutputDir），发布时本地文件可能已不存在。发布前重算只能靠回源下载整片，
// 代价与整片上传相当，不适合放在发布路径上 —— 那是门禁（B6）该做的事。
//
// 读取失败返回空串：调用方据此把该片判为"没有内容凭据"，发布校验会拒绝通过，
// 而不是把空摘要当成合法值。
func computeFileSHA256(path string) string {
	if path == "" {
		return ""
	}

	file, err := os.Open(path)
	if err != nil {
		logx.Error("worker.segment.digest_open_failed", err, logx.Fields{
			"path": path,
		})
		return ""
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		logx.Error("worker.segment.digest_read_failed", err, logx.Fields{
			"path": path,
		})
		return ""
	}

	return hex.EncodeToString(hasher.Sum(nil))
}
