// Package migration provides tools for migrating sblite databases to Supabase.
package migration

import (
	"database/sql"
	"fmt"
)

// Service orchestrates migrations to Supabase.
type Service struct {
	db           *sql.DB
	state        *StateStore
	supabase     *SupabaseClient
	serverConfig *ServerConfig
}

// ServerConfig holds server configuration needed for migrations.
type ServerConfig struct {
	FunctionsDir string
	StorageDir   string
}

// NewService creates a new migration service.
func NewService(db *sql.DB, config *ServerConfig) *Service {
	return &Service{
		db:           db,
		state:        NewStateStore(db),
		serverConfig: config,
	}
}

// StartMigration creates a new migration session.
func (s *Service) StartMigration() (*Migration, error) {
	return s.state.CreateMigration()
}

// GetMigration retrieves a migration by ID.
func (s *Service) GetMigration(id string) (*Migration, error) {
	m, err := s.state.GetMigration(id)
	if err != nil {
		return nil, fmt.Errorf("get migration: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("migration not found: %s", id)
	}
	return m, nil
}

// ListMigrations returns all migrations.
func (s *Service) ListMigrations() ([]*Migration, error) {
	return s.state.ListMigrations()
}

// DeleteMigration deletes a migration and all its items.
func (s *Service) DeleteMigration(id string) error {
	// Delete items first (foreign key constraint)
	_, err := s.db.Exec(`DELETE FROM _migration_items WHERE migration_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete migration items: %w", err)
	}

	// Delete verifications
	_, err = s.db.Exec(`DELETE FROM _migration_verifications WHERE migration_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete migration verifications: %w", err)
	}

	return s.state.DeleteMigration(id)
}

// ConnectSupabase validates and stores the Supabase access token.
func (s *Service) ConnectSupabase(migrationID, token string) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Create client and validate token
	client := NewSupabaseClient(token)
	if err := client.ValidateToken(); err != nil {
		return fmt.Errorf("validate token: %w", err)
	}

	// Store the token
	// TODO: Encrypt credentials before storing. Currently storing plaintext.
	m.CredentialsEncrypted = []byte(token)

	if err := s.state.UpdateMigration(m); err != nil {
		return fmt.Errorf("update migration: %w", err)
	}

	// Keep client for subsequent operations
	s.supabase = client

	return nil
}

// ListSupabaseProjects returns available projects for the connected account.
func (s *Service) ListSupabaseProjects(migrationID string) ([]Project, error) {
	client, err := s.getSupabaseClient(migrationID)
	if err != nil {
		return nil, err
	}

	return client.ListProjects()
}

// SelectProject sets the target Supabase project for the migration.
func (s *Service) SelectProject(migrationID, projectRef string) error {
	client, err := s.getSupabaseClient(migrationID)
	if err != nil {
		return err
	}

	// Verify project exists and is accessible
	project, err := client.GetProject(projectRef)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	m.SupabaseProjectRef = project.ID
	m.SupabaseProjectName = project.Name

	return s.state.UpdateMigration(m)
}

// SelectItemsRequest specifies which items to include in the migration.
type SelectItemsRequest struct {
	// Boolean flags for single-instance items
	Schema         bool `json:"schema"`
	Users          bool `json:"users"`
	Identities     bool `json:"identities"`
	RLS            bool `json:"rls"`
	StorageBuckets bool `json:"storage_buckets"`
	Secrets        bool `json:"secrets"`
	AuthConfig     bool `json:"auth_config"`
	OAuthConfig    bool `json:"oauth_config"`
	EmailTemplates bool `json:"email_templates"`

	// Named items
	Data         []string `json:"data"`          // Table names for data migration
	StorageFiles []string `json:"storage_files"` // Bucket IDs for file migration
	Functions    []string `json:"functions"`     // Function names
}

// SelectItems creates migration items based on the selection request.
func (s *Service) SelectItems(migrationID string, req SelectItemsRequest) error {
	// Verify migration exists
	_, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Clear existing items for this migration
	_, err = s.db.Exec(`DELETE FROM _migration_items WHERE migration_id = ?`, migrationID)
	if err != nil {
		return fmt.Errorf("clear existing items: %w", err)
	}

	// Create items for boolean flags
	if req.Schema {
		if _, err := s.state.CreateItem(migrationID, ItemSchema, "schema"); err != nil {
			return fmt.Errorf("create schema item: %w", err)
		}
	}

	if req.Users {
		if _, err := s.state.CreateItem(migrationID, ItemUsers, "users"); err != nil {
			return fmt.Errorf("create users item: %w", err)
		}
	}

	if req.Identities {
		if _, err := s.state.CreateItem(migrationID, ItemIdentities, "identities"); err != nil {
			return fmt.Errorf("create identities item: %w", err)
		}
	}

	if req.RLS {
		if _, err := s.state.CreateItem(migrationID, ItemRLS, "rls"); err != nil {
			return fmt.Errorf("create rls item: %w", err)
		}
	}

	if req.StorageBuckets {
		if _, err := s.state.CreateItem(migrationID, ItemStorageBuckets, "storage_buckets"); err != nil {
			return fmt.Errorf("create storage_buckets item: %w", err)
		}
	}

	if req.Secrets {
		if _, err := s.state.CreateItem(migrationID, ItemSecrets, "secrets"); err != nil {
			return fmt.Errorf("create secrets item: %w", err)
		}
	}

	if req.AuthConfig {
		if _, err := s.state.CreateItem(migrationID, ItemAuthConfig, "auth_config"); err != nil {
			return fmt.Errorf("create auth_config item: %w", err)
		}
	}

	if req.OAuthConfig {
		if _, err := s.state.CreateItem(migrationID, ItemOAuthConfig, "oauth_config"); err != nil {
			return fmt.Errorf("create oauth_config item: %w", err)
		}
	}

	if req.EmailTemplates {
		if _, err := s.state.CreateItem(migrationID, ItemEmailTemplates, "email_templates"); err != nil {
			return fmt.Errorf("create email_templates item: %w", err)
		}
	}

	// Create items for named collections
	for _, tableName := range req.Data {
		if _, err := s.state.CreateItem(migrationID, ItemData, tableName); err != nil {
			return fmt.Errorf("create data item for %s: %w", tableName, err)
		}
	}

	for _, bucketID := range req.StorageFiles {
		if _, err := s.state.CreateItem(migrationID, ItemStorageFiles, bucketID); err != nil {
			return fmt.Errorf("create storage_files item for %s: %w", bucketID, err)
		}
	}

	for _, funcName := range req.Functions {
		if _, err := s.state.CreateItem(migrationID, ItemFunctions, funcName); err != nil {
			return fmt.Errorf("create functions item for %s: %w", funcName, err)
		}
	}

	return nil
}

// GetItems retrieves all items for a migration.
func (s *Service) GetItems(migrationID string) ([]*MigrationItem, error) {
	return s.state.GetItems(migrationID)
}

// MigrationProgress summarizes the progress of a migration.
type MigrationProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Pending   int `json:"pending"`
	Skipped   int `json:"skipped"`
}

// GetProgress calculates the current progress of a migration.
func (s *Service) GetProgress(migrationID string) (*MigrationProgress, error) {
	items, err := s.state.GetItems(migrationID)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	progress := &MigrationProgress{
		Total: len(items),
	}

	for _, item := range items {
		switch item.Status {
		case ItemCompleted:
			progress.Completed++
		case ItemFailed:
			progress.Failed++
		case ItemPending:
			progress.Pending++
		case ItemSkipped:
			progress.Skipped++
		// ItemInProgress and ItemRolledBack are not counted in these buckets
		}
	}

	return progress, nil
}

// getSupabaseClient returns a Supabase client for the given migration.
// It creates a new client from stored credentials if not already cached.
func (s *Service) getSupabaseClient(migrationID string) (*SupabaseClient, error) {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return nil, err
	}

	if len(m.CredentialsEncrypted) == 0 {
		return nil, fmt.Errorf("no Supabase credentials stored for migration %s", migrationID)
	}

	// TODO: Decrypt credentials when encryption is implemented
	token := string(m.CredentialsEncrypted)

	return NewSupabaseClient(token), nil
}
