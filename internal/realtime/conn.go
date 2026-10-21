package realtime

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Same-origin only: the dashboard is served by this server.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client; still needs a valid ticket
		}
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	},
}

// Serve upgrades the request and streams events for examID until the client
// disconnects or ctx (server shutdown) ends. Authorisation happens before
// this is called.
func (h *Hub) Serve(ctx context.Context, w http.ResponseWriter, r *http.Request, examID int64) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote an HTTP error.
	}
	c := &client{examID: examID, send: make(chan []byte, 256)}
	select {
	case h.register <- c:
	case <-ctx.Done():
		ws.Close()
		return
	}

	done := make(chan struct{})
	go h.readPump(ws, c, done)
	h.writePump(ctx, ws, c, done)
}

// readPump only services control frames (pongs/close); dashboards never send
// data. It signals done when the connection dies.
func (h *Hub) readPump(ws *websocket.Conn, c *client, done chan<- struct{}) {
	defer close(done)
	ws.SetReadLimit(maxMsgSize)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) writePump(ctx context.Context, ws *websocket.Conn, c *client, done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		// Unregister unless the hub already dropped us (send closed). The
		// select prevents blocking forever if the hub has stopped.
		select {
		case h.unregister <- c:
		case <-ctx.Done():
		}
		ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}
