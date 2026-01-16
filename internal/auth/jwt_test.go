// internal/auth/jwt_test.go
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAccessToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user := &User{
		ID:    "test-user-id",
		Email: "test@example.com",
		Role:  "authenticated",
	}

	tokenString, err := service.GenerateAccessToken(user, "session-123")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Parse and verify token
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-min-32-characters"), nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to get claims")
	}

	if claims["sub"] != "test-user-id" {
		t.Errorf("expected sub=test-user-id, got %v", claims["sub"])
	}
	if claims["email"] != "test@example.com" {
		t.Errorf("expected email=test@example.com, got %v", claims["email"])
	}
	if claims["role"] != "authenticated" {
		t.Errorf("expected role=authenticated, got %v", claims["role"])
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user, _ := service.CreateUser("test@example.com", "password123", nil)

	session, refreshToken, err := service.CreateSession(user)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.ID == "" {
		t.Error("expected session ID to be set")
	}
	if refreshToken == "" {
		t.Error("expected refresh token to be set")
	}
}

func TestValidateAccessToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user := &User{
		ID:    "test-user-id",
		Email: "test@example.com",
		Role:  "authenticated",
	}

	tokenString, err := service.GenerateAccessToken(user, "session-123")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := service.ValidateAccessToken(tokenString)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if (*claims)["sub"] != "test-user-id" {
		t.Errorf("expected sub=test-user-id, got %v", (*claims)["sub"])
	}
}

func TestValidateAccessTokenInvalid(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	_, err := service.ValidateAccessToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestRefreshSession(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user, _ := service.CreateUser("test@example.com", "password123", nil)

	session, refreshToken, err := service.CreateSession(user)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Refresh the session
	newUser, newSession, newRefreshToken, err := service.RefreshSession(refreshToken)
	if err != nil {
		t.Fatalf("failed to refresh session: %v", err)
	}

	if newUser.ID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, newUser.ID)
	}
	if newSession.ID != session.ID {
		t.Errorf("expected same session ID %s, got %s", session.ID, newSession.ID)
	}
	if newRefreshToken == refreshToken {
		t.Error("expected new refresh token to be different")
	}

	// Old refresh token should be revoked
	_, _, _, err = service.RefreshSession(refreshToken)
	if err == nil {
		t.Error("expected error for revoked refresh token")
	}
}

func TestRevokeSession(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user, _ := service.CreateUser("test@example.com", "password123", nil)

	session, refreshToken, err := service.CreateSession(user)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err = service.RevokeSession(session.ID)
	if err != nil {
		t.Fatalf("failed to revoke session: %v", err)
	}

	// Refresh token should now fail
	_, _, _, err = service.RefreshSession(refreshToken)
	if err == nil {
		t.Error("expected error for revoked session")
	}
}
