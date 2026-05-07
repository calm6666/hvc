//go:build !darwin && !linux && !windows

package gpu

// probePlatform 为未知平台保留空实现。
//
// 当当前编译目标不属于 darwin / linux / windows 时，统一返回空结果，
// 让上层自动走 fallback 路径，避免因为缺少平台文件而无法编译。
func probePlatform() []ProbeResult {
	return nil
}
