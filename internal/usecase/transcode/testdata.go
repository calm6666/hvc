package transcode

import "hvc/internal/model"

func TestCreateJobRequest() model.CreateJobRequest {
	return model.CreateJobRequest{
		RequestID: "req_test_0001",
		SourceURL: "https://cdn.example.com/test.mp4",
		Priority:  10,
	}
}
