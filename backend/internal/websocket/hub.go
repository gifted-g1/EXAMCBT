package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"
)

// EventType enumerates every real-time event the platform pushes to
// connected dashboards/clients.
type EventType string

const (
	EventStudentJoined       EventType = "STUDENT_JOINED"
	EventStudentStarted      EventType = "STUDENT_STARTED"
	EventStudentVerified     EventType = "STUDENT_VERIFIED"
	EventStudentSubmitted    EventType = "STUDENT_SUBMITTED"
	EventStudentDisconnected EventType = "STUDENT_DISCONNECTED"
	EventFaceVerification    EventType = "FACE_VERIFICATION"
	EventAIMonitoring        EventType = "AI_MONITORING_EVENT"
	EventSuspiciousActivity  EventType = "SUSPICIOUS_ACTIVITY"
	EventCameraStatus        EventType = "CAMERA_STATUS"
	EventExamStatus          EventType = "EXAM_STATUS"
	EventTimeRemaining       EventType = "TIME_REMAINING"
)

// Message is the envelope broadcast to subscribers of an exam's room.
type Message struct {
	Type      EventType   `json:"type"`
	ExamID    uuid.UUID   `json:"exam_id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client represents a single authenticated WebSocket connection,
// subscribed to exactly one exam "room" (dashboard) at a time.
type Client struct {
	conn   *gorilla.Conn
	send   chan Message
	examID uuid.UUID
	userID uuid.UUID
}

// Hub keeps track of all connected clients grouped by exam room and
// fans out broadcasts. One Hub instance runs per server process.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[uuid.UUID]map[*Client]bool
	logger  *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		rooms:  make(map[uuid.UUID]map[*Client]bool),
		logger: logger,
	}
}

func (h *Hub) Register(examID, userID uuid.UUID, conn *gorilla.Conn) *Client {
	c := &Client{conn: conn, send: make(chan Message, 32), examID: examID, userID: userID}
	h.mu.Lock()
	if h.rooms[examID] == nil {
		h.rooms[examID] = make(map[*Client]bool)
	}
	h.rooms[examID][c] = true
	h.mu.Unlock()

	go h.writePump(c)
	return c
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if clients, ok := h.rooms[c.examID]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.rooms, c.examID)
		}
	}
	h.mu.Unlock()
	close(c.send)
	_ = c.conn.Close()
}

// Broadcast pushes an event to every client subscribed to the given
// exam room (i.e. every admin/lecturer dashboard currently monitoring
// that exam).
func (h *Hub) Broadcast(examID uuid.UUID, eventType EventType, payload interface{}) {
	msg := Message{Type: eventType, ExamID: examID, Payload: payload, Timestamp: time.Now()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[examID] {
		select {
		case c.send <- msg:
		default:
			h.logger.Warn("dropping websocket message: client send buffer full", "user_id", c.userID)
		}
	}
}

func (h *Hub) writePump(c *Client) {
	for msg := range c.send {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if err := c.conn.WriteMessage(gorilla.TextMessage, data); err != nil {
			h.Unregister(c)
			return
		}
	}
}

// ReadPump drains inbound frames (mostly pings/pongs; the dashboard is
// push-only) and unregisters the client on disconnect.
func (h *Hub) ReadPump(c *Client) {
	defer h.Unregister(c)
	c.conn.SetReadLimit(4096)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
