package timex

import "time"

// NowUnixMilli 返回当前毫秒时间戳。
func NowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// ParseRFC3339 解析 RFC3339 时间。
func ParseRFC3339(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}
