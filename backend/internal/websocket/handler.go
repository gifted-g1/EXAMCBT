package websocket

import (
	"net/http"

	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"
	"github.com/gorilla/mux"

	"examshield/internal/auth"
	appmw "examshield/internal/middleware"
)

// upgrader validates Origin against the configured allowed origins to
// prevent cross-site WebSocket hijacking. It is configured in NewServeHandler.
func newUpgrader(allowedOrigins map[string]bool) gorilla.Upgrader {
	return gorilla.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return allowedOrigins[origin]
		},
	}
}

// ServeMonitoring upgrades an authenticated request to a WebSocket and
// subscribes it to the given exam's real-time event room. The token is
// passed as a query parameter (?token=) since browsers cannot set
// custom headers during the WebSocket handshake.
func ServeMonitoring(hub *Hub, tm *auth.TokenManager, allowedOrigins []string) http.HandlerFunc {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}
	upgrader := newUpgrader(originSet)

	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		claims, err := tm.ParseAccessToken(token)
		if err != nil {
			appmw.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		vars := mux.Vars(r)
		examID, err := uuid.Parse(vars["id"])
		if err != nil {
			appmw.WriteError(w, http.StatusBadRequest, "invalid exam id")
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := hub.Register(examID, claims.UserID, conn)
		hub.ReadPump(client)
	}
}
