//go:build darwin

package hoststats

import (
	"strconv"
	"strings"
	"sync"
)

type darwinCPUSampler struct {
	mu       sync.Mutex
	lastIdle uint64
	lastBusy uint64
	inited   bool
}

func newCPUSampler() cpuSampler {
	return &darwinCPUSampler{}
}

func (s *darwinCPUSampler) Percent() int {
	idle, busy, ok := readDarwinCPUTimes()
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

func readDarwinCPUTimes() (uint64, uint64, bool) {
	lines := splitNonEmptyLines(commandOutput("sh", "-c", "iostat -C 1 2 | tail -n 1"))
	if len(lines) == 0 {
		return 0, 0, false
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 3 {
		return 0, 0, false
	}
	userPercent, err1 := strconv.ParseFloat(fields[len(fields)-3], 64)
	sysPercent, err2 := strconv.ParseFloat(fields[len(fields)-2], 64)
	idlePercent, err3 := strconv.ParseFloat(fields[len(fields)-1], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, false
	}
	idle := uint64(idlePercent * 100)
	busy := uint64((userPercent + sysPercent) * 100)
	return idle, busy, true
}
