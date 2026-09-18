package middleware

import (
	"encoding/json"
	"net/http"
)

// APIResponse is the consistent envelope every endpoint returns.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{Success: status < 400, Data: data})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Deliberately generic: never leak internals (stack traces, SQL,
	// file paths) to the client.
	_ = json.NewEncoder(w).Encode(APIResponse{Success: false, Error: message})
}

func WriteError(w http.ResponseWriter, status int, message string) {
	writeError(w, status, message)
}
