//go:build linux

package hoststats

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

type linuxCPUSampler struct {
	mu       sync.Mutex
	lastIdle uint64
	lastBusy uint64
	inited   bool
}

func newCPUSampler() cpuSampler {
	return &linuxCPUSampler{}
}

func (s *linuxCPUSampler) Percent() int {
	idle, busy, ok := readLinuxCPUTimes()
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

func readLinuxCPUTimes() (uint64, uint64, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	lines := splitNonEmptyLines(data)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "cpu ") {
		return 0, 0, false
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0, 0, false
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return 0, 0, false
		}
		values = append(values, value)
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	busy := total - idle
	return idle, busy, true
}
