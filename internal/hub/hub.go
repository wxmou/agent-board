package hub

import (
	"encoding/json"
	"sync"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type subscriber struct {
	ch   chan Event
	done chan struct{}
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
}

func New() *Hub {
	return &Hub{subscribers: make(map[*subscriber]struct{})}
}

func (h *Hub) Subscribe() (<-chan Event, func()) {
	sub := &subscriber{ch: make(chan Event, 64), done: make(chan struct{})}
	h.mu.Lock()
	h.subscribers[sub] = struct{}{}
	h.mu.Unlock()
	cancel := func() {
		h.mu.Lock()
		if _, ok := h.subscribers[sub]; ok {
			delete(h.subscribers, sub)
			close(sub.done)
		}
		h.mu.Unlock()
	}
	return sub.ch, cancel
}

func (h *Hub) Publish(eventType string, data any) {
	event := Event{Type: eventType, Data: data, Timestamp: time.Now().UTC()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subscribers {
		select {
		case sub.ch <- event:
		case <-sub.done:
		default:
			// A slow dashboard must not stall writes. It reconnects and reloads.
		}
	}
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// Close ends all active streams so HTTP graceful shutdown does not wait for
// long-lived SSE connections.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subscribers {
		close(sub.ch)
		delete(h.subscribers, sub)
	}
}

func Marshal(event Event) ([]byte, error) {
	return json.Marshal(event)
}
