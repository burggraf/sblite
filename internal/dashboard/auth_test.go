package dashboard

import (
	"testing"
)

func TestAuthSetupPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Setup password
	err := auth.SetupPassword("mypassword123")
	if err != nil {
		t.Fatalf("failed to setup password: %v", err)
	}

	// Should now need login, not setup
	if auth.NeedsSetup() {
		t.Error("expected NeedsSetup to return false after setup")
	}
}

func TestAuthVerifyPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Setup password
	err := auth.SetupPassword("mypassword123")
	if err != nil {
		t.Fatalf("failed to setup password: %v", err)
	}

	// Verify correct password
	if !auth.VerifyPassword("mypassword123") {
		t.Error("expected correct password to verify")
	}

	// Verify incorrect password
	if auth.VerifyPassword("wrongpassword") {
		t.Error("expected wrong password to fail verification")
	}
}

func TestAuthSetupOnlyOnce(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// First setup should succeed
	err := auth.SetupPassword("password1")
	if err != nil {
		t.Fatalf("first setup failed: %v", err)
	}

	// Second setup should fail
	err = auth.SetupPassword("password2")
	if err == nil {
		t.Error("expected second setup to fail")
	}
}

func TestAuthPasswordMinLength(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Too short password
	err := auth.SetupPassword("short")
	if err == nil {
		t.Error("expected short password to fail")
	}
}

func TestAuthResetPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Setup initial password
	err := auth.SetupPassword("password123")
	if err != nil {
		t.Fatalf("failed to setup password: %v", err)
	}

	// Reset to new password
	err = auth.ResetPassword("newpassword456")
	if err != nil {
		t.Fatalf("failed to reset password: %v", err)
	}

	// Old password should fail
	if auth.VerifyPassword("password123") {
		t.Error("expected old password to fail after reset")
	}

	// New password should work
	if !auth.VerifyPassword("newpassword456") {
		t.Error("expected new password to verify after reset")
	}
}

func TestAuthResetPasswordMinLength(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Setup initial password
	err := auth.SetupPassword("password123")
	if err != nil {
		t.Fatalf("failed to setup password: %v", err)
	}

	// Reset with too short password should fail
	err = auth.ResetPassword("short")
	if err == nil {
		t.Error("expected short password to fail on reset")
	}
}

func TestAuthNeedsSetupInitially(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Initially needs setup
	if !auth.NeedsSetup() {
		t.Error("expected NeedsSetup to return true initially")
	}
}

func TestAuthVerifyPasswordNoPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Verify should return false when no password is set
	if auth.VerifyPassword("anypassword") {
		t.Error("expected VerifyPassword to return false when no password set")
	}
}
