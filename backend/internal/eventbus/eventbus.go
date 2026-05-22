package eventbus

import "sync"

type TaskEvent struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type Bus struct {
	mu          sync.RWMutex
	subscribers map[chan TaskEvent]struct{}
}

func New() *Bus {
	return &Bus{
		subscribers: make(map[chan TaskEvent]struct{}),
	}
}

func (b *Bus) Subscribe() chan TaskEvent {
	ch := make(chan TaskEvent, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan TaskEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *Bus) Publish(event TaskEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
