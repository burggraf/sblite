// internal/realtime/handler.go
package realtime

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/markb/sblite/internal/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (CORS handled elsewhere)
	},
}

// HandleWebSocket handles WebSocket upgrade requests
func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Validate API key
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		apiKey = r.Header.Get("apikey")
	}

	if !s.validateAPIKey(apiKey) {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("realtime: upgrade failed", "error", err.Error())
		return
	}

	// Create connection and start pumps
	conn := s.hub.NewConn(ws)
	log.Debug("realtime: new connection", "conn_id", conn.ID())

	go conn.WritePump()
	go conn.ReadPump()
}

// validateAPIKey checks if the API key is valid
func (s *Service) validateAPIKey(key string) bool {
	return key == s.anonKey || key == s.serviceKey
}
