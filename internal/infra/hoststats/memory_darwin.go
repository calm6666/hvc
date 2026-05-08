//go:build darwin

package hoststats

import (
	"strconv"
	"strings"
)

func memoryUsagePercent() int {
	totalText := string(commandOutput("sysctl", "-n", "hw.memsize"))
	totalBytes, err := strconv.ParseUint(strings.TrimSpace(totalText), 10, 64)
	if err != nil || totalBytes == 0 {
		return 0
	}
	lines := splitNonEmptyLines(commandOutput("vm_stat"))
	if len(lines) == 0 {
		return 0
	}
	var pageSize uint64 = 4096
	var freePages uint64
	for _, line := range lines {
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for idx, field := range fields {
				if field == "of" && idx+1 < len(fields) {
					pageSize, _ = strconv.ParseUint(fields[idx+1], 10, 64)
					break
				}
			}
		}
		if strings.HasPrefix(line, "Pages free:") {
			freePages = parseVMStatValue(line)
		}
	}
	freeBytes := freePages * pageSize
	if freeBytes > totalBytes {
		return 0
	}
	usedBytes := totalBytes - freeBytes
	return int((usedBytes * 100) / totalBytes)
}

func parseVMStatValue(line string) uint64 {
	line = strings.TrimSpace(strings.TrimSuffix(strings.SplitN(line, ":", 2)[1], "."))
	value, err := strconv.ParseUint(strings.ReplaceAll(line, ".", ""), 10, 64)
	if err != nil {
		return 0
	}
	return value
}
