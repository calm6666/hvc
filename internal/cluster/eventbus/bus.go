package eventbus

import (
	"context"
	"sync"
)

// Bus 表示进程内事件总线。
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan any
}

// NewBus 创建事件总线。
func NewBus() *Bus {
	return &Bus{subscribers: make(map[string][]chan any)}
}

// Publish 发布事件。
func (b *Bus) Publish(ctx context.Context, topic string, payload any) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers[topic] {
		select {
		case <-ctx.Done():
			return
		case ch <- payload:
		default:
		}
	}
}

// Subscribe 订阅事件。
func (b *Bus) Subscribe(topic string, buffer int) <-chan any {
	if buffer <= 0 {
		buffer = 1
	}
	ch := make(chan any, buffer)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	return ch
}
