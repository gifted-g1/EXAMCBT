// Package realtime pushes live proctoring events to staff dashboards over
// WebSocket. One goroutine (Run) owns all room state, so there is no shared
// map to race on; producers talk to it only through channels.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// Event is the envelope sent to dashboards.
type Event struct {
	Type   string    `json:"type"` // violation | session | finding | exam
	ExamID int64     `json:"exam_id"`
	At     time.Time `json:"at"`
	Data   any       `json:"data"`
}

type client struct {
	examID int64
	send   chan []byte
}

type Hub struct {
	register   chan *client
	unregister chan *client
	broadcast  chan Event
	rooms      map[int64]map[*client]struct{}
	log        *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan Event, 1024),
		rooms:      make(map[int64]map[*client]struct{}),
		log:        log,
	}
}

// Run owns the hub state until ctx is cancelled, then closes every client.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for _, room := range h.rooms {
				for c := range room {
					close(c.send)
				}
			}
			h.rooms = map[int64]map[*client]struct{}{}
			return
		case c := <-h.register:
			room := h.rooms[c.examID]
			if room == nil {
				room = make(map[*client]struct{})
				h.rooms[c.examID] = room
			}
			room[c] = struct{}{}
		case c := <-h.unregister:
			h.remove(c)
		case ev := <-h.broadcast:
			msg, err := json.Marshal(ev)
			if err != nil {
				h.log.Error("hub: marshal event", "err", err)
				continue
			}
			for c := range h.rooms[ev.ExamID] {
				select {
				case c.send <- msg:
				default:
					// Slow consumer: drop it rather than stall every room.
					h.remove(c)
				}
			}
		}
	}
}

func (h *Hub) remove(c *client) {
	room := h.rooms[c.examID]
	if _, ok := room[c]; !ok {
		return
	}
	delete(room, c)
	close(c.send)
	if len(room) == 0 {
		delete(h.rooms, c.examID)
	}
}

// Publish queues an event without ever blocking the caller (an HTTP handler
// or stream consumer). If the hub is saturated the event is dropped; the
// dashboard's periodic refresh reconciles state.
func (h *Hub) Publish(examID int64, typ string, data any) {
	ev := Event{Type: typ, ExamID: examID, At: time.Now().UTC(), Data: data}
	select {
	case h.broadcast <- ev:
	default:
		h.log.Warn("hub: broadcast queue full; event dropped", "type", typ, "exam_id", examID)
	}
}
