// internal/auth/user.go
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

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
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Service) CreateUser(email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, ?, ?, '{"provider":"email","providers":["email"]}', '{}', ?, ?)
	`, id, email, string(hash), now, now)

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

	err := s.db.QueryRow(`
		SELECT id, email, encrypted_password, email_confirmed_at, last_sign_in_at,
		       role, created_at, updated_at
		FROM auth_users WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &emailConfirmedAt,
		&lastSignInAt, &user.Role, &createdAt, &updatedAt)

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
	user.AppMetadata = map[string]any{"provider": "email", "providers": []string{"email"}}
	user.UserMetadata = map[string]any{}

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
