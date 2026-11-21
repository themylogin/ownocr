package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	subscriberBuffer = 16
	writeTimeout     = 5 * time.Second
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type Hub struct {
	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	origins     []string
}

type subscriber struct {
	events chan []byte
}

func NewHub(origins []string) *Hub {
	return &Hub{
		subscribers: make(map[*subscriber]struct{}),
		origins:     origins,
	}
}

func (h *Hub) Publish(eventType string, payload any) {
	message, err := json.Marshal(Event{Type: eventType, Payload: payload})
	if err != nil {
		slog.Error("marshal event", "type", eventType, "error", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		select {
		case sub.events <- message:
		default:
			// A subscriber that cannot keep up is dropped; it reconnects and refetches.
			delete(h.subscribers, sub)
			close(sub.events)
		}
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		slog.Warn("websocket accept", "error", err)
		return
	}
	defer conn.CloseNow()

	sub := &subscriber{events: make(chan []byte, subscriberBuffer)}
	h.add(sub)
	defer h.remove(sub)

	ctx := conn.CloseRead(r.Context())

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-sub.events:
			if !ok {
				conn.Close(websocket.StatusPolicyViolation, "subscriber too slow")
				return
			}
			if err := write(ctx, conn, message); err != nil {
				return
			}
		}
	}
}

func write(ctx context.Context, conn *websocket.Conn, message []byte) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, message)
}

func (h *Hub) add(sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[sub] = struct{}{}
}

func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

func (h *Hub) remove(sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers, sub)
}
