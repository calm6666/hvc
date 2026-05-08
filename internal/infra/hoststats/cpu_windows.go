//go:build windows

package hoststats

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32DLL        = syscall.NewLazyDLL("kernel32.dll")
	getSystemTimesProc = kernel32DLL.NewProc("GetSystemTimes")
)

type windowsCPUSampler struct {
	mu       sync.Mutex
	lastIdle uint64
	lastBusy uint64
	inited   bool
}

func newCPUSampler() cpuSampler {
	return &windowsCPUSampler{}
}

func (s *windowsCPUSampler) Percent() int {
	idle, busy, ok := readWindowsCPUTimes()
	if !ok {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.inited {
		s.lastIdle = idle
		s.lastBusy = busy
		s.inited = true
		return 0
	}
	idleDelta := idle - s.lastIdle
	busyDelta := busy - s.lastBusy
	totalDelta := idleDelta + busyDelta
	s.lastIdle = idle
	s.lastBusy = busy
	if totalDelta == 0 {
		return 0
	}
	return int((busyDelta * 100) / totalDelta)
}

func readWindowsCPUTimes() (uint64, uint64, bool) {
	var idle syscall.Filetime
	var kernel syscall.Filetime
	var user syscall.Filetime
	ret, _, _ := getSystemTimesProc.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return 0, 0, false
	}
	idleValue := filetimeToUint64(idle)
	kernelValue := filetimeToUint64(kernel)
	userValue := filetimeToUint64(user)
	busyValue := (kernelValue - idleValue) + userValue
	return idleValue, busyValue, true
}

func filetimeToUint64(value syscall.Filetime) uint64 {
	return (uint64(value.HighDateTime) << 32) | uint64(value.LowDateTime)
}
