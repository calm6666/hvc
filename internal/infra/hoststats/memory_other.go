//go:build !linux && !windows && !darwin

package hoststats

func memoryUsagePercent() int {
	return 0
}

func totalMemoryMB() int {
	return 0
}
