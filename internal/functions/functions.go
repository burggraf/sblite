// Package functions provides edge function execution via Supabase Edge Runtime.
// It manages the runtime lifecycle, function discovery, and HTTP request proxying.
package functions

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/markb/sblite/internal/log"
)

// Service manages edge functions, including runtime lifecycle and function discovery.
type Service struct {
	db           *sql.DB
	store        *Store
	runtime      *RuntimeManager
	functionsDir string
	jwtSecret    string
	baseURL      string
	sblitePort   int

	mu      sync.RWMutex
	started bool
}

// Config holds configuration for the functions service.
type Config struct {
	// FunctionsDir is the directory containing edge functions (default: ./functions)
	FunctionsDir string
	// RuntimePort is the internal port for edge-runtime (default: 8081)
	RuntimePort int
	// JWTSecret is used for validating function invocation requests
	JWTSecret string
	// BaseURL is the sblite server URL (e.g., http://localhost:8080)
	BaseURL string
	// SblitePort is the sblite server port for env var injection
	SblitePort int
	// AnonKey is the anon API key for env var injection
	AnonKey string
	// ServiceKey is the service_role API key for env var injection
	ServiceKey string
	// DBPath is the database path for env var injection
	DBPath string
}

// DefaultConfig returns default configuration for the functions service.
func DefaultConfig() *Config {
	return &Config{
		FunctionsDir: "./functions",
		RuntimePort:  8081,
	}
}

// NewService creates a new functions service.
func NewService(db *sql.DB, cfg *Config) (*Service, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Ensure functions directory exists
	if err := os.MkdirAll(cfg.FunctionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create functions directory: %w", err)
	}

	// Create store for secrets and configuration
	var store *Store
	if db != nil && cfg.JWTSecret != "" {
		store = NewStore(db, cfg.JWTSecret)
	}

	// Create runtime manager
	runtime := NewRuntimeManager(RuntimeConfig{
		FunctionsDir: cfg.FunctionsDir,
		Port:         cfg.RuntimePort,
		SblitePort:   cfg.SblitePort,
		AnonKey:      cfg.AnonKey,
		ServiceKey:   cfg.ServiceKey,
		DBPath:       cfg.DBPath,
	})

	return &Service{
		db:           db,
		store:        store,
		runtime:      runtime,
		functionsDir: cfg.FunctionsDir,
		jwtSecret:    cfg.JWTSecret,
		baseURL:      cfg.BaseURL,
		sblitePort:   cfg.SblitePort,
	}, nil
}

// Start starts the edge runtime process.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	log.Info("starting edge functions service", "functions_dir", s.functionsDir)

	// Load secrets from store and inject into runtime
	if s.store != nil {
		secrets, err := s.store.GetAllSecrets()
		if err != nil {
			log.Warn("failed to load secrets from store", "error", err)
		} else if len(secrets) > 0 {
			s.runtime.UpdateSecrets(secrets)
			log.Info("injected secrets into edge runtime", "count", len(secrets))
		}
	}

	if err := s.runtime.Start(ctx); err != nil {
		return fmt.Errorf("failed to start edge runtime: %w", err)
	}

	s.started = true
	return nil
}

// Stop stops the edge runtime process.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	log.Info("stopping edge functions service")

	if err := s.runtime.Stop(); err != nil {
		return fmt.Errorf("failed to stop edge runtime: %w", err)
	}

	s.started = false
	return nil
}

// IsRunning returns true if the edge runtime is running.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started && s.runtime.IsHealthy()
}

// RuntimePort returns the port the edge runtime is listening on.
func (s *Service) RuntimePort() int {
	return s.runtime.Port()
}

// JWTSecret returns the JWT secret for function invocation validation.
func (s *Service) JWTSecret() string {
	return s.jwtSecret
}

// ListFunctions returns a list of discovered functions.
func (s *Service) ListFunctions() ([]FunctionInfo, error) {
	var functions []FunctionInfo

	entries, err := os.ReadDir(s.functionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return functions, nil
		}
		return nil, fmt.Errorf("failed to read functions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip hidden directories and _shared
		if name[0] == '.' || name == "_shared" {
			continue
		}

		// Check for index.ts or index.js
		indexTS := filepath.Join(s.functionsDir, name, "index.ts")
		indexJS := filepath.Join(s.functionsDir, name, "index.js")

		var entrypoint string
		if _, err := os.Stat(indexTS); err == nil {
			entrypoint = "index.ts"
		} else if _, err := os.Stat(indexJS); err == nil {
			entrypoint = "index.js"
		} else {
			continue // No valid entrypoint
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		functions = append(functions, FunctionInfo{
			Name:       name,
			Entrypoint: entrypoint,
			Path:       filepath.Join(s.functionsDir, name),
			ModTime:    info.ModTime(),
		})
	}

	return functions, nil
}

// FunctionExists returns true if a function with the given name exists.
func (s *Service) FunctionExists(name string) bool {
	path := filepath.Join(s.functionsDir, name)
	indexTS := filepath.Join(path, "index.ts")
	indexJS := filepath.Join(path, "index.js")

	if _, err := os.Stat(indexTS); err == nil {
		return true
	}
	if _, err := os.Stat(indexJS); err == nil {
		return true
	}
	return false
}

// GetFunction returns information about a specific function.
func (s *Service) GetFunction(name string) (*FunctionInfo, error) {
	path := filepath.Join(s.functionsDir, name)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("function %q not found", name)
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("function %q is not a directory", name)
	}

	// Check for entrypoint
	indexTS := filepath.Join(path, "index.ts")
	indexJS := filepath.Join(path, "index.js")

	var entrypoint string
	if _, err := os.Stat(indexTS); err == nil {
		entrypoint = "index.ts"
	} else if _, err := os.Stat(indexJS); err == nil {
		entrypoint = "index.js"
	} else {
		return nil, fmt.Errorf("function %q has no index.ts or index.js", name)
	}

	return &FunctionInfo{
		Name:       name,
		Entrypoint: entrypoint,
		Path:       path,
		ModTime:    info.ModTime(),
	}, nil
}

// CreateFunction creates a new function from a template.
func (s *Service) CreateFunction(name string, templateType string) error {
	path := filepath.Join(s.functionsDir, name)

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("function %q already exists", name)
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create function directory: %w", err)
	}

	// Write template based on type
	indexPath := filepath.Join(path, "index.ts")
	templateContent := GetTemplate(TemplateType(templateType), name)
	if err := os.WriteFile(indexPath, []byte(templateContent), 0644); err != nil {
		os.RemoveAll(path)
		return fmt.Errorf("failed to write function template: %w", err)
	}

	log.Info("created function", "name", name, "path", path, "template", templateType)
	return nil
}

// DeleteFunction deletes a function and its directory.
func (s *Service) DeleteFunction(name string) error {
	path := filepath.Join(s.functionsDir, name)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("function %q not found", name)
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}

	log.Info("deleted function", "name", name)
	return nil
}

// FunctionsDir returns the functions directory path.
func (s *Service) FunctionsDir() string {
	return s.functionsDir
}

// Store returns the functions store (for secrets/config management).
func (s *Service) Store() *Store {
	return s.store
}

// SetSecret sets a secret value (requires restart to take effect).
func (s *Service) SetSecret(name, value string) error {
	if s.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return s.store.SetSecret(name, value)
}

// DeleteSecret deletes a secret (requires restart to take effect).
func (s *Service) DeleteSecret(name string) error {
	if s.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return s.store.DeleteSecret(name)
}

// ListSecrets returns all secret names (values are never exposed).
func (s *Service) ListSecrets() ([]Secret, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return s.store.ListSecrets()
}

// GetMetadata returns metadata for a function.
func (s *Service) GetMetadata(name string) (*FunctionMetadata, error) {
	if s.store == nil {
		return &FunctionMetadata{Name: name, VerifyJWT: true, EnvVars: make(map[string]string)}, nil
	}
	return s.store.GetMetadata(name)
}

// SetMetadata saves metadata for a function.
func (s *Service) SetMetadata(meta *FunctionMetadata) error {
	if s.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return s.store.SetMetadata(meta)
}

// Restart restarts the edge runtime (stops and starts).
func (s *Service) Restart(ctx context.Context) error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start(ctx)
}

// ReloadSecrets reloads secrets from the store and updates the runtime.
// Call this after modifying secrets, then restart the runtime for changes to take effect.
func (s *Service) ReloadSecrets() error {
	if s.store == nil {
		return nil
	}

	secrets, err := s.store.GetAllSecrets()
	if err != nil {
		return fmt.Errorf("failed to load secrets: %w", err)
	}

	s.runtime.UpdateSecrets(secrets)
	return nil
}
