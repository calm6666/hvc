package videotoolbox

import "hvc/internal/model"

// HWAccelName 返回 Apple VideoToolbox 硬件加速名。
//
// 之所以单独建目录，而不是只在 probe_darwin.go 里散落字符串，
// 是为了让 internal/infra/gpu 下的硬件加速目录结构保持一致：
// - nvidia/
// - qsv/
// - amf/
// - vaapi/
// - videotoolbox/
//
// 这样后续无论是 probe、调度匹配、命令构建还是能力快照写库，
// 都可以复用统一的“硬件类型名入口”，避免字符串常量继续四散在各层代码里。
func HWAccelName() string { return model.ExecutionHWAppleVideoToolbox }
