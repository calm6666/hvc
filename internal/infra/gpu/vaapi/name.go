package vaapi

import "hvc/internal/model"

// HWAccelName 返回 VAAPI 硬件加速名。
//
// 当前项目里 Linux 平台的“其它硬件加速”主要先落到 VAAPI，
// 因为它既覆盖 Intel iGPU 的常见路径，也能承载部分 AMD 路径的统一归一化。
// 单独建目录的目的和 videotoolbox 一样，都是为了让 internal/infra/gpu 的目录结构完整、对称且可复用。
func HWAccelName() string { return model.ExecutionHWVAAPI }
