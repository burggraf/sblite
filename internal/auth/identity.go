// internal/auth/identity.go
package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Error variables for identity operations
var (
	ErrIdentityNotFound      = errors.New("identity not found")
	ErrIdentityAlreadyExists = errors.New("identity already exists")
)

// Identity represents an OAuth provider account linked to a user
type Identity struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	Provider     string                 `json:"provider"`
	ProviderID   string                 `json:"provider_id"`
	IdentityData map[string]interface{} `json:"identity_data,omitempty"`
	LastSignInAt *time.Time             `json:"last_sign_in_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// CreateIdentity creates a new identity linking an OAuth provider to a user
func (s *Service) CreateIdentity(identity *Identity) error {
	if identity.ID == "" {
		identity.ID = generateID()
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	identity.CreatedAt = now
	identity.UpdatedAt = now

	// Marshal identity_data to JSON
	identityDataJSON := "{}"
	if identity.IdentityData != nil {
		dataBytes, err := json.Marshal(identity.IdentityData)
		if err != nil {
			return fmt.Errorf("failed to marshal identity data: %w", err)
		}
		identityDataJSON = string(dataBytes)
	}

	_, err := s.db.Exec(`
		INSERT INTO auth_identities (id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, identity.ID, identity.UserID, identity.Provider, identity.ProviderID, identityDataJSON, nil, nowStr, nowStr)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrIdentityAlreadyExists
		}
		return fmt.Errorf("failed to create identity: %w", err)
	}

	return nil
}

// GetIdentityByProvider retrieves an identity by provider and provider ID
func (s *Service) GetIdentityByProvider(provider, providerID string) (*Identity, error) {
	var identity Identity
	var createdAt, updatedAt string
	var lastSignInAt, identityDataJSON sql.NullString

	err := s.db.QueryRow(`
		SELECT id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at
		FROM auth_identities
		WHERE provider = ? AND provider_id = ?
	`, provider, providerID).Scan(
		&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderID,
		&identityDataJSON, &lastSignInAt, &createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrIdentityNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %w", err)
	}

	identity.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	identity.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if lastSignInAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSignInAt.String)
		identity.LastSignInAt = &t
	}

	// Parse identity_data from JSON
	identity.IdentityData = map[string]interface{}{}
	if identityDataJSON.Valid && identityDataJSON.String != "" {
		json.Unmarshal([]byte(identityDataJSON.String), &identity.IdentityData)
	}

	return &identity, nil
}

// GetIdentitiesByUser retrieves all identities for a user
func (s *Service) GetIdentitiesByUser(userID string) ([]*Identity, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at
		FROM auth_identities
		WHERE user_id = ?
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to query identities: %w", err)
	}
	defer rows.Close()

	var identities []*Identity
	for rows.Next() {
		var identity Identity
		var createdAt, updatedAt string
		var lastSignInAt, identityDataJSON sql.NullString

		err := rows.Scan(
			&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderID,
			&identityDataJSON, &lastSignInAt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan identity: %w", err)
		}

		identity.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		identity.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		if lastSignInAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastSignInAt.String)
			identity.LastSignInAt = &t
		}

		// Parse identity_data from JSON
		identity.IdentityData = map[string]interface{}{}
		if identityDataJSON.Valid && identityDataJSON.String != "" {
			json.Unmarshal([]byte(identityDataJSON.String), &identity.IdentityData)
		}

		identities = append(identities, &identity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating identities: %w", err)
	}

	return identities, nil
}

// DeleteIdentity deletes an identity by user ID and provider
func (s *Service) DeleteIdentity(userID, provider string) error {
	result, err := s.db.Exec(`
		DELETE FROM auth_identities
		WHERE user_id = ? AND provider = ?
	`, userID, provider)

	if err != nil {
		return fmt.Errorf("failed to delete identity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrIdentityNotFound
	}

	return nil
}

// UpdateIdentityLastSignIn updates the last_sign_in_at timestamp for an identity
func (s *Service) UpdateIdentityLastSignIn(provider, providerID string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.Exec(`
		UPDATE auth_identities
		SET last_sign_in_at = ?, updated_at = ?
		WHERE provider = ? AND provider_id = ?
	`, now, now, provider, providerID)

	if err != nil {
		return fmt.Errorf("failed to update identity last sign in: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrIdentityNotFound
	}

	return nil
}
