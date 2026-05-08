//go:build windows

package hoststats

import "unsafe"

var globalMemoryStatusExProc = kernel32DLL.NewProc("GlobalMemoryStatusEx")

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

func memoryUsagePercent() int {
	status := memoryStatusEx{}
	status.length = uint32(unsafe.Sizeof(status))
	ret, _, _ := globalMemoryStatusExProc.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return 0
	}
	return int(status.memoryLoad)
}
