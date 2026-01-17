package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	defaultSessionMaxAge = 24 * time.Hour
)

// SessionManager handles dashboard sessions.
type SessionManager struct {
	store  *Store
	maxAge time.Duration
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(store *Store) *SessionManager {
	return &SessionManager{
		store:  store,
		maxAge: defaultSessionMaxAge,
	}
}

// Create creates a new session and returns the token.
func (s *SessionManager) Create() (string, error) {
	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	// Store session with expiry
	expiry := time.Now().Add(s.maxAge).Format(time.RFC3339)
	if err := s.store.Set("session_token", token); err != nil {
		return "", err
	}
	if err := s.store.Set("session_expiry", expiry); err != nil {
		return "", err
	}

	return token, nil
}

// Validate checks if the token is valid and not expired.
func (s *SessionManager) Validate(token string) bool {
	storedToken, err := s.store.Get("session_token")
	if err != nil || storedToken == "" || storedToken != token {
		return false
	}

	expiryStr, err := s.store.Get("session_expiry")
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
	s.store.Set("session_token", "")
	s.store.Set("session_expiry", "")
}

// Refresh extends the session expiry if valid.
func (s *SessionManager) Refresh(token string) bool {
	if !s.Validate(token) {
		return false
	}

	expiry := time.Now().Add(s.maxAge).Format(time.RFC3339)
	s.store.Set("session_expiry", expiry)
	return true
}
