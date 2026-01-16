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

	_, err = s.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, ?, ?, '{"provider":"email","providers":["email"]}', ?, ?, ?)
	`, id, email, string(hash), userMetaJSON, now, now)

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

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *Service) GenerateConfirmationToken(userID string) (string, error) {
	token := generateToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
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
	err := s.db.QueryRow(`
		SELECT id FROM auth_users WHERE confirmation_token = ? AND deleted_at IS NULL
	`, token).Scan(&userID)

	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
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
