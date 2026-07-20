package eventbus

import "sync"

type TaskEvent struct {
	TaskID   string  `json:"task_id"`
	Status   string  `json:"status"`
	DlProg   float64 `json:"dl_prog,omitempty"`
	DlSpeed  int64   `json:"dl_speed,omitempty"`
	DlDone   int64   `json:"dl_done,omitempty"`
	DlTotal  int64   `json:"dl_total,omitempty"`
	UlProg   float64 `json:"ul_prog,omitempty"`
	UlSpeed  int64   `json:"ul_speed,omitempty"`
	UlDone   int64   `json:"ul_done,omitempty"`
	UlTotal  int64   `json:"ul_total,omitempty"`
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
	// Larger buffer absorbs short UI freezes without dropping progress ticks.
	ch := make(chan TaskEvent, 256)
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
			// Channel full: drop one stale event then push latest so UI keeps freshest progress.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- event:
			default:
			}
		}
	}
}
