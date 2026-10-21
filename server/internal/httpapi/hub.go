package httpapi

import (
	"context"
	"sync"
)

type subscriber struct {
	userID string
	send   chan []byte
}

type Hub struct {
	mu      sync.Mutex
	clients map[*subscriber]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[*subscriber]struct{}{}}
}

func (h *Hub) Add(userID string) *subscriber {
	sub := &subscriber{userID: userID, send: make(chan []byte, 8)}
	h.mu.Lock()
	h.clients[sub] = struct{}{}
	h.mu.Unlock()
	return sub
}

func (h *Hub) Remove(sub *subscriber) {
	h.mu.Lock()
	delete(h.clients, sub)
	h.mu.Unlock()
}

func (h *Hub) Dispatch(ctx context.Context, payload []byte, allow func(ctx context.Context, userID string) bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.clients {
		if !allow(ctx, sub.userID) {
			continue
		}
		select {
		case sub.send <- payload:
		default:
		}
	}
}
