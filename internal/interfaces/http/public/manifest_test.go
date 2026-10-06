package public

import (
	"net/http/httptest"
	"testing"
)

func TestParsePathUint(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/manifest/dash/123", nil)
	req.SetPathValue("job_id", "123")

	jobID, err := parsePathUint(req, "job_id")
	if err != nil {
		t.Fatalf("parse job_id failed: %v", err)
	}
	if jobID != 123 {
		t.Fatalf("unexpected job_id: %d", jobID)
	}
}

func TestParsePathString(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/manifest/hls/123/1080p", nil)
	req.SetPathValue("rendition", "1080p")

	rendition := parsePathString(req, "rendition")
	if rendition != "1080p" {
		t.Fatalf("unexpected rendition: %q", rendition)
	}
}

func TestParsePathStringKeepsRawPathValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/manifest/dash/123.txt", nil)
	req.SetPathValue("job_id", "123.txt")

	if value := parsePathString(req, "job_id"); value != "123.txt" {
		t.Fatalf("unexpected path value: %q", value)
	}
}
