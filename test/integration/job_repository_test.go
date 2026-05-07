package integration

import (
	"context"
	"testing"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
)

func TestJobRepositoryCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := config.MySQLConfig{
		DSN: "root:root@tcp(127.0.0.1:3306)/hvc?parseTime=true&charset=utf8mb4",
	}
	db, err := mysql.Open(cfg)
	if err != nil {
		t.Skipf("skipping: database not available: %v", err)
	}

	repo := mysql.NewJobRepository(db)
	ctx := context.Background()

	job := model.TranscodeJob{
		RequestID:          "test-integration-001",
		SourceURL:          "https://example.com/test.mp4",
		Status:             model.JobStatusQueued,
		Priority:           0,
		SegmentDurationSec: 6,
		SupportDash:        true,
		SupportHLS:         true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	t.Run("CreateAndQuery", func(t *testing.T) {
		err := repo.Save(ctx, job)
		if err != nil {
			t.Fatalf("failed to save job: %v", err)
		}

		found, ok := repo.FindByRequestID(ctx, job.RequestID)
		if !ok {
			t.Fatal("expected to find created job")
		}
		if found.RequestID != job.RequestID {
			t.Fatalf("expected request_id=%s, got %s", job.RequestID, found.RequestID)
		}
	})

	t.Run("ListQueued", func(t *testing.T) {
		jobs := repo.ListQueued(ctx)
		if len(jobs) == 0 {
			t.Fatal("expected at least one queued job")
		}
	})

	t.Run("MarkCompleted", func(t *testing.T) {
		jobs := repo.ListQueued(ctx)
		if len(jobs) > 0 {
			err := repo.MarkCompleted(ctx, jobs[0].JobID)
			if err != nil {
				t.Fatalf("failed to mark completed: %v", err)
			}
		}
	})
}
