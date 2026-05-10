//go:build linux

package hoststats

import (
	"os"
	"strconv"
	"strings"
)

func memoryUsagePercent() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var totalKB uint64
	var availableKB uint64
	for _, line := range splitNonEmptyLines(data) {
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB = parseMeminfoValue(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			availableKB = parseMeminfoValue(line)
		}
	}
	if totalKB == 0 || availableKB > totalKB {
		return 0
	}
	usedKB := totalKB - availableKB
	return int((usedKB * 100) / totalKB)
}

func totalMemoryMB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range splitNonEmptyLines(data) {
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB := parseMeminfoValue(line)
			if totalKB == 0 {
				return 0
			}
			return int(totalKB / 1024)
		}
	}
	return 0
}

func parseMeminfoValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	value, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return value
}
