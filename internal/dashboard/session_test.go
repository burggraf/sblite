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

func TestSessionPortIsolation(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Create two session managers with different ports (simulating two sblite instances)
	sessions8080 := NewSessionManager(store)
	sessions8080.SetPort(8080)

	sessions8081 := NewSessionManager(store)
	sessions8081.SetPort(8081)

	// Create session on port 8080
	token8080, err := sessions8080.Create()
	if err != nil {
		t.Fatalf("failed to create session for 8080: %v", err)
	}

	// Create session on port 8081
	token8081, err := sessions8081.Create()
	if err != nil {
		t.Fatalf("failed to create session for 8081: %v", err)
	}

	// Both sessions should be valid on their respective managers
	if !sessions8080.Validate(token8080) {
		t.Error("expected 8080 session to be valid")
	}
	if !sessions8081.Validate(token8081) {
		t.Error("expected 8081 session to be valid")
	}

	// Cross-validation should fail (8080 token on 8081 manager and vice versa)
	if sessions8080.Validate(token8081) {
		t.Error("expected 8081 token to be invalid on 8080 manager")
	}
	if sessions8081.Validate(token8080) {
		t.Error("expected 8080 token to be invalid on 8081 manager")
	}

	// Destroying one session should not affect the other
	sessions8080.Destroy()

	if sessions8080.Validate(token8080) {
		t.Error("expected 8080 session to be invalid after destroy")
	}
	if !sessions8081.Validate(token8081) {
		t.Error("expected 8081 session to still be valid after destroying 8080 session")
	}
}
