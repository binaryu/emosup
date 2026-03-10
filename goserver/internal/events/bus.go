package events

import (
	"sync"

	"emosup/goserver/internal/domain"
)

type Bus struct {
	mu          sync.RWMutex
	subscribers map[chan domain.Event]struct{}
	buffer      int
}

func NewBus(buffer int) *Bus {
	if buffer <= 0 {
		buffer = 1
	}
	return &Bus{
		subscribers: make(map[chan domain.Event]struct{}),
		buffer:      buffer,
	}
}

func (b *Bus) Subscribe() chan domain.Event {
	ch := make(chan domain.Event, b.buffer)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan domain.Event) {
	b.mu.Lock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *Bus) Publish(event domain.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
