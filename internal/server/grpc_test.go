package server

import (
	"testing"

	transcodev1 "hvc/api/pb/transcodev1"
)

func TestToModelCreateJobRequestIncludesCallbackURL(t *testing.T) {
	req := &transcodev1.CreateJobRequest{
		RequestId:   "req-1",
		SourceUrl:   "https://example.com/video.mp4",
		CallbackUrl: "https://callback.example.com/task/req-1",
	}

	modelReq := toModelCreateJobRequest(req)
	if modelReq.CallbackURL != "https://callback.example.com/task/req-1" {
		t.Fatalf("unexpected callback url: %s", modelReq.CallbackURL)
	}
}
