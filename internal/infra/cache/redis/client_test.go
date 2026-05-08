package redis

import "testing"

func TestParseRedisInfoField(t *testing.T) {
	info := "# Server\nredis_version:7.2.5\nredis_mode:cluster\n"

	if got := parseRedisInfoField(info, "redis_version"); got != "7.2.5" {
		t.Fatalf("parse redis version failed, got=%q", got)
	}
	if got := parseRedisInfoField(info, "redis_mode"); got != "cluster" {
		t.Fatalf("parse redis mode failed, got=%q", got)
	}
	if got := parseRedisInfoField(info, "unknown_field"); got != "" {
		t.Fatalf("unknown field should be empty, got=%q", got)
	}
}
