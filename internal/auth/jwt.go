// internal/auth/jwt.go
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	AAL       string    `json:"aal"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

const (
	AccessTokenExpiry  = 3600   // 1 hour
	RefreshTokenExpiry = 604800 // 1 week
)

func (s *Service) GenerateAccessToken(user *User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"aud":        "authenticated",
		"exp":        now.Add(time.Duration(AccessTokenExpiry) * time.Second).Unix(),
		"iat":        now.Unix(),
		"iss":        "http://localhost:8080/auth/v1",
		"sub":        user.ID,
		"email":      user.Email,
		"phone":      "",
		"role":       user.Role,
		"aal":        "aal1",
		"session_id": sessionID,
		"app_metadata": map[string]any{
			"provider":  "email",
			"providers": []string{"email"},
		},
		"user_metadata": user.UserMetadata,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *Service) ValidateAccessToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &claims, nil
}

func generateRefreshToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "v1." + base64.RawURLEncoding.EncodeToString(b)
}

func (s *Service) CreateSession(user *User) (*Session, string, error) {
	sessionID := generateID()
	refreshToken := generateRefreshToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		INSERT INTO auth_sessions (id, user_id, created_at, aal)
		VALUES (?, ?, ?, 'aal1')
	`, sessionID, user.ID, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO auth_refresh_tokens (token, user_id, session_id, created_at)
		VALUES (?, ?, ?, ?)
	`, refreshToken, user.ID, sessionID, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: time.Now().UTC(),
		AAL:       "aal1",
	}

	return session, refreshToken, nil
}

func (s *Service) RefreshSession(refreshToken string) (*User, *Session, string, error) {
	var userID, sessionID string
	var revoked int

	err := s.db.QueryRow(`
		SELECT user_id, session_id, revoked FROM auth_refresh_tokens WHERE token = ?
	`, refreshToken).Scan(&userID, &sessionID, &revoked)

	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid refresh token")
	}

	if revoked == 1 {
		return nil, nil, "", fmt.Errorf("refresh token has been revoked")
	}

	// Revoke old token
	if _, err := s.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1 WHERE token = ?", refreshToken); err != nil {
		return nil, nil, "", fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, nil, "", err
	}

	// Create new session
	return s.createSessionWithExistingID(user, sessionID)
}

func (s *Service) createSessionWithExistingID(user *User, sessionID string) (*User, *Session, string, error) {
	refreshToken := generateRefreshToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		INSERT INTO auth_refresh_tokens (token, user_id, session_id, created_at)
		VALUES (?, ?, ?, ?)
	`, refreshToken, user.ID, sessionID, now)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: time.Now().UTC(),
		AAL:       "aal1",
	}

	return user, session, refreshToken, nil
}

func (s *Service) RevokeSession(sessionID string) error {
	_, err := s.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1 WHERE session_id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	_, err = s.db.Exec("DELETE FROM auth_sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// RevokeAllUserSessions revokes all sessions for a user (global logout).
func (s *Service) RevokeAllUserSessions(userID string) error {
	_, err := s.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1 WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}

	_, err = s.db.Exec("DELETE FROM auth_sessions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}

	return nil
}

// RevokeOtherSessions revokes all sessions for a user except the current one.
func (s *Service) RevokeOtherSessions(userID, currentSessionID string) error {
	_, err := s.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1 WHERE user_id = ? AND session_id != ?", userID, currentSessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke other refresh tokens: %w", err)
	}

	_, err = s.db.Exec("DELETE FROM auth_sessions WHERE user_id = ? AND id != ?", userID, currentSessionID)
	if err != nil {
		return fmt.Errorf("failed to delete other sessions: %w", err)
	}

	return nil
}
