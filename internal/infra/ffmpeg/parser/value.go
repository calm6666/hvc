package parser

import (
	"strconv"
	"strings"
)

func ParseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func ParseFloat64(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func ParseFPS(raw string) float64 {
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) == 2 {
		num := ParseFloat64(parts[0])
		den := ParseFloat64(parts[1])
		if den > 0 {
			return num / den
		}
	}
	return ParseFloat64(raw)
}

func ParseBitrateKbps(raw string) int {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "bits/s")
	raw = strings.TrimSuffix(raw, "kb/s")
	raw = strings.TrimSuffix(raw, "Mb/s")
	raw = strings.TrimSuffix(raw, "GB/s")
	raw = strings.TrimSpace(raw)
	val := ParseFloat64(raw)
	if val == 0 {
		return 0
	}
	return int(val)
}

func ParseDurationToMS(raw string) int64 {
	if raw == "" || raw == "N/A" {
		return 0
	}
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) != 3 {
		return int64(ParseFloat64(raw) * 1000)
	}
	h := ParseInt64(parts[0])
	m := ParseInt64(parts[1])
	s := ParseFloat64(parts[2])
	return h*3600000 + m*60000 + int64(s*1000)
}
