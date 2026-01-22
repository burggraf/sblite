// internal/realtime/handler.go
package realtime

import (
	"net/http"

	"github.com/golang-jwt/jwt/v5"
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
	log.Debug("realtime: websocket request received")

	// Validate API key
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		apiKey = r.Header.Get("apikey")
	}

	if !s.validateAPIKey(apiKey) {
		log.Debug("realtime: invalid API key")
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	log.Debug("realtime: API key validated, upgrading connection")

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("realtime: upgrade failed", "error", err.Error())
		return
	}

	// Create connection and start pumps
	conn := s.hub.NewConn(ws)
	log.Debug("realtime: new connection", "conn_id", conn.ID())

	go conn.WritePump()
	go conn.ReadPump()
}

// validateAPIKey checks if the API key is valid
// Accepts either stored API keys or JWT-signed API keys
func (s *Service) validateAPIKey(key string) bool {
	// Check against stored API keys
	if key == s.anonKey || key == s.serviceKey {
		return true
	}

	// Try to validate as a JWT-based API key
	token, err := jwt.Parse(key, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return false
	}

	// Check if the role is anon or service_role
	if role, ok := claims["role"].(string); ok {
		return role == "anon" || role == "service_role"
	}

	return false
}
