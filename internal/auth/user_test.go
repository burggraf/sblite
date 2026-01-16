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
