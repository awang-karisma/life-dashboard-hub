package handlers

import (
	"sync"
	"time"
)

type IngestionEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	DeviceID   uint      `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Source     string    `json:"source"`
	SHA256     string    `json:"sha256"`
	Records    int64     `json:"records"`
	Tombstones int64     `json:"tombstones"`
	LatencyMs  int64     `json:"latency_ms"`
	Status     string    `json:"status"`
}

type IngestionBus struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan IngestionEvent
	nextID      uint64
	recent      []IngestionEvent // ring buffer, last 50
}

func NewIngestionBus() *IngestionBus {
	return &IngestionBus{
		subscribers: make(map[uint64]chan IngestionEvent),
		recent:      make([]IngestionEvent, 0, 50),
	}
}

func (b *IngestionBus) Publish(event IngestionEvent) {
	b.mu.Lock()
	if len(b.recent) >= 50 {
		b.recent = b.recent[1:]
	}
	b.recent = append(b.recent, event)

	subs := make([]chan IngestionEvent, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		subs = append(subs, ch)
	}
	b.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if channel full
		}
	}
}

func (b *IngestionBus) Subscribe() (uint64, chan IngestionEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID
	ch := make(chan IngestionEvent, 16)
	b.subscribers[id] = ch
	return id, ch
}

func (b *IngestionBus) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(ch)
	}
}

func (b *IngestionBus) Recent() []IngestionEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]IngestionEvent, len(b.recent))
	copy(out, b.recent)
	return out
}
