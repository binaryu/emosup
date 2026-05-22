package scheduler

import "sync"

type TaskEvent struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan TaskEvent]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan TaskEvent]struct{}),
	}
}

func (b *EventBus) Subscribe() chan TaskEvent {
	ch := make(chan TaskEvent, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBus) Unsubscribe(ch chan TaskEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *EventBus) Publish(event TaskEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
