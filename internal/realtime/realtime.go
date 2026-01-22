// internal/realtime/realtime.go
package realtime

import (
	"database/sql"

	"github.com/markb/sblite/internal/rls"
)

// Service provides realtime functionality
type Service struct {
	hub        *Hub
	db         *sql.DB
	jwtSecret  string
	anonKey    string
	serviceKey string
}

// Config holds realtime configuration
type Config struct {
	JWTSecret  string
	AnonKey    string
	ServiceKey string
}

// NewService creates a new realtime service
func NewService(db *sql.DB, rlsService *rls.Service, cfg Config) *Service {
	return &Service{
		hub:        NewHub(db, rlsService, cfg.JWTSecret),
		db:         db,
		jwtSecret:  cfg.JWTSecret,
		anonKey:    cfg.AnonKey,
		serviceKey: cfg.ServiceKey,
	}
}

// Hub returns the connection hub
func (s *Service) Hub() *Hub {
	return s.hub
}

// Stats returns realtime statistics
func (s *Service) Stats() any {
	return s.hub.Stats()
}

// NotifyChange broadcasts a database change to subscribers
func (s *Service) NotifyChange(schema, table, eventType string, oldRow, newRow map[string]any) {
	s.hub.broadcastChange(schema, table, eventType, oldRow, newRow)
}
