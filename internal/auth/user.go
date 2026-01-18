// internal/auth/user.go
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/markb/sblite/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                string         `json:"id"`
	Email             string         `json:"email"`
	EncryptedPassword string         `json:"-"`
	EmailConfirmedAt  *time.Time     `json:"email_confirmed_at,omitempty"`
	LastSignInAt      *time.Time     `json:"last_sign_in_at,omitempty"`
	AppMetadata       map[string]any `json:"app_metadata"`
	UserMetadata      map[string]any `json:"user_metadata"`
	Role              string         `json:"role"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type Service struct {
	db        *db.DB
	jwtSecret string
}

func NewService(database *db.DB, jwtSecret string) *Service {
	return &Service{db: database, jwtSecret: jwtSecret}
}

func generateID() string {
	return uuid.New().String()
}

func (s *Service) CreateUser(email, password string, userMetadata map[string]any) (*User, error) {
	return s.CreateUserWithOptions(email, password, userMetadata, false)
}

// CreateUserWithOptions creates a new user with optional email auto-confirmation.
func (s *Service) CreateUserWithOptions(email, password string, userMetadata map[string]any, autoConfirm bool) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	// Marshal user metadata to JSON
	userMetaJSON := "{}"
	if userMetadata != nil {
		metaBytes, err := json.Marshal(userMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal user metadata: %w", err)
		}
		userMetaJSON = string(metaBytes)
	}

	// Set email_confirmed_at if auto-confirm is enabled
	var emailConfirmedAt interface{} = nil
	if autoConfirm {
		emailConfirmedAt = now
	}

	_, err = s.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, email_confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, '{"provider":"email","providers":["email"]}', ?, ?, ?)
	`, id, email, string(hash), emailConfirmedAt, userMetaJSON, now, now)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("user with email %s already exists", email)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.GetUserByID(id)
}

func (s *Service) GetUserByID(id string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	var emailConfirmedAt, lastSignInAt sql.NullString
	var rawAppMetaData, rawUserMetaData string

	err := s.db.QueryRow(`
		SELECT id, email, encrypted_password, email_confirmed_at, last_sign_in_at,
		       role, created_at, updated_at, raw_app_meta_data, raw_user_meta_data
		FROM auth_users WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &emailConfirmedAt,
		&lastSignInAt, &user.Role, &createdAt, &updatedAt, &rawAppMetaData, &rawUserMetaData)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if emailConfirmedAt.Valid {
		t, _ := time.Parse(time.RFC3339, emailConfirmedAt.String)
		user.EmailConfirmedAt = &t
	}
	if lastSignInAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSignInAt.String)
		user.LastSignInAt = &t
	}

	// Parse app metadata from JSON
	user.AppMetadata = map[string]any{}
	if rawAppMetaData != "" {
		json.Unmarshal([]byte(rawAppMetaData), &user.AppMetadata)
	}

	// Parse user metadata from JSON
	user.UserMetadata = map[string]any{}
	if rawUserMetaData != "" {
		json.Unmarshal([]byte(rawUserMetaData), &user.UserMetadata)
	}

	return &user, nil
}

func (s *Service) GetUserByEmail(email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var id string
	err := s.db.QueryRow("SELECT id FROM auth_users WHERE email = ? AND deleted_at IS NULL", email).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return s.GetUserByID(id)
}

func (s *Service) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.EncryptedPassword), []byte(password))
	return err == nil
}

func (s *Service) UpdateLastSignIn(userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec("UPDATE auth_users SET last_sign_in_at = ? WHERE id = ?", now, userID)
	return err
}

func (s *Service) UpdateUserMetadata(userID string, metadata map[string]any) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec("UPDATE auth_users SET raw_user_meta_data = ?, updated_at = ? WHERE id = ?",
		string(metadataJSON), now, userID)
	return err
}

func (s *Service) UpdatePassword(userID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec("UPDATE auth_users SET encrypted_password = ?, updated_at = ? WHERE id = ?",
		string(hash), now, userID)
	return err
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *Service) GenerateConfirmationToken(userID string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		UPDATE auth_users SET confirmation_token = ?, confirmation_sent_at = ?
		WHERE id = ?
	`, token, now, userID)

	if err != nil {
		return "", fmt.Errorf("failed to generate confirmation token: %w", err)
	}
	return token, nil
}

func (s *Service) VerifyEmail(token string) (*User, error) {
	var userID string
	var confirmationSentAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, confirmation_sent_at FROM auth_users
		WHERE confirmation_token = ? AND deleted_at IS NULL
	`, token).Scan(&userID, &confirmationSentAt)

	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	// Check token age (24 hours expiration)
	if confirmationSentAt.Valid {
		sentAt, err := time.Parse(time.RFC3339, confirmationSentAt.String)
		if err == nil && time.Since(sentAt) > 24*time.Hour {
			return nil, fmt.Errorf("invalid or expired token")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		UPDATE auth_users SET email_confirmed_at = ?, confirmation_token = NULL WHERE id = ?
	`, now, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to verify email: %w", err)
	}

	return s.GetUserByID(userID)
}

// ConfirmEmail directly confirms a user's email by setting email_confirmed_at.
// This is useful for testing or admin operations.
func (s *Service) ConfirmEmail(userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE auth_users SET email_confirmed_at = ? WHERE id = ?
	`, now, userID)
	return err
}

func (s *Service) GenerateRecoveryToken(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var userID string
	err := s.db.QueryRow("SELECT id FROM auth_users WHERE email = ? AND deleted_at IS NULL", email).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		UPDATE auth_users SET recovery_token = ?, recovery_sent_at = ?
		WHERE id = ?
	`, token, now, userID)

	if err != nil {
		return "", fmt.Errorf("failed to generate recovery token: %w", err)
	}
	return token, nil
}

func (s *Service) ResetPassword(token, newPassword string) (*User, error) {
	var userID string
	var recoverySentAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, recovery_sent_at FROM auth_users WHERE recovery_token = ? AND deleted_at IS NULL
	`, token).Scan(&userID, &recoverySentAt)

	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	// Check token age (24 hours expiration)
	if recoverySentAt.Valid {
		sentAt, err := time.Parse(time.RFC3339, recoverySentAt.String)
		if err == nil && time.Since(sentAt) > 24*time.Hour {
			return nil, fmt.Errorf("invalid or expired token")
		}
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		UPDATE auth_users SET encrypted_password = ?, recovery_token = NULL, updated_at = ?
		WHERE id = ?
	`, string(hash), now, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to reset password: %w", err)
	}

	return s.GetUserByID(userID)
}

// CreateOAuthUser creates a new user from OAuth sign-in (no password, email confirmed).
func (s *Service) CreateOAuthUser(email, provider string, userMetadata map[string]interface{}) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	// Marshal user metadata to JSON
	userMetaJSON := "{}"
	if userMetadata != nil {
		metaBytes, err := json.Marshal(userMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal user metadata: %w", err)
		}
		userMetaJSON = string(metaBytes)
	}

	// Create app_metadata with provider info
	appMetadata := map[string]interface{}{
		"provider":  provider,
		"providers": []string{provider},
	}
	appMetaJSON, err := json.Marshal(appMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal app metadata: %w", err)
	}

	// OAuth users are created with email confirmed and no password
	_, err = s.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, email_confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, ?, '', ?, ?, ?, ?, ?)
	`, id, email, now, string(appMetaJSON), userMetaJSON, now, now)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("user with email %s already exists", email)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.GetUserByID(id)
}

// AddProviderToUser adds a provider to the user's app_metadata.providers array.
func (s *Service) AddProviderToUser(userID, provider string) error {
	// Get current app_metadata
	var rawAppMetaData string
	err := s.db.QueryRow("SELECT raw_app_meta_data FROM auth_users WHERE id = ?", userID).Scan(&rawAppMetaData)
	if err != nil {
		return fmt.Errorf("failed to get user app_metadata: %w", err)
	}

	// Parse existing app_metadata
	appMeta := map[string]interface{}{}
	if rawAppMetaData != "" {
		if err := json.Unmarshal([]byte(rawAppMetaData), &appMeta); err != nil {
			return fmt.Errorf("failed to parse app_metadata: %w", err)
		}
	}

	// Get existing providers array
	providers := []string{}
	if existingProviders, ok := appMeta["providers"].([]interface{}); ok {
		for _, p := range existingProviders {
			if pStr, ok := p.(string); ok {
				providers = append(providers, pStr)
			}
		}
	}

	// Check if provider already exists
	for _, p := range providers {
		if p == provider {
			return nil // Already linked
		}
	}

	// Add new provider
	providers = append(providers, provider)
	appMeta["providers"] = providers

	// Marshal and update
	appMetaJSON, err := json.Marshal(appMeta)
	if err != nil {
		return fmt.Errorf("failed to marshal app_metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec("UPDATE auth_users SET raw_app_meta_data = ?, updated_at = ? WHERE id = ?",
		string(appMetaJSON), now, userID)
	if err != nil {
		return fmt.Errorf("failed to update app_metadata: %w", err)
	}

	return nil
}

// RemoveProviderFromUser removes a provider from the user's app_metadata.providers array.
func (s *Service) RemoveProviderFromUser(userID, provider string) error {
	// Get current app_metadata
	var rawAppMetaData string
	err := s.db.QueryRow("SELECT raw_app_meta_data FROM auth_users WHERE id = ?", userID).Scan(&rawAppMetaData)
	if err != nil {
		return fmt.Errorf("failed to get user app_metadata: %w", err)
	}

	// Parse existing app_metadata
	appMeta := map[string]interface{}{}
	if rawAppMetaData != "" {
		if err := json.Unmarshal([]byte(rawAppMetaData), &appMeta); err != nil {
			return fmt.Errorf("failed to parse app_metadata: %w", err)
		}
	}

	// Get existing providers array
	providers := []string{}
	if existingProviders, ok := appMeta["providers"].([]interface{}); ok {
		for _, p := range existingProviders {
			if pStr, ok := p.(string); ok {
				// Skip the provider we're removing
				if pStr != provider {
					providers = append(providers, pStr)
				}
			}
		}
	}

	// Update providers array
	appMeta["providers"] = providers

	// If the removed provider was the primary provider, update it
	if primaryProvider, ok := appMeta["provider"].(string); ok && primaryProvider == provider {
		if len(providers) > 0 {
			appMeta["provider"] = providers[0]
		} else {
			delete(appMeta, "provider")
		}
	}

	// Marshal and update
	appMetaJSON, err := json.Marshal(appMeta)
	if err != nil {
		return fmt.Errorf("failed to marshal app_metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec("UPDATE auth_users SET raw_app_meta_data = ?, updated_at = ? WHERE id = ?",
		string(appMetaJSON), now, userID)
	if err != nil {
		return fmt.Errorf("failed to update app_metadata: %w", err)
	}

	return nil
}
