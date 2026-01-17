package dashboard

import (
	"testing"
	"time"
)

func TestSessionCreateAndValidate(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Create session
	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	// Validate session
	if !sessions.Validate(token) {
		t.Error("expected session to be valid")
	}
}

func TestSessionInvalidToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Invalid token should fail
	if sessions.Validate("invalid-token") {
		t.Error("expected invalid token to fail validation")
	}
}

func TestSessionDestroy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Create session
	token, _ := sessions.Create()

	// Validate should work
	if !sessions.Validate(token) {
		t.Error("expected session to be valid")
	}

	// Destroy session
	sessions.Destroy()

	// Validate should fail now
	if sessions.Validate(token) {
		t.Error("expected session to be invalid after destroy")
	}
}

func TestSessionExpiry(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)
	sessions.maxAge = 1 * time.Second // Short expiry for testing

	// Create session
	token, _ := sessions.Create()

	// Should be valid immediately
	if !sessions.Validate(token) {
		t.Error("expected session to be valid immediately")
	}

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Should be invalid now
	if sessions.Validate(token) {
		t.Error("expected session to expire")
	}
}

func TestSessionRefresh(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)
	sessions.maxAge = 5 * time.Second // Short expiry for testing

	// Create session
	token, _ := sessions.Create()

	// Wait 1 second
	time.Sleep(1 * time.Second)

	// Refresh should extend expiry (now expires 5s from refresh time)
	if !sessions.Refresh(token) {
		t.Error("expected refresh to succeed")
	}

	// Wait another 3 seconds (would have expired at original 5s without refresh)
	time.Sleep(3 * time.Second)

	// Should still be valid because we refreshed (we're at ~4s total, expires at ~6s)
	if !sessions.Validate(token) {
		t.Error("expected session to be valid after refresh")
	}
}

func TestSessionRefreshInvalidToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Refresh invalid token should fail
	if sessions.Refresh("invalid-token") {
		t.Error("expected refresh of invalid token to fail")
	}
}

func TestSessionTokenFormat(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Create session
	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Token should be 64 hex characters (32 bytes)
	if len(token) != 64 {
		t.Errorf("expected token length 64, got %d", len(token))
	}

	// All characters should be valid hex
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected hex character, got %c", c)
		}
	}
}
