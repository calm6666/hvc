package capability

// SupportsHardwareFilter 判断是否支持硬件滤镜。
func SupportsHardwareFilter(filters []string, target string) bool {
	for _, filter := range filters {
		if filter == target {
			return true
		}
	}
	return false
}
