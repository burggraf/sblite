// internal/auth/user_test.go
package auth

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database
}

func TestCreateUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user, err := service.CreateUser("test@example.com", "password123", nil)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	_, err := service.CreateUser("test@example.com", "password123", nil)
	if err != nil {
		t.Fatalf("failed to create first user: %v", err)
	}

	_, err = service.CreateUser("test@example.com", "password456", nil)
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestCreateOAuthUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	userMeta := map[string]interface{}{
		"name":       "OAuth User",
		"avatar_url": "https://example.com/avatar.jpg",
	}

	user, err := service.CreateOAuthUser("oauth@example.com", "google", userMeta)
	if err != nil {
		t.Fatalf("failed to create OAuth user: %v", err)
	}

	// Verify user properties
	if user.Email != "oauth@example.com" {
		t.Errorf("expected email oauth@example.com, got %s", user.Email)
	}
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
	// OAuth users should have email confirmed
	if user.EmailConfirmedAt == nil {
		t.Error("expected email to be confirmed for OAuth user")
	}
	// Check app_metadata has provider
	provider, ok := user.AppMetadata["provider"]
	if !ok || provider != "google" {
		t.Errorf("expected app_metadata.provider to be 'google', got %v", provider)
	}
	// Check providers array
	providers, ok := user.AppMetadata["providers"]
	if !ok {
		t.Error("expected app_metadata.providers to exist")
	}
	providersArr, ok := providers.([]interface{})
	if !ok || len(providersArr) != 1 || providersArr[0] != "google" {
		t.Errorf("expected providers to be ['google'], got %v", providers)
	}
	// Check user_metadata
	name, ok := user.UserMetadata["name"]
	if !ok || name != "OAuth User" {
		t.Errorf("expected user_metadata.name to be 'OAuth User', got %v", name)
	}
}

func TestCreateOAuthUserDuplicateEmail(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	// Create first user
	_, err := service.CreateOAuthUser("test@example.com", "google", nil)
	if err != nil {
		t.Fatalf("failed to create first OAuth user: %v", err)
	}

	// Try to create another user with same email
	_, err = service.CreateOAuthUser("test@example.com", "github", nil)
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestAddProviderToUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	// Create a user with email provider
	user, err := service.CreateUser("test@example.com", "password123", nil)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Add google provider
	err = service.AddProviderToUser(user.ID, "google")
	if err != nil {
		t.Fatalf("failed to add provider: %v", err)
	}

	// Verify provider was added
	updatedUser, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	providers, ok := updatedUser.AppMetadata["providers"].([]interface{})
	if !ok {
		t.Fatal("expected providers to be an array")
	}

	// Should have both email and google
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}

	hasEmail := false
	hasGoogle := false
	for _, p := range providers {
		if p == "email" {
			hasEmail = true
		}
		if p == "google" {
			hasGoogle = true
		}
	}
	if !hasEmail || !hasGoogle {
		t.Errorf("expected providers to contain 'email' and 'google', got %v", providers)
	}
}

func TestAddProviderToUserDuplicate(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	// Create user with email
	user, err := service.CreateUser("test@example.com", "password123", nil)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Try to add email provider again (should be no-op)
	err = service.AddProviderToUser(user.ID, "email")
	if err != nil {
		t.Fatalf("expected no error for duplicate provider: %v", err)
	}

	// Verify still only one email entry
	updatedUser, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	providers, ok := updatedUser.AppMetadata["providers"].([]interface{})
	if !ok {
		t.Fatal("expected providers to be an array")
	}

	emailCount := 0
	for _, p := range providers {
		if p == "email" {
			emailCount++
		}
	}
	if emailCount != 1 {
		t.Errorf("expected exactly 1 email provider, found %d", emailCount)
	}
}

func TestCreateAnonymousUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	userMeta := map[string]any{"theme": "dark"}
	user, err := service.CreateAnonymousUser(userMeta)
	if err != nil {
		t.Fatalf("failed to create anonymous user: %v", err)
	}

	// Verify user properties
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
	if user.Email != "" {
		t.Errorf("expected email to be empty, got %s", user.Email)
	}
	if !user.IsAnonymous {
		t.Error("expected IsAnonymous to be true")
	}
	if user.Role != "authenticated" {
		t.Errorf("expected role to be 'authenticated', got %s", user.Role)
	}

	// Check app_metadata
	provider, ok := user.AppMetadata["provider"]
	if !ok || provider != "anonymous" {
		t.Errorf("expected app_metadata.provider to be 'anonymous', got %v", provider)
	}

	// Check user_metadata
	theme, ok := user.UserMetadata["theme"]
	if !ok || theme != "dark" {
		t.Errorf("expected user_metadata.theme to be 'dark', got %v", theme)
	}
}
