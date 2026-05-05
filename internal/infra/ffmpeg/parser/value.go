package parser

import "strconv"

// ParseInt64 解析 int64。
func ParseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}
