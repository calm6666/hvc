package transcode

import (
	"testing"
	"time"

	"hvc/pkg/idgen"
)

func TestCreateJobUseCase_Execute(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)
	useCase := NewCreateJobUseCase()
	result, err := useCase.Execute(TestCreateJobRequest())
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Job.JobID == 0 {
		t.Fatalf("job id should not be zero")
	}
	if result.Job.RequestID == "" {
		t.Fatalf("request id should not be empty")
	}
	if result.Job.Status != 2 {
		t.Fatalf("unexpected job status: %d", result.Job.Status)
	}
}
