// internal/auth/tokens.go
package auth

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TokenTypeConfirmation = "confirmation"
	TokenTypeRecovery     = "recovery"
	TokenTypeMagicLink    = "magiclink"
	TokenTypeEmailChange  = "email_change"
	TokenTypeInvite       = "invite"
)

// VerificationToken represents a token stored in auth_verification_tokens.
type VerificationToken struct {
	ID        string
	UserID    string
	Type      string
	Email     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreateVerificationToken creates a new verification token.
func (s *Service) CreateVerificationToken(userID, tokenType, email string, expiresIn time.Duration) (string, error) {
	token := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(expiresIn)

	_, err := s.db.Exec(`
		INSERT INTO auth_verification_tokens (id, user_id, type, email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token, userID, tokenType, email, expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))

	if err != nil {
		return "", fmt.Errorf("failed to create verification token: %w", err)
	}

	return token, nil
}

// ValidateVerificationToken checks if a token is valid (exists, not expired, not used).
func (s *Service) ValidateVerificationToken(token, expectedType string) (*VerificationToken, error) {
	var vt VerificationToken
	var expiresAt, createdAt string
	var usedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, user_id, type, email, expires_at, used_at, created_at
		FROM auth_verification_tokens WHERE id = ?
	`, token).Scan(&vt.ID, &vt.UserID, &vt.Type, &vt.Email, &expiresAt, &usedAt, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	vt.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expires_at: %w", err)
	}
	vt.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if usedAt.Valid {
		t, err := time.Parse(time.RFC3339, usedAt.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse used_at: %w", err)
		}
		vt.UsedAt = &t
	}

	// Check type
	if vt.Type != expectedType {
		return nil, fmt.Errorf("invalid token type")
	}

	// Check if already used
	if vt.UsedAt != nil {
		return nil, fmt.Errorf("token already used")
	}

	// Check expiration
	if time.Now().After(vt.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	return &vt, nil
}

// MarkTokenUsed marks a verification token as used.
func (s *Service) MarkTokenUsed(token string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec("UPDATE auth_verification_tokens SET used_at = ? WHERE id = ?", now, token)
	if err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("token not found")
	}
	return nil
}

// GenerateMagicLinkToken creates a magic link token for the given email.
func (s *Service) GenerateMagicLinkToken(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.GetUserByEmail(email)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}

	return s.CreateVerificationToken(user.ID, TokenTypeMagicLink, email, 1*time.Hour)
}

// GenerateInviteToken creates an invite token for a new user.
func (s *Service) GenerateInviteToken(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Check if user already exists
	_, err := s.GetUserByEmail(email)
	if err == nil {
		return "", fmt.Errorf("user already exists")
	}

	// Create a placeholder user ID (will be created when invite is accepted)
	placeholderID := uuid.New().String()

	return s.CreateVerificationToken(placeholderID, TokenTypeInvite, email, 7*24*time.Hour)
}

// GenerateConfirmationTokenNew creates a confirmation token for a user using the verification tokens table.
// This is an alternative to GenerateConfirmationToken which stores tokens in the auth_users table.
func (s *Service) GenerateConfirmationTokenNew(userID string) (string, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}
	return s.CreateVerificationToken(userID, TokenTypeConfirmation, user.Email, 24*time.Hour)
}

// VerifyMagicLink verifies a magic link token and returns a session.
func (s *Service) VerifyMagicLink(token string) (*User, *Session, string, error) {
	vt, err := s.ValidateVerificationToken(token, TokenTypeMagicLink)
	if err != nil {
		return nil, nil, "", err
	}

	user, err := s.GetUserByID(vt.UserID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("user not found")
	}

	// Mark token as used
	if err := s.MarkTokenUsed(token); err != nil {
		return nil, nil, "", fmt.Errorf("failed to mark token as used: %w", err)
	}

	// Confirm email if not already confirmed
	if user.EmailConfirmedAt == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := s.db.Exec("UPDATE auth_users SET email_confirmed_at = ? WHERE id = ?", now, user.ID); err != nil {
			return nil, nil, "", fmt.Errorf("failed to confirm email: %w", err)
		}
	}

	// Create session
	session, refreshToken, err := s.CreateSession(user)
	if err != nil {
		return nil, nil, "", err
	}

	return user, session, refreshToken, nil
}

// AcceptInvite accepts an invitation and creates a new user.
func (s *Service) AcceptInvite(token, password string) (*User, error) {
	vt, err := s.ValidateVerificationToken(token, TokenTypeInvite)
	if err != nil {
		return nil, err
	}

	// Create the user
	user, err := s.CreateUser(vt.Email, password, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Confirm email immediately since they came from invite
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec("UPDATE auth_users SET email_confirmed_at = ?, invited_at = ? WHERE id = ?", now, now, user.ID); err != nil {
		return nil, fmt.Errorf("failed to confirm invited user email: %w", err)
	}

	// Mark token as used
	if err := s.MarkTokenUsed(token); err != nil {
		return nil, fmt.Errorf("failed to mark invite token as used: %w", err)
	}

	return s.GetUserByID(user.ID)
}
