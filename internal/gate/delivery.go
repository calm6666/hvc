package gate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"hvc/internal/model"
)

/*
 * 本文件是门禁的第三条：A1 —— "回调送达 ⇒ 清单引用的每个分片都能 GET 到且 sha256 通过"。
 *
 * 它与 A4–A7 的区别：那几条判的是"库里的记录自不自洽"，这一条必须真的**去对象存储取内容**复算，
 * 否则证明不了"下游按清单去拉能拿到正确内容"。
 *
 * 因此这里不依赖具体的存储客户端实现，只声明一个最小能力接口 ObjectReader，由调用方用真实客户端
 * （worker 侧已经在用的 storage.Client）适配一层。这样门禁可以：
 *   - 在单测里用假实现覆盖成功/摘要不符/取不到三种分支；
 *   - 在没有配置对象存储的环境里明确返回 Skipped（**不计入通过**），而不是假装验过。
 */

// ObjectReader 门禁读取对象内容所需的最小能力。
//
// 实现方（适配器）约定：
//   - 返回该对象的**完整**字节；分片体积有上限（一片几秒的媒体），可以整体读进内存；
//   - 对象不存在、网络失败、权限不足等一律以 error 返回，不要用空切片加 nil 错误表示失败。
type ObjectReader interface {
	ReadObject(ctx context.Context, objectKey string) ([]byte, error)
}

// RunSegmentFetchAndDigest A1（推导）：逐片取回对象、复算 sha256，并与分片行里记录的摘要比对。
//
// 判据口径：
//   - 参与判定的分片 = 分片行里非 init 的媒体分片 + init 段（init 段同样是清单引用的对象，
//     下游一样要拉到才能起播，所以一并校验）；
//   - 分片行没有摘要（SHA256 为空）⇒ 直接失败，而不是跳过：没有摘要就无法证明内容正确；
//   - 任何一片取不到 ⇒ 失败（附对象 Key，便于定位是哪个清晰度的哪一片）。
//
// reader 为 nil（没有配置对象存储）⇒ Skipped，明确不算通过。
func RunSegmentFetchAndDigest(ctx context.Context, reader ObjectReader, segments []model.Segment) CheckResult {
	const id = "A1"
	const title = "清单引用的每个分片可 GET 且 sha256 通过（推导）"

	if reader == nil {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "未注入对象存储读取器，无法判定"}
	}

	if len(segments) == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有分片行，无法判定"}
	}

	verified := 0

	for _, segment := range segments {
		if segment.ObjectKey == "" {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片 seq=%d（%s）没有对象 Key，无法取回", segment.SequenceNo, segment.RenditionName),
			}
		}

		body, err := reader.ReadObject(ctx, segment.ObjectKey)
		if err != nil {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片取回失败：key=%q（清晰度 %s seq=%d）：%v",
					segment.ObjectKey, segment.RenditionName, segment.SequenceNo, err),
			}
		}

		if segment.SHA256 == "" {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片没有记录摘要，无法证明内容正确：key=%q（清晰度 %s seq=%d）",
					segment.ObjectKey, segment.RenditionName, segment.SequenceNo),
			}
		}

		sum := sha256.Sum256(body)
		actual := hex.EncodeToString(sum[:])

		if actual != segment.SHA256 {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片内容摘要不符：key=%q（清晰度 %s seq=%d）实际=%s 记录=%s",
					segment.ObjectKey, segment.RenditionName, segment.SequenceNo, actual, segment.SHA256),
			}
		}

		if segment.ObjectSizeBytes != 0 && uint64(len(body)) != segment.ObjectSizeBytes {
			return CheckResult{
				ID: id, Title: title,
				Detail: fmt.Sprintf("分片大小不符：key=%q（清晰度 %s seq=%d）实际=%d 记录=%d",
					segment.ObjectKey, segment.RenditionName, segment.SequenceNo,
					len(body), segment.ObjectSizeBytes),
			}
		}

		verified++
	}

	if verified == 0 {
		return CheckResult{ID: id, Title: title, Skipped: true, Detail: "没有可校验的分片"}
	}

	return CheckResult{ID: id, Title: title, Passed: true}
}
