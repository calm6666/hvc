package bootstrap

import (
	"context"
	"sync"

	"hvc/pkg/logx"
)

type managedTask struct {
	name       string
	mu         sync.Mutex
	running    bool
	currentKey string
	cancel     context.CancelFunc
}

func (t *managedTask) Reconcile(parent context.Context, enabled bool, key string, start func(context.Context) error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !enabled {
		t.stopLocked()
		return
	}
	if t.running && t.currentKey == key {
		return
	}
	t.stopLocked()

	ctx, cancel := context.WithCancel(parent)
	t.cancel = cancel
	t.running = true
	t.currentKey = key
	go func(expectedKey string) {
		if err := start(ctx); err != nil && ctx.Err() == nil {
			logx.Error("managed_task.failed", err, logx.Fields{
				"task": t.name,
				"key":  expectedKey,
			})
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.currentKey == expectedKey {
			t.running = false
			t.currentKey = ""
			t.cancel = nil
		}
	}(key)
}

func (t *managedTask) stopLocked() {
	if t.cancel != nil {
		t.cancel()
	}
	t.cancel = nil
	t.running = false
	t.currentKey = ""
}
