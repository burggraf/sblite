package oauth

import (
	"sync"
)

// Registry manages OAuth providers and their enabled states.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	enabled   map[string]bool
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		enabled:   make(map[string]bool),
	}
}

// Register adds a provider to the registry and enables it.
func (r *Registry) Register(provider Provider) {
	r.RegisterWithConfig(provider, true)
}

// RegisterWithConfig adds a provider with an explicit enabled state.
func (r *Registry) RegisterWithConfig(provider Provider, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	r.providers[name] = provider
	r.enabled[name] = enabled
}

// Get returns the provider if it exists and is enabled.
// Returns ErrProviderNotFound if the provider doesn't exist.
// Returns ErrProviderNotEnabled if the provider exists but is disabled.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	if !r.enabled[name] {
		return nil, ErrProviderNotEnabled
	}

	return provider, nil
}

// IsEnabled returns whether the provider is registered and enabled.
func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.enabled[name]
}

// SetEnabled enables or disables a provider.
// Does nothing if the provider doesn't exist.
func (r *Registry) SetEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		r.enabled[name] = enabled
	}
}

// ListEnabled returns the names of all enabled providers.
func (r *Registry) ListEnabled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, isEnabled := range r.enabled {
		if isEnabled {
			names = append(names, name)
		}
	}
	return names
}

// ListAll returns the names of all registered providers.
func (r *Registry) ListAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
