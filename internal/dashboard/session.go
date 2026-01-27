package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	defaultSessionMaxAge = 24 * time.Hour
)

// SessionManager handles dashboard sessions.
type SessionManager struct {
	store     *Store
	maxAge    time.Duration
	keyPrefix string // Port-specific prefix for session keys (e.g., "8080_")
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(store *Store) *SessionManager {
	return &SessionManager{
		store:  store,
		maxAge: defaultSessionMaxAge,
	}
}

// SetPort sets the port for session key scoping.
// This allows multiple sblite instances on different ports to share
// the same database without session conflicts.
func (s *SessionManager) SetPort(port int) {
	s.keyPrefix = fmt.Sprintf("%d_", port)
}

// sessionTokenKey returns the port-scoped session token key.
func (s *SessionManager) sessionTokenKey() string {
	return s.keyPrefix + "session_token"
}

// sessionExpiryKey returns the port-scoped session expiry key.
func (s *SessionManager) sessionExpiryKey() string {
	return s.keyPrefix + "session_expiry"
}

// Create creates a new session and returns the token.
func (s *SessionManager) Create() (string, error) {
	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	// Store session with expiry (using port-scoped keys)
	expiry := time.Now().Add(s.maxAge).Format(time.RFC3339)
	if err := s.store.Set(s.sessionTokenKey(), token); err != nil {
		return "", err
	}
	if err := s.store.Set(s.sessionExpiryKey(), expiry); err != nil {
		return "", err
	}

	return token, nil
}

// Validate checks if the token is valid and not expired.
func (s *SessionManager) Validate(token string) bool {
	storedToken, err := s.store.Get(s.sessionTokenKey())
	if err != nil || storedToken == "" || storedToken != token {
		return false
	}

	expiryStr, err := s.store.Get(s.sessionExpiryKey())
	if err != nil || expiryStr == "" {
		return false
	}

	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return false
	}

	return time.Now().Before(expiry)
}

// Destroy removes the current session.
func (s *SessionManager) Destroy() {
	s.store.Set(s.sessionTokenKey(), "")
	s.store.Set(s.sessionExpiryKey(), "")
}

// Refresh extends the session expiry if valid.
func (s *SessionManager) Refresh(token string) bool {
	if !s.Validate(token) {
		return false
	}

	expiry := time.Now().Add(s.maxAge).Format(time.RFC3339)
	s.store.Set(s.sessionExpiryKey(), expiry)
	return true
}
