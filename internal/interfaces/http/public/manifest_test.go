package public

import (
	"net/http/httptest"
	"testing"
)

func TestParsePathUintWithSuffix(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/manifest/dash/123.mpd", nil)
	req.SetPathValue("job_id", "123.mpd")

	jobID, err := parsePathUintWithSuffix(req, "job_id", ".mpd")
	if err != nil {
		t.Fatalf("parse job_id failed: %v", err)
	}
	if jobID != 123 {
		t.Fatalf("unexpected job_id: %d", jobID)
	}
}

func TestParsePathStringWithSuffix(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/manifest/hls/123/1080p.m3u8", nil)
	req.SetPathValue("rendition", "1080p.m3u8")

	rendition := parsePathStringWithSuffix(req, "rendition", ".m3u8")
	if rendition != "1080p" {
		t.Fatalf("unexpected rendition: %q", rendition)
	}
}

func TestParsePathStringWithSuffixRejectsUnexpectedSuffix(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/manifest/dash/123.txt", nil)
	req.SetPathValue("job_id", "123.txt")

	if value := parsePathStringWithSuffix(req, "job_id", ".mpd"); value != "" {
		t.Fatalf("unexpected path value: %q", value)
	}
}
