// internal/auth/identity_test.go
package auth

import (
	"testing"

	"github.com/markb/sblite/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*Service, func()) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	svc := NewService(database, "test-secret-key-min-32-characters")
	cleanup := func() {
		database.Close()
	}
	return svc, cleanup
}

func TestCreateIdentity(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, err := svc.CreateUserWithOptions("test@example.com", "password123", nil, true)
	require.NoError(t, err)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123456",
		IdentityData: map[string]interface{}{
			"email":      "test@example.com",
			"name":       "Test User",
			"avatar_url": "https://example.com/avatar.jpg",
		},
	}

	err = svc.CreateIdentity(identity)
	require.NoError(t, err)
	assert.NotEmpty(t, identity.ID)
}

func TestGetIdentityByProvider(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUserWithOptions("test@example.com", "password123", nil, true)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "github",
		ProviderID: "github-789",
		IdentityData: map[string]interface{}{
			"email": "test@example.com",
		},
	}
	svc.CreateIdentity(identity)

	found, err := svc.GetIdentityByProvider("github", "github-789")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.UserID)
}

func TestGetIdentitiesByUser(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUserWithOptions("test@example.com", "password123", nil, true)

	svc.CreateIdentity(&Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})
	svc.CreateIdentity(&Identity{
		UserID:     user.ID,
		Provider:   "github",
		ProviderID: "github-456",
	})

	identities, err := svc.GetIdentitiesByUser(user.ID)
	require.NoError(t, err)
	assert.Len(t, identities, 2)
}

func TestDeleteIdentity(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUserWithOptions("test@example.com", "password123", nil, true)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	}
	svc.CreateIdentity(identity)

	err := svc.DeleteIdentity(user.ID, "google")
	require.NoError(t, err)

	_, err = svc.GetIdentityByProvider("google", "google-123")
	assert.Error(t, err)
}

func TestUpdateIdentityLastSignIn(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUserWithOptions("test@example.com", "password123", nil, true)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	}
	svc.CreateIdentity(identity)

	err := svc.UpdateIdentityLastSignIn("google", "google-123")
	require.NoError(t, err)

	found, err := svc.GetIdentityByProvider("google", "google-123")
	require.NoError(t, err)
	assert.NotNil(t, found.LastSignInAt)
}

func TestIdentityAlreadyExists(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUserWithOptions("test@example.com", "password123", nil, true)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	}
	err := svc.CreateIdentity(identity)
	require.NoError(t, err)

	// Try to create same identity again
	identity2 := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	}
	err = svc.CreateIdentity(identity2)
	assert.ErrorIs(t, err, ErrIdentityAlreadyExists)
}

func TestIdentityNotFound(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	_, err := svc.GetIdentityByProvider("google", "nonexistent")
	assert.ErrorIs(t, err, ErrIdentityNotFound)
}
