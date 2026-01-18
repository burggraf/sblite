package oauth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrStateNotFound is returned when OAuth state is not found or expired.
var ErrStateNotFound = errors.New("oauth state not found or expired")

// FlowState represents the OAuth flow state stored during PKCE flow.
type FlowState struct {
	ID            string
	Provider      string
	CodeVerifier  string
	RedirectTo    string
	LinkingUserID string // User ID if linking an anonymous user to OAuth
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// StateStore manages OAuth flow state in the database.
type StateStore struct {
	db *sql.DB
}

// NewStateStore creates a new state store.
func NewStateStore(db *sql.DB) *StateStore {
	return &StateStore{db: db}
}

// Save stores a new flow state with 10-minute expiry.
func (s *StateStore) Save(state *FlowState) error {
	now := time.Now().UTC()
	expiresAt := now.Add(10 * time.Minute)

	// Handle empty LinkingUserID as NULL
	var linkingUserID interface{} = nil
	if state.LinkingUserID != "" {
		linkingUserID = state.LinkingUserID
	}

	_, err := s.db.Exec(`
		INSERT INTO auth_flow_state (id, provider, code_verifier, redirect_to, linking_user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		state.ID, state.Provider, state.CodeVerifier, state.RedirectTo, linkingUserID,
		now.Format(time.RFC3339), expiresAt.Format(time.RFC3339))
	return err
}

// Get retrieves and validates a flow state. Returns error if expired or not found.
func (s *StateStore) Get(id string) (*FlowState, error) {
	var state FlowState
	var createdAt, expiresAt string
	var linkingUserID sql.NullString

	err := s.db.QueryRow(`
		SELECT id, provider, code_verifier, redirect_to, linking_user_id, created_at, expires_at
		FROM auth_flow_state
		WHERE id = ? AND expires_at > datetime('now')`,
		id).Scan(&state.ID, &state.Provider, &state.CodeVerifier, &state.RedirectTo, &linkingUserID, &createdAt, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, ErrStateNotFound
	}
	if err != nil {
		return nil, err
	}

	if linkingUserID.Valid {
		state.LinkingUserID = linkingUserID.String
	}

	state.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	state.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}

	return &state, nil
}

// Delete removes a flow state (called after successful exchange).
func (s *StateStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM auth_flow_state WHERE id = ?", id)
	return err
}

// CleanupExpired removes all expired flow states.
func (s *StateStore) CleanupExpired() error {
	_, err := s.db.Exec("DELETE FROM auth_flow_state WHERE expires_at <= datetime('now')")
	return err
}
