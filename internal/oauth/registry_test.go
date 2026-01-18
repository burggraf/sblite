package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryGetProvider(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{
		ClientID:     "google-id",
		ClientSecret: "google-secret",
		Enabled:      true,
	}))

	provider, err := registry.Get("google")
	require.NoError(t, err)
	assert.Equal(t, "google", provider.Name())
}

func TestRegistryProviderNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestRegistryListEnabled(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{Enabled: true}))
	registry.Register(NewGitHubProvider(Config{Enabled: true}))

	providers := registry.ListEnabled()
	assert.Len(t, providers, 2)
}

func TestRegistryRegisterWithConfigDisabled(t *testing.T) {
	registry := NewRegistry()

	registry.RegisterWithConfig(NewGoogleProvider(Config{
		ClientID:     "google-id",
		ClientSecret: "google-secret",
	}), false)

	// Provider should not be returned by Get when disabled
	_, err := registry.Get("google")
	assert.ErrorIs(t, err, ErrProviderNotEnabled)

	// But should still exist in ListAll
	all := registry.ListAll()
	assert.Contains(t, all, "google")

	// And not in ListEnabled
	enabled := registry.ListEnabled()
	assert.NotContains(t, enabled, "google")
}

func TestRegistryIsEnabled(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{Enabled: true}))
	registry.RegisterWithConfig(NewGitHubProvider(Config{}), false)

	assert.True(t, registry.IsEnabled("google"))
	assert.False(t, registry.IsEnabled("github"))
	assert.False(t, registry.IsEnabled("nonexistent"))
}

func TestRegistrySetEnabled(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{Enabled: true}))

	// Initially enabled
	assert.True(t, registry.IsEnabled("google"))
	_, err := registry.Get("google")
	require.NoError(t, err)

	// Disable it
	registry.SetEnabled("google", false)
	assert.False(t, registry.IsEnabled("google"))
	_, err = registry.Get("google")
	assert.ErrorIs(t, err, ErrProviderNotEnabled)

	// Re-enable it
	registry.SetEnabled("google", true)
	assert.True(t, registry.IsEnabled("google"))
	_, err = registry.Get("google")
	require.NoError(t, err)
}

func TestRegistryListAll(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{Enabled: true}))
	registry.RegisterWithConfig(NewGitHubProvider(Config{}), false)

	all := registry.ListAll()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "google")
	assert.Contains(t, all, "github")
}

func TestRegistryConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewGoogleProvider(Config{Enabled: true}))

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				registry.Get("google")
				registry.IsEnabled("google")
				registry.ListEnabled()
				registry.ListAll()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				registry.SetEnabled("google", j%2 == 0)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
}
