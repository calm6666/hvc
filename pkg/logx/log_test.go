package logx

import (
	"errors"
	"testing"
	"time"
)

type fakeBatchSink struct {
	items     []PersistedEntry
	failTimes int
	calls     int
}

func (s *fakeBatchSink) SavePersistedEntry(item PersistedEntry) error {
	s.items = append(s.items, item)
	return nil
}

func (s *fakeBatchSink) SavePersistedEntries(items []PersistedEntry) error {
	s.calls++
	if s.failTimes > 0 {
		s.failTimes--
		return errors.New("temporary failure")
	}
	s.items = append(s.items, items...)
	return nil
}

func TestSavePersistedEntriesWithRetry(t *testing.T) {
	sink := &fakeBatchSink{failTimes: 2}
	items := []PersistedEntry{{Action: "a"}, {Action: "b"}}
	start := time.Now()
	if err := savePersistedEntriesWithRetry(sink.SavePersistedEntries, items); err != nil {
		t.Fatalf("expected retry success, got err: %v", err)
	}
	if sink.calls != 3 {
		t.Fatalf("unexpected call count: %d", sink.calls)
	}
	if len(sink.items) != 2 {
		t.Fatalf("unexpected persisted size: %d", len(sink.items))
	}
	if time.Since(start) < 400*time.Millisecond {
		t.Fatal("expected retry backoff to be applied")
	}
}

func TestResolvePersistRetryBackoff(t *testing.T) {
	if got := resolvePersistRetryBackoff(0); got != 100*time.Millisecond {
		t.Fatalf("unexpected backoff 0: %s", got)
	}
	if got := resolvePersistRetryBackoff(1); got != 300*time.Millisecond {
		t.Fatalf("unexpected backoff 1: %s", got)
	}
	if got := resolvePersistRetryBackoff(2); got != time.Second {
		t.Fatalf("unexpected backoff 2: %s", got)
	}
}

func TestResolveHTTPLogPolicy(t *testing.T) {
	testCases := []struct {
		name                string
		method              string
		path                string
		captureRequestBody  bool
		captureResponseBody bool
	}{
		{
			name:                "healthz omits bodies",
			method:              "GET",
			path:                "/healthz",
			captureRequestBody:  false,
			captureResponseBody: false,
		},
		{
			name:                "manifest omits bodies",
			method:              "GET",
			path:                "/v1/manifest/dash/1.mpd",
			captureRequestBody:  false,
			captureResponseBody: false,
		},
		{
			name:                "internal heartbeat omits bodies",
			method:              "POST",
			path:                "/v1/internal/worker/heartbeat",
			captureRequestBody:  false,
			captureResponseBody: false,
		},
		{
			name:                "create job keeps bodies",
			method:              "POST",
			path:                "/v1/transcode/job/create",
			captureRequestBody:  true,
			captureResponseBody: true,
		},
	}
	for _, tc := range testCases {
		policy := resolveHTTPLogPolicy(tc.method, tc.path)
		if policy.captureRequestBody != tc.captureRequestBody {
			t.Fatalf("%s: unexpected request policy", tc.name)
		}
		if policy.captureResponseBody != tc.captureResponseBody {
			t.Fatalf("%s: unexpected response policy", tc.name)
		}
	}
}
