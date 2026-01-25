// Package migration provides tools for migrating sblite databases to Supabase.
package migration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/markb/sblite/internal/migrate"
	"github.com/markb/sblite/internal/schema"

	_ "github.com/lib/pq" // PostgreSQL driver for Supabase connection
)

// quoteIdentifier quotes a SQL identifier to prevent injection.
// It validates the identifier contains only safe characters and double-quotes it.
func quoteIdentifier(name string) (string, error) {
	// Validate: only allow alphanumeric, underscore
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return "", fmt.Errorf("invalid identifier: %s", name)
		}
	}
	// Double-quote for PostgreSQL
	return fmt.Sprintf(`"%s"`, name), nil
}

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
	JWTSecret    string // For decrypting function secrets
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

	// Encrypt the token before storing
	encryptedToken, err := s.encryptCredential(token)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}
	m.CredentialsEncrypted = encryptedToken

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

// SetDatabasePassword stores the Supabase database password for direct PostgreSQL connections.
// The password is needed to execute schema DDL and data migrations.
// The password is encrypted before storage using AES-GCM with the JWT secret.
func (s *Service) SetDatabasePassword(migrationID, password string) error {
	// Encrypt password before storing
	encryptedPassword, err := s.encryptCredential(password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	// Store encrypted password (base64 encoded) in _dashboard table with a migration-specific key
	encoded := base64.StdEncoding.EncodeToString(encryptedPassword)
	_, err = s.db.Exec(`
		INSERT INTO _dashboard (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, "migration_db_password_"+migrationID, encoded)
	if err != nil {
		return fmt.Errorf("store database password: %w", err)
	}
	return nil
}

// getDatabasePassword retrieves and decrypts the stored database password for a migration.
func (s *Service) getDatabasePassword(migrationID string) (string, error) {
	var encoded string
	err := s.db.QueryRow(`SELECT value FROM _dashboard WHERE key = ?`, "migration_db_password_"+migrationID).Scan(&encoded)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("database password not set for migration %s", migrationID)
	}
	if err != nil {
		return "", fmt.Errorf("get database password: %w", err)
	}

	// Decode from base64
	encryptedPassword, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode password: %w", err)
	}

	// Decrypt password
	password, err := s.decryptCredential(encryptedPassword)
	if err != nil {
		return "", fmt.Errorf("decrypt password: %w", err)
	}

	return password, nil
}

// getPostgresConnection creates a direct connection to the Supabase PostgreSQL database.
func (s *Service) getPostgresConnection(migration *Migration) (*sql.DB, error) {
	password, err := s.getDatabasePassword(migration.ID)
	if err != nil {
		return nil, err
	}

	// Supabase pooler connection string format:
	// postgres://postgres.[PROJECT_REF]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres
	// For direct connection without pooler:
	// postgres://postgres:[PASSWORD]@db.[PROJECT_REF].supabase.co:5432/postgres
	connStr := fmt.Sprintf(
		"postgres://postgres:%s@db.%s.supabase.co:5432/postgres?sslmode=require",
		password,
		migration.SupabaseProjectRef,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
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

// RetryFailedItems resets all failed items to pending so they can be retried.
func (s *Service) RetryFailedItems(migrationID string) error {
	// Verify migration exists
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Only allow retry on failed migrations
	if m.Status != StatusFailed {
		return fmt.Errorf("can only retry failed migrations (current status: %s)", m.Status)
	}

	// Get all items for this migration
	items, err := s.state.GetItems(migrationID)
	if err != nil {
		return fmt.Errorf("get items: %w", err)
	}

	// Reset failed items to pending
	for _, item := range items {
		if item.Status == ItemFailed {
			item.Status = ItemPending
			item.ErrorMessage = ""
			item.StartedAt = nil
			item.CompletedAt = nil
			if err := s.state.UpdateItem(item); err != nil {
				return fmt.Errorf("reset item %s: %w", item.ID, err)
			}
		}
	}

	// Reset migration status to pending
	m.Status = StatusPending
	m.ErrorMessage = ""
	m.CompletedAt = nil
	if err := s.state.UpdateMigration(m); err != nil {
		return fmt.Errorf("reset migration status: %w", err)
	}

	return nil
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

	// Decrypt credentials
	token, err := s.decryptCredential(m.CredentialsEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}

	return NewSupabaseClient(token), nil
}

// RunMigration executes the migration, processing all pending items.
func (s *Service) RunMigration(migrationID string) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Validate migration is ready
	if m.SupabaseProjectRef == "" {
		return fmt.Errorf("no Supabase project selected for migration")
	}

	items, err := s.state.GetItems(migrationID)
	if err != nil {
		return fmt.Errorf("get items: %w", err)
	}

	if len(items) == 0 {
		return fmt.Errorf("no items selected for migration")
	}

	// Update migration status to in_progress
	m.Status = StatusInProgress
	if err := s.state.UpdateMigration(m); err != nil {
		return fmt.Errorf("update migration status: %w", err)
	}

	// Track overall success
	hasFailures := false

	// Process each pending item
	for _, item := range items {
		if item.Status != ItemPending {
			continue
		}

		// Run the appropriate migrator based on item type
		var migrateErr error
		switch item.ItemType {
		case ItemSchema:
			migrateErr = s.migrateSchema(m, item)
		case ItemData:
			migrateErr = s.migrateData(m, item)
		case ItemUsers:
			migrateErr = s.migrateUsers(m, item)
		case ItemIdentities:
			migrateErr = s.migrateIdentities(m, item)
		case ItemRLS:
			migrateErr = s.migrateRLS(m, item)
		case ItemStorageBuckets:
			migrateErr = s.migrateStorageBuckets(m, item)
		case ItemStorageFiles:
			migrateErr = s.migrateStorageFiles(m, item)
		case ItemFunctions:
			migrateErr = s.migrateFunctions(m, item)
		case ItemSecrets:
			migrateErr = s.migrateSecrets(m, item)
		case ItemAuthConfig:
			migrateErr = s.migrateAuthConfig(m, item)
		case ItemOAuthConfig:
			migrateErr = s.migrateOAuthConfig(m, item)
		case ItemEmailTemplates:
			migrateErr = s.migrateEmailTemplates(m, item)
		default:
			migrateErr = fmt.Errorf("unknown item type: %s", item.ItemType)
		}

		if migrateErr != nil {
			hasFailures = true
			// Error is already stored in item by the migrator
		}
	}

	// Update migration status based on results
	now := time.Now().UTC()
	m.CompletedAt = &now

	if hasFailures {
		m.Status = StatusFailed
		m.ErrorMessage = "one or more items failed to migrate"
	} else {
		m.Status = StatusCompleted
	}

	if err := s.state.UpdateMigration(m); err != nil {
		return fmt.Errorf("update migration status: %w", err)
	}

	return nil
}

// markItemStarted marks an item as in_progress with a start time.
func (s *Service) markItemStarted(item *MigrationItem) error {
	now := time.Now().UTC()
	item.Status = ItemInProgress
	item.StartedAt = &now
	return s.state.UpdateItem(item)
}

// markItemCompleted marks an item as completed with rollback info.
func (s *Service) markItemCompleted(item *MigrationItem, rollbackInfo interface{}) error {
	now := time.Now().UTC()
	item.Status = ItemCompleted
	item.CompletedAt = &now

	if rollbackInfo != nil {
		data, err := json.Marshal(rollbackInfo)
		if err != nil {
			return fmt.Errorf("marshal rollback info: %w", err)
		}
		item.RollbackInfo = string(data)
	}

	return s.state.UpdateItem(item)
}

// markItemFailed marks an item as failed with an error message.
func (s *Service) markItemFailed(item *MigrationItem, err error) error {
	now := time.Now().UTC()
	item.Status = ItemFailed
	item.CompletedAt = &now
	item.ErrorMessage = err.Error()
	return s.state.UpdateItem(item)
}

// SchemaRollbackInfo contains info needed to rollback schema migration.
type SchemaRollbackInfo struct {
	Tables []string `json:"tables"`
}

// migrateSchema exports and executes DDL in Supabase.
func (s *Service) migrateSchema(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Get schema exporter
	sch := schema.New(s.db)
	exporter := migrate.New(sch)

	// Export DDL
	ddl, err := exporter.ExportDDL()
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("export DDL: %w", err))
		return err
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("connect to postgres: %w", err))
		return err
	}
	defer pgDB.Close()

	// Execute DDL
	_, err = pgDB.Exec(ddl)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("execute DDL: %w", err))
		return err
	}

	// Get list of created tables for rollback
	tables, err := sch.ListTables()
	if err != nil {
		tables = []string{} // Non-fatal
	}

	rollbackInfo := SchemaRollbackInfo{Tables: tables}
	return s.markItemCompleted(item, rollbackInfo)
}

// DataRollbackInfo contains info needed to rollback data migration.
type DataRollbackInfo struct {
	TableName string `json:"table_name"`
	RowCount  int    `json:"row_count"`
}

// migrateData migrates table data from sblite to Supabase.
func (s *Service) migrateData(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	tableName := item.ItemName

	// Validate and quote identifiers to prevent SQL injection
	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("invalid table name: %w", err))
		return err
	}

	// Get all rows from sblite table
	rows, err := s.db.Query(fmt.Sprintf("SELECT * FROM %s", quotedTable))
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query table: %w", err))
		return err
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get columns: %w", err))
		return err
	}

	// Validate and quote all column names
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedCol, err := quoteIdentifier(col)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("invalid column name %s: %w", col, err))
			return err
		}
		quotedColumns[i] = quotedCol
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("connect to postgres: %w", err))
		return err
	}
	defer pgDB.Close()

	// Start transaction for batch insert
	tx, err := pgDB.Begin()
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("begin transaction: %w", err))
		return err
	}

	rowCount := 0

	// Process each row
	for rows.Next() {
		// Create slice to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("scan row: %w", err))
			return err
		}

		// Build INSERT statement with placeholders
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		insertSQL := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			quotedTable,
			strings.Join(quotedColumns, ", "),
			strings.Join(placeholders, ", "),
		)

		// Execute insert
		if _, err := tx.Exec(insertSQL, values...); err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("insert row: %w", err))
			return err
		}

		rowCount++
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		s.markItemFailed(item, fmt.Errorf("iterate rows: %w", err))
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		s.markItemFailed(item, fmt.Errorf("commit transaction: %w", err))
		return err
	}

	rollbackInfo := DataRollbackInfo{TableName: tableName, RowCount: rowCount}
	return s.markItemCompleted(item, rollbackInfo)
}

// UsersRollbackInfo contains info needed to rollback users migration.
type UsersRollbackInfo struct {
	UserIDs []string `json:"user_ids"`
}

// migrateUsers migrates auth users from sblite to Supabase.
func (s *Service) migrateUsers(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Query users from sblite
	rows, err := s.db.Query(`
		SELECT id, email, encrypted_password, email_confirmed_at, phone, phone_confirmed_at,
		       confirmation_token, recovery_token, email_change_token_new, email_change,
		       last_sign_in_at, raw_app_meta_data, raw_user_meta_data, is_super_admin,
		       created_at, updated_at, is_anonymous
		FROM auth_users
	`)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query users: %w", err))
		return err
	}
	defer rows.Close()

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("connect to postgres: %w", err))
		return err
	}
	defer pgDB.Close()

	tx, err := pgDB.Begin()
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("begin transaction: %w", err))
		return err
	}

	var userIDs []string

	for rows.Next() {
		var (
			id, email, encryptedPassword                               string
			emailConfirmedAt, phone, phoneConfirmedAt                  sql.NullString
			confirmationToken, recoveryToken                           sql.NullString
			emailChangeTokenNew, emailChange                           sql.NullString
			lastSignInAt, rawAppMetaData, rawUserMetaData              sql.NullString
			isSuperAdmin                                               int
			createdAt, updatedAt                                       string
			isAnonymous                                                sql.NullInt64
		)

		err := rows.Scan(
			&id, &email, &encryptedPassword, &emailConfirmedAt, &phone, &phoneConfirmedAt,
			&confirmationToken, &recoveryToken, &emailChangeTokenNew, &emailChange,
			&lastSignInAt, &rawAppMetaData, &rawUserMetaData, &isSuperAdmin,
			&createdAt, &updatedAt, &isAnonymous,
		)
		if err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("scan user: %w", err))
			return err
		}

		// Insert into Supabase auth.users
		_, err = tx.Exec(`
			INSERT INTO auth.users (
				id, email, encrypted_password, email_confirmed_at, phone, phone_confirmed_at,
				confirmation_token, recovery_token, email_change_token_new, email_change,
				last_sign_in_at, raw_app_meta_data, raw_user_meta_data, is_super_admin,
				created_at, updated_at, is_anonymous
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`,
			id, email, encryptedPassword, nullStr(emailConfirmedAt), nullStr(phone), nullStr(phoneConfirmedAt),
			nullStr(confirmationToken), nullStr(recoveryToken), nullStr(emailChangeTokenNew), nullStr(emailChange),
			nullStr(lastSignInAt), nullStr(rawAppMetaData), nullStr(rawUserMetaData), isSuperAdmin == 1,
			createdAt, updatedAt, isAnonymous.Valid && isAnonymous.Int64 == 1,
		)
		if err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("insert user %s: %w", id, err))
			return err
		}

		userIDs = append(userIDs, id)
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		s.markItemFailed(item, fmt.Errorf("iterate users: %w", err))
		return err
	}

	if err := tx.Commit(); err != nil {
		s.markItemFailed(item, fmt.Errorf("commit transaction: %w", err))
		return err
	}

	rollbackInfo := UsersRollbackInfo{UserIDs: userIDs}
	return s.markItemCompleted(item, rollbackInfo)
}

// IdentitiesRollbackInfo contains info needed to rollback identities migration.
type IdentitiesRollbackInfo struct {
	IdentityIDs []string `json:"identity_ids"`
}

// migrateIdentities migrates OAuth identities from sblite to Supabase.
func (s *Service) migrateIdentities(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Query identities from sblite
	rows, err := s.db.Query(`
		SELECT id, user_id, identity_data, provider, provider_id, last_sign_in_at, created_at, updated_at
		FROM auth_identities
	`)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query identities: %w", err))
		return err
	}
	defer rows.Close()

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("connect to postgres: %w", err))
		return err
	}
	defer pgDB.Close()

	tx, err := pgDB.Begin()
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("begin transaction: %w", err))
		return err
	}

	var identityIDs []string

	for rows.Next() {
		var (
			id, userID, identityData, provider, providerID string
			lastSignInAt, createdAt, updatedAt             sql.NullString
		)

		err := rows.Scan(&id, &userID, &identityData, &provider, &providerID, &lastSignInAt, &createdAt, &updatedAt)
		if err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("scan identity: %w", err))
			return err
		}

		// Insert into Supabase auth.identities
		_, err = tx.Exec(`
			INSERT INTO auth.identities (id, user_id, identity_data, provider, provider_id, last_sign_in_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, id, userID, identityData, provider, providerID, nullStr(lastSignInAt), nullStr(createdAt), nullStr(updatedAt))
		if err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("insert identity %s: %w", id, err))
			return err
		}

		identityIDs = append(identityIDs, id)
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		s.markItemFailed(item, fmt.Errorf("iterate identities: %w", err))
		return err
	}

	if err := tx.Commit(); err != nil {
		s.markItemFailed(item, fmt.Errorf("commit transaction: %w", err))
		return err
	}

	rollbackInfo := IdentitiesRollbackInfo{IdentityIDs: identityIDs}
	return s.markItemCompleted(item, rollbackInfo)
}

// RLSRollbackInfo contains info needed to rollback RLS policies migration.
type RLSRollbackInfo struct {
	Policies []struct {
		TableName  string `json:"table_name"`
		PolicyName string `json:"policy_name"`
	} `json:"policies"`
}

// migrateRLS migrates RLS policies from sblite to Supabase.
func (s *Service) migrateRLS(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Query RLS policies from sblite
	rows, err := s.db.Query(`
		SELECT table_name, policy_name, command, using_expr, check_expr, enabled
		FROM _rls_policies
	`)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query policies: %w", err))
		return err
	}
	defer rows.Close()

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("connect to postgres: %w", err))
		return err
	}
	defer pgDB.Close()

	var rollbackInfo RLSRollbackInfo

	// Valid commands for RLS policies
	validCommands := map[string]bool{
		"ALL":    true,
		"SELECT": true,
		"INSERT": true,
		"UPDATE": true,
		"DELETE": true,
	}

	for rows.Next() {
		var tableName, policyName, command string
		var usingExpr, checkExpr sql.NullString
		var enabled int

		err := rows.Scan(&tableName, &policyName, &command, &usingExpr, &checkExpr, &enabled)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("scan policy: %w", err))
			return err
		}

		if enabled == 0 {
			continue // Skip disabled policies
		}

		// Validate and quote identifiers to prevent SQL injection
		quotedTable, err := quoteIdentifier(tableName)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("invalid table name %s: %w", tableName, err))
			return err
		}

		quotedPolicy, err := quoteIdentifier(policyName)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("invalid policy name %s: %w", policyName, err))
			return err
		}

		// Validate command is a known RLS command
		commandUpper := strings.ToUpper(command)
		if !validCommands[commandUpper] {
			s.markItemFailed(item, fmt.Errorf("invalid policy command: %s", command))
			return fmt.Errorf("invalid policy command: %s", command)
		}

		// Enable RLS on the table first
		_, err = pgDB.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", quotedTable))
		if err != nil {
			// Table might already have RLS enabled, continue
		}

		// Build CREATE POLICY statement
		policySQL := fmt.Sprintf("CREATE POLICY %s ON %s FOR %s", quotedPolicy, quotedTable, commandUpper)
		if usingExpr.Valid && usingExpr.String != "" {
			policySQL += fmt.Sprintf(" USING (%s)", usingExpr.String)
		}
		if checkExpr.Valid && checkExpr.String != "" {
			policySQL += fmt.Sprintf(" WITH CHECK (%s)", checkExpr.String)
		}

		_, err = pgDB.Exec(policySQL)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("create policy %s.%s: %w", tableName, policyName, err))
			return err
		}

		rollbackInfo.Policies = append(rollbackInfo.Policies, struct {
			TableName  string `json:"table_name"`
			PolicyName string `json:"policy_name"`
		}{TableName: tableName, PolicyName: policyName})
	}

	if err := rows.Err(); err != nil {
		s.markItemFailed(item, fmt.Errorf("iterate policies: %w", err))
		return err
	}

	return s.markItemCompleted(item, rollbackInfo)
}

// BucketsRollbackInfo contains info needed to rollback storage buckets migration.
type BucketsRollbackInfo struct {
	BucketIDs []string `json:"bucket_ids"`
}

// migrateStorageBuckets migrates storage buckets from sblite to Supabase.
func (s *Service) migrateStorageBuckets(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Query storage buckets from sblite
	rows, err := s.db.Query(`
		SELECT id, name, public, file_size_limit, allowed_mime_types, created_at, updated_at
		FROM storage_buckets
	`)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query buckets: %w", err))
		return err
	}
	defer rows.Close()

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("connect to postgres: %w", err))
		return err
	}
	defer pgDB.Close()

	tx, err := pgDB.Begin()
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("begin transaction: %w", err))
		return err
	}

	var bucketIDs []string

	for rows.Next() {
		var (
			id, name             string
			public               int
			fileSizeLimit        sql.NullInt64
			allowedMimeTypes     sql.NullString
			createdAt, updatedAt string
		)

		err := rows.Scan(&id, &name, &public, &fileSizeLimit, &allowedMimeTypes, &createdAt, &updatedAt)
		if err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("scan bucket: %w", err))
			return err
		}

		// Insert into Supabase storage.buckets
		_, err = tx.Exec(`
			INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, id, name, public == 1, nullInt64(fileSizeLimit), nullStr(allowedMimeTypes), createdAt, updatedAt)
		if err != nil {
			tx.Rollback()
			s.markItemFailed(item, fmt.Errorf("insert bucket %s: %w", id, err))
			return err
		}

		bucketIDs = append(bucketIDs, id)
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		s.markItemFailed(item, fmt.Errorf("iterate buckets: %w", err))
		return err
	}

	if err := tx.Commit(); err != nil {
		s.markItemFailed(item, fmt.Errorf("commit transaction: %w", err))
		return err
	}

	rollbackInfo := BucketsRollbackInfo{BucketIDs: bucketIDs}
	return s.markItemCompleted(item, rollbackInfo)
}

// FilesRollbackInfo contains info needed to rollback storage files migration.
type FilesRollbackInfo struct {
	BucketID string   `json:"bucket_id"`
	Paths    []string `json:"paths"`
}

// migrateStorageFiles migrates storage files for a specific bucket from sblite to Supabase.
func (s *Service) migrateStorageFiles(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	bucketID := item.ItemName

	// Get Supabase client and API keys
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get supabase client: %w", err))
		return err
	}

	apiKeys, err := client.GetAPIKeys(m.SupabaseProjectRef)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get api keys: %w", err))
		return err
	}

	// Find service_role key
	var serviceKey string
	for _, key := range apiKeys {
		if key.Name == "service_role" {
			serviceKey = key.APIKey
			break
		}
	}
	if serviceKey == "" {
		s.markItemFailed(item, fmt.Errorf("service_role key not found"))
		return fmt.Errorf("service_role key not found")
	}

	// Query storage objects for this bucket from sblite
	rows, err := s.db.Query(`
		SELECT name, content_type
		FROM storage_objects
		WHERE bucket_id = ?
	`, bucketID)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query objects: %w", err))
		return err
	}
	defer rows.Close()

	storageURL := fmt.Sprintf("https://%s.supabase.co/storage/v1/object/%s", m.SupabaseProjectRef, bucketID)
	var uploadedPaths []string

	for rows.Next() {
		var name, contentType string
		if err := rows.Scan(&name, &contentType); err != nil {
			s.markItemFailed(item, fmt.Errorf("scan object: %w", err))
			return err
		}

		// Read file from local storage
		localPath := filepath.Join(s.serverConfig.StorageDir, bucketID, name)
		fileData, err := os.ReadFile(localPath)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("read file %s: %w", name, err))
			return err
		}

		// Upload to Supabase Storage API
		uploadURL := fmt.Sprintf("%s/%s", storageURL, name)
		req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(fileData))
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("create request for %s: %w", name, err))
			return err
		}

		req.Header.Set("Authorization", "Bearer "+serviceKey)
		req.Header.Set("Content-Type", contentType)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("upload %s: %w", name, err))
			return err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			s.markItemFailed(item, fmt.Errorf("upload %s: status %d", name, resp.StatusCode))
			return fmt.Errorf("upload %s: status %d", name, resp.StatusCode)
		}

		uploadedPaths = append(uploadedPaths, name)
	}

	if err := rows.Err(); err != nil {
		s.markItemFailed(item, fmt.Errorf("iterate objects: %w", err))
		return err
	}

	rollbackInfo := FilesRollbackInfo{BucketID: bucketID, Paths: uploadedPaths}
	return s.markItemCompleted(item, rollbackInfo)
}

// FunctionsRollbackInfo contains info needed to rollback functions migration.
type FunctionsRollbackInfo struct {
	FunctionName string `json:"function_name"`
}

// migrateFunctions deploys an edge function to Supabase.
func (s *Service) migrateFunctions(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	funcName := item.ItemName
	funcDir := filepath.Join(s.serverConfig.FunctionsDir, funcName)

	// Verify function exists
	if _, err := os.Stat(funcDir); os.IsNotExist(err) {
		s.markItemFailed(item, fmt.Errorf("function directory not found: %s", funcDir))
		return err
	}

	// Create tar.gz archive of function
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.Walk(funcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(funcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = funcName + "/" + relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Write file content if it's a regular file
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if _, err := tw.Write(data); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("create archive: %w", err))
		return err
	}

	if err := tw.Close(); err != nil {
		s.markItemFailed(item, fmt.Errorf("close tar writer: %w", err))
		return err
	}
	if err := gw.Close(); err != nil {
		s.markItemFailed(item, fmt.Errorf("close gzip writer: %w", err))
		return err
	}

	// Get Supabase client
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get supabase client: %w", err))
		return err
	}

	// Get function metadata from sblite to check verify_jwt setting
	verifyJWT := true
	var verifyJWTInt int
	err = s.db.QueryRow("SELECT verify_jwt FROM _functions_metadata WHERE name = ?", funcName).Scan(&verifyJWTInt)
	if err == nil {
		verifyJWT = verifyJWTInt == 1
	}

	// Deploy function
	metadata := FunctionMetadata{
		Name:           funcName,
		EntrypointPath: funcName + "/index.ts",
		VerifyJWT:      &verifyJWT,
	}

	if err := client.DeployFunction(m.SupabaseProjectRef, funcName, metadata, buf.Bytes()); err != nil {
		s.markItemFailed(item, fmt.Errorf("deploy function: %w", err))
		return err
	}

	rollbackInfo := FunctionsRollbackInfo{FunctionName: funcName}
	return s.markItemCompleted(item, rollbackInfo)
}

// SecretsRollbackInfo contains info needed to rollback secrets migration.
type SecretsRollbackInfo struct {
	SecretNames []string `json:"secret_names"`
}

// migrateSecrets migrates function secrets from sblite to Supabase.
func (s *Service) migrateSecrets(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Query secrets from sblite (need the actual values, not just names)
	// Secrets are encrypted in _functions_secrets, we need to decrypt them
	rows, err := s.db.Query("SELECT name, value FROM _functions_secrets")
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("query secrets: %w", err))
		return err
	}
	defer rows.Close()

	// Get Supabase client
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get supabase client: %w", err))
		return err
	}

	var secrets []Secret
	var secretNames []string

	// We need the JWT secret to decrypt the values
	// For now, we'll need to read the encrypted values and decrypt them
	// This requires access to the functions store which uses the JWT secret
	for rows.Next() {
		var name, encryptedValue string
		if err := rows.Scan(&name, &encryptedValue); err != nil {
			s.markItemFailed(item, fmt.Errorf("scan secret: %w", err))
			return err
		}

		// Decrypt the value using the server's JWT secret
		// We need to create a temporary store for decryption
		if s.serverConfig.JWTSecret == "" {
			s.markItemFailed(item, fmt.Errorf("JWT secret not configured, cannot decrypt secrets"))
			return fmt.Errorf("JWT secret not configured")
		}

		value, err := decryptSecret(encryptedValue, s.serverConfig.JWTSecret)
		if err != nil {
			s.markItemFailed(item, fmt.Errorf("decrypt secret %s: %w", name, err))
			return err
		}

		secrets = append(secrets, Secret{Name: name, Value: value})
		secretNames = append(secretNames, name)
	}

	if err := rows.Err(); err != nil {
		s.markItemFailed(item, fmt.Errorf("iterate secrets: %w", err))
		return err
	}

	if len(secrets) > 0 {
		if err := client.CreateSecrets(m.SupabaseProjectRef, secrets); err != nil {
			s.markItemFailed(item, fmt.Errorf("create secrets: %w", err))
			return err
		}
	}

	rollbackInfo := SecretsRollbackInfo{SecretNames: secretNames}
	return s.markItemCompleted(item, rollbackInfo)
}

// migrateAuthConfig migrates auth configuration from sblite to Supabase.
func (s *Service) migrateAuthConfig(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Get Supabase client
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get supabase client: %w", err))
		return err
	}

	// Query auth settings from sblite _dashboard table
	var allowAnonymous string
	err = s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'allow_anonymous'").Scan(&allowAnonymous)
	if err != nil && err != sql.ErrNoRows {
		s.markItemFailed(item, fmt.Errorf("query auth settings: %w", err))
		return err
	}

	// Build auth config update
	config := AuthConfig{}
	if allowAnonymous == "true" {
		config["EXTERNAL_ANONYMOUS_USERS_ENABLED"] = true
	}

	// Only update if we have settings to apply
	if len(config) > 0 {
		if err := client.UpdateAuthConfig(m.SupabaseProjectRef, config); err != nil {
			s.markItemFailed(item, fmt.Errorf("update auth config: %w", err))
			return err
		}
	}

	// No rollback info for config changes (hard to restore previous values)
	return s.markItemCompleted(item, nil)
}

// migrateOAuthConfig migrates OAuth provider configuration from sblite to Supabase.
func (s *Service) migrateOAuthConfig(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Get Supabase client
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		s.markItemFailed(item, fmt.Errorf("get supabase client: %w", err))
		return err
	}

	// Query OAuth settings from sblite _dashboard table
	config := AuthConfig{}

	// Google OAuth
	var googleEnabled, googleClientID, googleSecret string
	s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'oauth_google_enabled'").Scan(&googleEnabled)
	s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'oauth_google_client_id'").Scan(&googleClientID)
	s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'oauth_google_client_secret'").Scan(&googleSecret)

	if googleEnabled == "true" && googleClientID != "" {
		config["EXTERNAL_GOOGLE_ENABLED"] = true
		config["EXTERNAL_GOOGLE_CLIENT_ID"] = googleClientID
		if googleSecret != "" {
			config["EXTERNAL_GOOGLE_SECRET"] = googleSecret
		}
	}

	// GitHub OAuth
	var githubEnabled, githubClientID, githubSecret string
	s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'oauth_github_enabled'").Scan(&githubEnabled)
	s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'oauth_github_client_id'").Scan(&githubClientID)
	s.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'oauth_github_client_secret'").Scan(&githubSecret)

	if githubEnabled == "true" && githubClientID != "" {
		config["EXTERNAL_GITHUB_ENABLED"] = true
		config["EXTERNAL_GITHUB_CLIENT_ID"] = githubClientID
		if githubSecret != "" {
			config["EXTERNAL_GITHUB_SECRET"] = githubSecret
		}
	}

	// Only update if we have settings to apply
	if len(config) > 0 {
		if err := client.UpdateAuthConfig(m.SupabaseProjectRef, config); err != nil {
			s.markItemFailed(item, fmt.Errorf("update oauth config: %w", err))
			return err
		}
	}

	// No rollback info for config changes
	return s.markItemCompleted(item, nil)
}

// migrateEmailTemplates marks email templates migration as completed with a note.
// Email templates must be configured via the Supabase Dashboard UI.
func (s *Service) migrateEmailTemplates(m *Migration, item *MigrationItem) error {
	if err := s.markItemStarted(item); err != nil {
		return err
	}

	// Email templates cannot be migrated via API
	// They must be manually configured in the Supabase Dashboard
	metadata := map[string]string{
		"note": "Email templates must be configured manually in the Supabase Dashboard under Authentication > Email Templates",
	}

	metadataJSON, _ := json.Marshal(metadata)
	item.Metadata = metadataJSON

	return s.markItemCompleted(item, nil)
}

// Helper functions for null handling

func nullStr(ns sql.NullString) interface{} {
	if ns.Valid {
		return ns.String
	}
	return nil
}

func nullInt64(ni sql.NullInt64) interface{} {
	if ni.Valid {
		return ni.Int64
	}
	return nil
}

// encryptCredential encrypts a credential using AES-GCM with the JWT secret as key.
func (s *Service) encryptCredential(plaintext string) ([]byte, error) {
	if s.serverConfig == nil || s.serverConfig.JWTSecret == "" {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	key := sha256.Sum256([]byte(s.serverConfig.JWTSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// decryptCredential decrypts a credential encrypted with encryptCredential.
func (s *Service) decryptCredential(ciphertext []byte) (string, error) {
	if s.serverConfig == nil || s.serverConfig.JWTSecret == "" {
		return "", fmt.Errorf("JWT secret not configured")
	}

	key := sha256.Sum256([]byte(s.serverConfig.JWTSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// decryptSecret decrypts an AES-GCM encrypted secret using the JWT secret.
// This mirrors the encryption in internal/functions/store.go.
func decryptSecret(encrypted, jwtSecret string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	// Derive key from JWT secret using SHA-256
	hash := sha256.Sum256([]byte(jwtSecret))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// RunBasicVerification runs basic verification checks for a migration.
// It creates a verification record, executes the checks, and stores the results.
func (s *Service) RunBasicVerification(migrationID string) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return fmt.Errorf("get migration: %w", err)
	}

	// Verify migration has a project selected
	if m.SupabaseProjectRef == "" {
		return fmt.Errorf("no Supabase project selected for migration")
	}

	// Create verification record
	verification, err := s.state.CreateVerification(migrationID, LayerBasic)
	if err != nil {
		return fmt.Errorf("create verification: %w", err)
	}

	// Mark as running
	now := time.Now().UTC()
	verification.Status = VerifyRunning
	verification.StartedAt = &now
	if err := s.state.UpdateVerification(verification); err != nil {
		return fmt.Errorf("update verification status: %w", err)
	}

	// Get migration items
	items, err := s.state.GetItems(migrationID)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get items: %w", err))
		return fmt.Errorf("get items: %w", err)
	}

	// Get Supabase client
	client, err := s.getSupabaseClient(migrationID)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get supabase client: %w", err))
		return fmt.Errorf("get supabase client: %w", err)
	}

	// Get Postgres connection
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get postgres connection: %w", err))
		return fmt.Errorf("get postgres connection: %w", err)
	}
	defer pgDB.Close()

	// Convert items to verification format
	verifyItems := make([]*verificationItem, len(items))
	for i, item := range items {
		verifyItems[i] = &verificationItem{
			ID:          item.ID,
			MigrationID: item.MigrationID,
			ItemType:    string(item.ItemType),
			ItemName:    item.ItemName,
			Status:      string(item.Status),
			Metadata:    item.Metadata,
		}
	}

	// Create verifier
	verifier := newBasicVerifier(s.db, pgDB, &supabaseClientAdapter{client}, m, verifyItems)

	// Run checks
	result, err := verifier.runBasicChecks()
	if err != nil {
		s.markVerificationFailed(verification, err)
		return fmt.Errorf("run basic checks: %w", err)
	}

	// Store results
	resultsJSON, err := json.Marshal(result)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("marshal results: %w", err))
		return fmt.Errorf("marshal results: %w", err)
	}

	completedAt := time.Now().UTC()
	verification.CompletedAt = &completedAt
	verification.Results = resultsJSON

	if result.Summary.Failed > 0 {
		verification.Status = VerifyFailed
	} else {
		verification.Status = VerifyPassed
	}

	if err := s.state.UpdateVerification(verification); err != nil {
		return fmt.Errorf("update verification: %w", err)
	}

	return nil
}

// markVerificationFailed marks a verification as failed with an error.
func (s *Service) markVerificationFailed(v *Verification, err error) {
	now := time.Now().UTC()
	v.Status = VerifyFailed
	v.CompletedAt = &now

	result := map[string]interface{}{
		"error": err.Error(),
	}
	resultsJSON, _ := json.Marshal(result)
	v.Results = resultsJSON

	s.state.UpdateVerification(v)
}

// GetVerifications retrieves all verifications for a migration.
func (s *Service) GetVerifications(migrationID string) ([]*Verification, error) {
	return s.state.GetVerifications(migrationID)
}

// verificationItem is an internal representation of MigrationItem for verification.
type verificationItem struct {
	ID          string          `json:"id"`
	MigrationID string          `json:"migration_id"`
	ItemType    string          `json:"item_type"`
	ItemName    string          `json:"item_name"`
	Status      string          `json:"status"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// supabaseClientAdapter adapts SupabaseClient to the verification interface.
type supabaseClientAdapter struct {
	client *SupabaseClient
}

func (a *supabaseClientAdapter) ListFunctions(projectRef string) ([]functionInfo, error) {
	funcs, err := a.client.ListFunctions(projectRef)
	if err != nil {
		return nil, err
	}
	result := make([]functionInfo, len(funcs))
	for i, f := range funcs {
		result[i] = functionInfo{
			Slug:      f.Slug,
			Name:      f.Name,
			Status:    f.Status,
			VerifyJWT: f.VerifyJWT,
		}
	}
	return result, nil
}

func (a *supabaseClientAdapter) ListSecrets(projectRef string) ([]secretInfo, error) {
	secrets, err := a.client.ListSecrets(projectRef)
	if err != nil {
		return nil, err
	}
	result := make([]secretInfo, len(secrets))
	for i, s := range secrets {
		result[i] = secretInfo{Name: s.Name}
	}
	return result, nil
}

func (a *supabaseClientAdapter) GetAuthConfig(projectRef string) (map[string]interface{}, error) {
	return a.client.GetAuthConfig(projectRef)
}

// Internal types for verification (to avoid circular imports)

type functionInfo struct {
	Slug      string
	Name      string
	Status    string
	VerifyJWT bool
}

type secretInfo struct {
	Name string
}

type verificationCheckResult struct {
	Name    string      `json:"name"`
	Passed  bool        `json:"passed"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

type verificationResult struct {
	Layer   VerificationLayer  `json:"layer"`
	Status  VerificationStatus `json:"status"`
	Checks  []verificationCheckResult `json:"checks"`
	Summary struct {
		Total  int `json:"total"`
		Passed int `json:"passed"`
		Failed int `json:"failed"`
	} `json:"summary"`
}

// supabaseClientVerifier is the interface for Supabase client operations needed by verification.
type supabaseClientVerifier interface {
	ListFunctions(projectRef string) ([]functionInfo, error)
	ListSecrets(projectRef string) ([]secretInfo, error)
	GetAuthConfig(projectRef string) (map[string]interface{}, error)
}

// basicVerifier performs basic verification checks after migration.
type basicVerifier struct {
	sbliteDB       *sql.DB
	supabaseDB     *sql.DB
	supabaseClient supabaseClientVerifier
	migration      *Migration
	items          []*verificationItem
}

func newBasicVerifier(
	sbliteDB *sql.DB,
	supabaseDB *sql.DB,
	supabaseClient supabaseClientVerifier,
	migration *Migration,
	items []*verificationItem,
) *basicVerifier {
	return &basicVerifier{
		sbliteDB:       sbliteDB,
		supabaseDB:     supabaseDB,
		supabaseClient: supabaseClient,
		migration:      migration,
		items:          items,
	}
}

func (v *basicVerifier) runBasicChecks() (*verificationResult, error) {
	result := &verificationResult{
		Layer:  LayerBasic,
		Status: VerifyRunning,
		Checks: []verificationCheckResult{},
	}

	// Group items by type for efficient checking
	itemsByType := make(map[string][]*verificationItem)
	for _, item := range v.items {
		if item.Status == "completed" {
			itemsByType[item.ItemType] = append(itemsByType[item.ItemType], item)
		}
	}

	// Run checks based on what was migrated
	if _, ok := itemsByType["schema"]; ok {
		checks := v.checkTablesExist()
		result.Checks = append(result.Checks, checks...)
	}

	if dataItems, ok := itemsByType["data"]; ok {
		checks := v.checkColumnsMatch(dataItems)
		result.Checks = append(result.Checks, checks...)
	}

	if funcItems, ok := itemsByType["functions"]; ok {
		checks := v.checkFunctionsDeployed(funcItems)
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["storage_buckets"]; ok {
		checks := v.checkBucketsExist()
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["rls"]; ok {
		checks := v.checkRLSEnabled()
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["secrets"]; ok {
		checks := v.checkSecretsExist()
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["auth_config"]; ok {
		checks := v.checkAuthConfig()
		result.Checks = append(result.Checks, checks...)
	}

	// Calculate summary
	for _, check := range result.Checks {
		result.Summary.Total++
		if check.Passed {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}

	// Set final status
	if result.Summary.Failed > 0 {
		result.Status = VerifyFailed
	} else {
		result.Status = VerifyPassed
	}

	return result, nil
}

func (v *basicVerifier) checkTablesExist() []verificationCheckResult {
	var results []verificationCheckResult

	// Get tables from sblite _columns
	sbliteTables, err := v.getSbliteTables()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "tables_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite tables: %v", err),
		})
		return results
	}

	if len(sbliteTables) == 0 {
		results = append(results, verificationCheckResult{
			Name:    "tables_exist",
			Passed:  true,
			Message: "No tables to verify",
		})
		return results
	}

	// Get tables from Supabase
	supabaseTables, err := v.getSupabaseTables()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "tables_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase tables: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseTableSet := make(map[string]bool)
	for _, t := range supabaseTables {
		supabaseTableSet[t] = true
	}

	// Check each sblite table exists in Supabase
	var missing []string
	var found []string
	for _, table := range sbliteTables {
		if supabaseTableSet[table] {
			found = append(found, table)
		} else {
			missing = append(missing, table)
		}
	}

	if len(missing) > 0 {
		results = append(results, verificationCheckResult{
			Name:    "tables_exist",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d tables missing in Supabase", len(missing), len(sbliteTables)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else {
		results = append(results, verificationCheckResult{
			Name:    "tables_exist",
			Passed:  true,
			Message: fmt.Sprintf("All %d tables exist in Supabase", len(sbliteTables)),
			Details: map[string]interface{}{
				"tables": found,
			},
		})
	}

	return results
}

func (v *basicVerifier) checkColumnsMatch(dataItems []*verificationItem) []verificationCheckResult {
	var results []verificationCheckResult

	for _, item := range dataItems {
		tableName := item.ItemName

		// Get column count from sblite
		sbliteCount, err := v.getSbliteColumnCount(tableName)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get sblite columns for %s: %v", tableName, err),
			})
			continue
		}

		// Get column count from Supabase
		supabaseCount, err := v.getSupabaseColumnCount(tableName)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get Supabase columns for %s: %v", tableName, err),
			})
			continue
		}

		if sbliteCount == supabaseCount {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  true,
				Message: fmt.Sprintf("Table %s has %d columns (matching)", tableName, sbliteCount),
			})
		} else {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Table %s column count mismatch: sblite=%d, Supabase=%d", tableName, sbliteCount, supabaseCount),
				Details: map[string]interface{}{
					"sblite_columns":   sbliteCount,
					"supabase_columns": supabaseCount,
				},
			})
		}
	}

	return results
}

func (v *basicVerifier) checkFunctionsDeployed(funcItems []*verificationItem) []verificationCheckResult {
	var results []verificationCheckResult

	// Get functions from Supabase
	functions, err := v.supabaseClient.ListFunctions(v.migration.SupabaseProjectRef)
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "functions_deployed",
			Passed:  false,
			Message: fmt.Sprintf("Failed to list Supabase functions: %v", err),
		})
		return results
	}

	// Build lookup set
	functionSet := make(map[string]functionInfo)
	for _, f := range functions {
		functionSet[f.Slug] = f
	}

	// Check each expected function
	var missing []string
	var found []string
	for _, item := range funcItems {
		funcName := item.ItemName
		if _, ok := functionSet[funcName]; ok {
			found = append(found, funcName)
		} else {
			missing = append(missing, funcName)
		}
	}

	if len(missing) > 0 {
		results = append(results, verificationCheckResult{
			Name:    "functions_deployed",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d functions missing in Supabase", len(missing), len(funcItems)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else if len(found) > 0 {
		results = append(results, verificationCheckResult{
			Name:    "functions_deployed",
			Passed:  true,
			Message: fmt.Sprintf("All %d functions deployed in Supabase", len(found)),
			Details: map[string]interface{}{
				"functions": found,
			},
		})
	}

	return results
}

func (v *basicVerifier) checkBucketsExist() []verificationCheckResult {
	var results []verificationCheckResult

	// Get buckets from sblite
	sbliteBuckets, err := v.getSbliteBuckets()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "buckets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite buckets: %v", err),
		})
		return results
	}

	if len(sbliteBuckets) == 0 {
		results = append(results, verificationCheckResult{
			Name:    "buckets_exist",
			Passed:  true,
			Message: "No buckets to verify",
		})
		return results
	}

	// Get buckets from Supabase
	supabaseBuckets, err := v.getSupabaseBuckets()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "buckets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase buckets: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseBucketSet := make(map[string]bool)
	for _, b := range supabaseBuckets {
		supabaseBucketSet[b] = true
	}

	// Check each sblite bucket exists in Supabase
	var missing []string
	var found []string
	for _, bucket := range sbliteBuckets {
		if supabaseBucketSet[bucket] {
			found = append(found, bucket)
		} else {
			missing = append(missing, bucket)
		}
	}

	if len(missing) > 0 {
		results = append(results, verificationCheckResult{
			Name:    "buckets_exist",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d buckets missing in Supabase", len(missing), len(sbliteBuckets)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else {
		results = append(results, verificationCheckResult{
			Name:    "buckets_exist",
			Passed:  true,
			Message: fmt.Sprintf("All %d buckets exist in Supabase", len(sbliteBuckets)),
			Details: map[string]interface{}{
				"buckets": found,
			},
		})
	}

	return results
}

func (v *basicVerifier) checkRLSEnabled() []verificationCheckResult {
	var results []verificationCheckResult

	// Get tables with RLS from sblite
	sbliteRLSTables, err := v.getSbliteRLSTables()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "rls_enabled",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite RLS tables: %v", err),
		})
		return results
	}

	if len(sbliteRLSTables) == 0 {
		results = append(results, verificationCheckResult{
			Name:    "rls_enabled",
			Passed:  true,
			Message: "No RLS tables to verify",
		})
		return results
	}

	// Get RLS-enabled tables from Supabase
	supabaseRLSTables, err := v.getSupabaseRLSTables()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "rls_enabled",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase RLS tables: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseRLSSet := make(map[string]bool)
	for _, t := range supabaseRLSTables {
		supabaseRLSSet[t] = true
	}

	// Check each sblite RLS table has RLS enabled in Supabase
	var missingRLS []string
	var hasRLS []string
	for _, table := range sbliteRLSTables {
		if supabaseRLSSet[table] {
			hasRLS = append(hasRLS, table)
		} else {
			missingRLS = append(missingRLS, table)
		}
	}

	if len(missingRLS) > 0 {
		results = append(results, verificationCheckResult{
			Name:    "rls_enabled",
			Passed:  false,
			Message: fmt.Sprintf("RLS not enabled on %d of %d tables", len(missingRLS), len(sbliteRLSTables)),
			Details: map[string]interface{}{
				"missing_rls": missingRLS,
				"has_rls":     hasRLS,
			},
		})
	} else {
		results = append(results, verificationCheckResult{
			Name:    "rls_enabled",
			Passed:  true,
			Message: fmt.Sprintf("RLS enabled on all %d expected tables", len(sbliteRLSTables)),
			Details: map[string]interface{}{
				"tables": hasRLS,
			},
		})
	}

	return results
}

func (v *basicVerifier) checkSecretsExist() []verificationCheckResult {
	var results []verificationCheckResult

	// Get secret names from sblite
	sbliteSecrets, err := v.getSbliteSecretNames()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "secrets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite secrets: %v", err),
		})
		return results
	}

	if len(sbliteSecrets) == 0 {
		results = append(results, verificationCheckResult{
			Name:    "secrets_exist",
			Passed:  true,
			Message: "No secrets to verify",
		})
		return results
	}

	// Get secret names from Supabase
	supabaseSecrets, err := v.supabaseClient.ListSecrets(v.migration.SupabaseProjectRef)
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "secrets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase secrets: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseSecretSet := make(map[string]bool)
	for _, s := range supabaseSecrets {
		supabaseSecretSet[s.Name] = true
	}

	// Check each sblite secret exists in Supabase
	var missing []string
	var found []string
	for _, secret := range sbliteSecrets {
		if supabaseSecretSet[secret] {
			found = append(found, secret)
		} else {
			missing = append(missing, secret)
		}
	}

	if len(missing) > 0 {
		results = append(results, verificationCheckResult{
			Name:    "secrets_exist",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d secrets missing in Supabase", len(missing), len(sbliteSecrets)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else {
		results = append(results, verificationCheckResult{
			Name:    "secrets_exist",
			Passed:  true,
			Message: fmt.Sprintf("All %d secrets exist in Supabase", len(sbliteSecrets)),
			Details: map[string]interface{}{
				"secrets": found,
			},
		})
	}

	return results
}

func (v *basicVerifier) checkAuthConfig() []verificationCheckResult {
	var results []verificationCheckResult

	// Get expected auth config from sblite
	var allowAnonymous string
	err := v.sbliteDB.QueryRow("SELECT value FROM _dashboard WHERE key = 'allow_anonymous'").Scan(&allowAnonymous)
	if err != nil && err != sql.ErrNoRows {
		results = append(results, verificationCheckResult{
			Name:    "auth_config",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite auth config: %v", err),
		})
		return results
	}

	// Get auth config from Supabase
	config, err := v.supabaseClient.GetAuthConfig(v.migration.SupabaseProjectRef)
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "auth_config",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase auth config: %v", err),
		})
		return results
	}

	// Check anonymous users setting
	if allowAnonymous == "true" {
		if val, ok := config["EXTERNAL_ANONYMOUS_USERS_ENABLED"].(bool); ok && val {
			results = append(results, verificationCheckResult{
				Name:    "auth_config_anonymous_users",
				Passed:  true,
				Message: "Anonymous users setting matches (enabled)",
			})
		} else {
			results = append(results, verificationCheckResult{
				Name:    "auth_config_anonymous_users",
				Passed:  false,
				Message: "Anonymous users should be enabled but is not",
				Details: map[string]interface{}{
					"expected": true,
					"actual":   config["EXTERNAL_ANONYMOUS_USERS_ENABLED"],
				},
			})
		}
	} else {
		results = append(results, verificationCheckResult{
			Name:    "auth_config",
			Passed:  true,
			Message: "Auth configuration verified",
		})
	}

	return results
}

// Helper methods for querying sblite

func (v *basicVerifier) getSbliteTables() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT DISTINCT table_name FROM _columns ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (v *basicVerifier) getSbliteColumnCount(tableName string) (int, error) {
	var count int
	err := v.sbliteDB.QueryRow("SELECT COUNT(*) FROM _columns WHERE table_name = ?", tableName).Scan(&count)
	return count, err
}

func (v *basicVerifier) getSbliteBuckets() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT id FROM storage_buckets ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		buckets = append(buckets, id)
	}
	return buckets, rows.Err()
}

func (v *basicVerifier) getSbliteRLSTables() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT DISTINCT table_name FROM _rls_policies WHERE enabled = 1 ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (v *basicVerifier) getSbliteSecretNames() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT name FROM _functions_secrets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		secrets = append(secrets, name)
	}
	return secrets, rows.Err()
}

// Helper methods for querying Supabase PostgreSQL

func (v *basicVerifier) getSupabaseTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := v.supabaseDB.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (v *basicVerifier) getSupabaseColumnCount(tableName string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	err := v.supabaseDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
	`, tableName).Scan(&count)
	return count, err
}

func (v *basicVerifier) getSupabaseBuckets() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := v.supabaseDB.QueryContext(ctx, `
		SELECT id FROM storage.buckets ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		buckets = append(buckets, id)
	}
	return buckets, rows.Err()
}

func (v *basicVerifier) getSupabaseRLSTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := v.supabaseDB.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		AND rowsecurity = true
		ORDER BY tablename
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// RunIntegrityVerification runs data integrity verification checks for a migration.
// It creates a verification record, executes the checks, and stores the results.
func (s *Service) RunIntegrityVerification(migrationID string) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return fmt.Errorf("get migration: %w", err)
	}

	// Verify migration has a project selected
	if m.SupabaseProjectRef == "" {
		return fmt.Errorf("no Supabase project selected for migration")
	}

	// Create verification record
	verification, err := s.state.CreateVerification(migrationID, LayerIntegrity)
	if err != nil {
		return fmt.Errorf("create verification: %w", err)
	}

	// Mark as running
	now := time.Now().UTC()
	verification.Status = VerifyRunning
	verification.StartedAt = &now
	if err := s.state.UpdateVerification(verification); err != nil {
		return fmt.Errorf("update verification status: %w", err)
	}

	// Get migration items
	items, err := s.state.GetItems(migrationID)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get items: %w", err))
		return fmt.Errorf("get items: %w", err)
	}

	// Get Postgres connection
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get postgres connection: %w", err))
		return fmt.Errorf("get postgres connection: %w", err)
	}
	defer pgDB.Close()

	// Convert items to verification format
	verifyItems := make([]*verificationItem, len(items))
	for i, item := range items {
		verifyItems[i] = &verificationItem{
			ID:          item.ID,
			MigrationID: item.MigrationID,
			ItemType:    string(item.ItemType),
			ItemName:    item.ItemName,
			Status:      string(item.Status),
			Metadata:    item.Metadata,
		}
	}

	// Create verifier
	verifier := newIntegrityVerifier(s.db, pgDB, m, verifyItems)

	// Run checks
	result, err := verifier.runIntegrityChecks()
	if err != nil {
		s.markVerificationFailed(verification, err)
		return fmt.Errorf("run integrity checks: %w", err)
	}

	// Store results
	resultsJSON, err := json.Marshal(result)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("marshal results: %w", err))
		return fmt.Errorf("marshal results: %w", err)
	}

	completedAt := time.Now().UTC()
	verification.CompletedAt = &completedAt
	verification.Results = resultsJSON

	if result.Summary.Failed > 0 {
		verification.Status = VerifyFailed
	} else {
		verification.Status = VerifyPassed
	}

	if err := s.state.UpdateVerification(verification); err != nil {
		return fmt.Errorf("update verification: %w", err)
	}

	return nil
}

// integrityVerifier performs data integrity verification checks after migration.
type integrityVerifier struct {
	sbliteDB   *sql.DB
	supabaseDB *sql.DB
	migration  *Migration
	items      []*verificationItem
}

func newIntegrityVerifier(
	sbliteDB *sql.DB,
	supabaseDB *sql.DB,
	migration *Migration,
	items []*verificationItem,
) *integrityVerifier {
	return &integrityVerifier{
		sbliteDB:   sbliteDB,
		supabaseDB: supabaseDB,
		migration:  migration,
		items:      items,
	}
}

func (v *integrityVerifier) runIntegrityChecks() (*verificationResult, error) {
	result := &verificationResult{
		Layer:  LayerIntegrity,
		Status: VerifyRunning,
		Checks: []verificationCheckResult{},
	}

	// Group items by type for efficient checking
	itemsByType := make(map[string][]*verificationItem)
	for _, item := range v.items {
		if item.Status == "completed" {
			itemsByType[item.ItemType] = append(itemsByType[item.ItemType], item)
		}
	}

	// Run row count checks for data items
	if dataItems, ok := itemsByType["data"]; ok {
		checks := v.checkRowCounts(dataItems)
		result.Checks = append(result.Checks, checks...)

		// Run sample row comparison
		sampleChecks := v.checkSampleRows(dataItems)
		result.Checks = append(result.Checks, sampleChecks...)

		// Run foreign key integrity checks
		fkChecks := v.checkForeignKeyIntegrity()
		result.Checks = append(result.Checks, fkChecks...)
	}

	// Check storage file counts
	if _, ok := itemsByType["storage_files"]; ok {
		checks := v.checkStorageFileCounts()
		result.Checks = append(result.Checks, checks...)
	}

	// Check user count
	if _, ok := itemsByType["users"]; ok {
		checks := v.checkUserCount()
		result.Checks = append(result.Checks, checks...)
	}

	// Calculate summary
	for _, check := range result.Checks {
		result.Summary.Total++
		if check.Passed {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}

	// Set final status
	if result.Summary.Failed > 0 {
		result.Status = VerifyFailed
	} else {
		result.Status = VerifyPassed
	}

	return result, nil
}

// checkRowCounts compares row counts for all migrated tables.
func (v *integrityVerifier) checkRowCounts(dataItems []*verificationItem) []verificationCheckResult {
	var results []verificationCheckResult

	for _, item := range dataItems {
		tableName := item.ItemName

		// Get row count from sblite
		sbliteCount, err := v.getSbliteRowCount(tableName)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("row_count_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get sblite row count for %s: %v", tableName, err),
			})
			continue
		}

		// Get row count from Supabase
		supabaseCount, err := v.getSupabaseRowCount(tableName)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("row_count_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get Supabase row count for %s: %v", tableName, err),
			})
			continue
		}

		if sbliteCount == supabaseCount {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("row_count_%s", tableName),
				Passed:  true,
				Message: fmt.Sprintf("Table %s has %d rows (matching)", tableName, sbliteCount),
				Details: map[string]interface{}{
					"count": sbliteCount,
				},
			})
		} else {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("row_count_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Table %s row count mismatch: sblite=%d, Supabase=%d", tableName, sbliteCount, supabaseCount),
				Details: map[string]interface{}{
					"sblite_count":   sbliteCount,
					"supabase_count": supabaseCount,
					"difference":     sbliteCount - supabaseCount,
				},
			})
		}
	}

	return results
}

// checkSampleRows compares first 10, last 10, and random 10 rows per table.
func (v *integrityVerifier) checkSampleRows(dataItems []*verificationItem) []verificationCheckResult {
	var results []verificationCheckResult

	for _, item := range dataItems {
		tableName := item.ItemName

		// Get primary key or first column for ordering
		orderColumn, err := v.getOrderColumn(tableName)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("sample_rows_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to determine order column for %s: %v", tableName, err),
			})
			continue
		}

		// Compare first 10 rows
		firstResult := v.compareSampleRows(tableName, orderColumn, "first", 10)
		results = append(results, firstResult)

		// Compare last 10 rows
		lastResult := v.compareSampleRows(tableName, orderColumn, "last", 10)
		results = append(results, lastResult)

		// Compare random 10 rows
		randomResult := v.compareRandomRows(tableName, 10)
		results = append(results, randomResult)
	}

	return results
}

// compareSampleRows compares a sample of rows between sblite and Supabase.
func (v *integrityVerifier) compareSampleRows(tableName, orderColumn, position string, limit int) verificationCheckResult {
	checkName := fmt.Sprintf("sample_%s_%s", position, tableName)

	// Build order direction
	orderDir := "ASC"
	if position == "last" {
		orderDir = "DESC"
	}

	// Get rows from sblite
	sbliteRows, err := v.getSampleRows(v.sbliteDB, tableName, orderColumn, orderDir, limit, false)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite %s rows for %s: %v", position, tableName, err),
		}
	}

	// Get rows from Supabase
	supabaseRows, err := v.getSampleRows(v.supabaseDB, tableName, orderColumn, orderDir, limit, true)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase %s rows for %s: %v", position, tableName, err),
		}
	}

	// Compare rows
	mismatches := v.compareRowSets(sbliteRows, supabaseRows)

	if len(mismatches) == 0 {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  true,
			Message: fmt.Sprintf("Table %s %s %d rows match", tableName, position, len(sbliteRows)),
			Details: map[string]interface{}{
				"rows_compared": len(sbliteRows),
			},
		}
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  false,
		Message: fmt.Sprintf("Table %s %s rows have %d mismatches", tableName, position, len(mismatches)),
		Details: map[string]interface{}{
			"rows_compared": len(sbliteRows),
			"mismatches":    mismatches,
		},
	}
}

// compareRandomRows compares random rows between sblite and Supabase.
func (v *integrityVerifier) compareRandomRows(tableName string, limit int) verificationCheckResult {
	checkName := fmt.Sprintf("sample_random_%s", tableName)

	// Get random rows from sblite using RANDOM()
	sbliteRows, err := v.getRandomRows(v.sbliteDB, tableName, limit, false)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite random rows for %s: %v", tableName, err),
		}
	}

	if len(sbliteRows) == 0 {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  true,
			Message: fmt.Sprintf("Table %s has no rows to compare", tableName),
		}
	}

	// Get the same rows from Supabase by their primary key values
	// First, we need to find matching rows
	orderColumn, err := v.getOrderColumn(tableName)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to get order column for %s: %v", tableName, err),
		}
	}

	// Extract the order column values from sblite rows
	var keyValues []interface{}
	for _, row := range sbliteRows {
		if val, ok := row[orderColumn]; ok {
			keyValues = append(keyValues, val)
		}
	}

	// Get matching rows from Supabase
	supabaseRows, err := v.getRowsByKeys(v.supabaseDB, tableName, orderColumn, keyValues, true)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase matching rows for %s: %v", tableName, err),
		}
	}

	// Compare rows
	mismatches := v.compareRowSets(sbliteRows, supabaseRows)

	if len(mismatches) == 0 {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  true,
			Message: fmt.Sprintf("Table %s random %d rows match", tableName, len(sbliteRows)),
			Details: map[string]interface{}{
				"rows_compared": len(sbliteRows),
			},
		}
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  false,
		Message: fmt.Sprintf("Table %s random rows have %d mismatches", tableName, len(mismatches)),
		Details: map[string]interface{}{
			"rows_compared": len(sbliteRows),
			"mismatches":    mismatches,
		},
	}
}

// checkStorageFileCounts compares object counts per bucket.
func (v *integrityVerifier) checkStorageFileCounts() []verificationCheckResult {
	var results []verificationCheckResult

	// Get buckets from sblite
	buckets, err := v.getSbliteBuckets()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "storage_file_counts",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite buckets: %v", err),
		})
		return results
	}

	if len(buckets) == 0 {
		results = append(results, verificationCheckResult{
			Name:    "storage_file_counts",
			Passed:  true,
			Message: "No storage buckets to verify",
		})
		return results
	}

	for _, bucket := range buckets {
		// Get object count from sblite
		sbliteCount, err := v.getSbliteObjectCount(bucket)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("storage_count_%s", bucket),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get sblite object count for bucket %s: %v", bucket, err),
			})
			continue
		}

		// Get object count from Supabase
		supabaseCount, err := v.getSupabaseObjectCount(bucket)
		if err != nil {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("storage_count_%s", bucket),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get Supabase object count for bucket %s: %v", bucket, err),
			})
			continue
		}

		if sbliteCount == supabaseCount {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("storage_count_%s", bucket),
				Passed:  true,
				Message: fmt.Sprintf("Bucket %s has %d objects (matching)", bucket, sbliteCount),
				Details: map[string]interface{}{
					"count": sbliteCount,
				},
			})
		} else {
			results = append(results, verificationCheckResult{
				Name:    fmt.Sprintf("storage_count_%s", bucket),
				Passed:  false,
				Message: fmt.Sprintf("Bucket %s object count mismatch: sblite=%d, Supabase=%d", bucket, sbliteCount, supabaseCount),
				Details: map[string]interface{}{
					"sblite_count":   sbliteCount,
					"supabase_count": supabaseCount,
					"difference":     sbliteCount - supabaseCount,
				},
			})
		}
	}

	return results
}

// checkUserCount compares auth.users count between sblite and Supabase.
func (v *integrityVerifier) checkUserCount() []verificationCheckResult {
	var results []verificationCheckResult

	// Get user count from sblite
	sbliteCount, err := v.getSbliteUserCount()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "user_count",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite user count: %v", err),
		})
		return results
	}

	// Get user count from Supabase
	supabaseCount, err := v.getSupabaseUserCount()
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "user_count",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase user count: %v", err),
		})
		return results
	}

	if sbliteCount == supabaseCount {
		results = append(results, verificationCheckResult{
			Name:    "user_count",
			Passed:  true,
			Message: fmt.Sprintf("User count matches: %d users", sbliteCount),
			Details: map[string]interface{}{
				"count": sbliteCount,
			},
		})
	} else {
		results = append(results, verificationCheckResult{
			Name:    "user_count",
			Passed:  false,
			Message: fmt.Sprintf("User count mismatch: sblite=%d, Supabase=%d", sbliteCount, supabaseCount),
			Details: map[string]interface{}{
				"sblite_count":   sbliteCount,
				"supabase_count": supabaseCount,
				"difference":     sbliteCount - supabaseCount,
			},
		})
	}

	return results
}

// checkForeignKeyIntegrity validates that all foreign key relationships are intact after migration.
func (v *integrityVerifier) checkForeignKeyIntegrity() []verificationCheckResult {
	var results []verificationCheckResult

	// Query foreign key constraints from Supabase information_schema
	query := `
		SELECT
			tc.constraint_name,
			tc.table_name AS from_table,
			kcu.column_name AS from_column,
			ccu.table_name AS to_table,
			ccu.column_name AS to_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = 'public'
	`

	rows, err := v.supabaseDB.Query(query)
	if err != nil {
		results = append(results, verificationCheckResult{
			Name:    "foreign_key_integrity",
			Passed:  false,
			Message: fmt.Sprintf("Failed to query foreign key constraints: %v", err),
		})
		return results
	}
	defer rows.Close()

	// Collect all foreign key constraints
	type fkConstraint struct {
		constraintName string
		fromTable      string
		fromColumn     string
		toTable        string
		toColumn       string
	}
	var constraints []fkConstraint

	for rows.Next() {
		var fk fkConstraint
		if err := rows.Scan(&fk.constraintName, &fk.fromTable, &fk.fromColumn, &fk.toTable, &fk.toColumn); err != nil {
			results = append(results, verificationCheckResult{
				Name:    "foreign_key_integrity",
				Passed:  false,
				Message: fmt.Sprintf("Failed to scan foreign key constraint: %v", err),
			})
			return results
		}
		constraints = append(constraints, fk)
	}

	if err := rows.Err(); err != nil {
		results = append(results, verificationCheckResult{
			Name:    "foreign_key_integrity",
			Passed:  false,
			Message: fmt.Sprintf("Error iterating foreign key constraints: %v", err),
		})
		return results
	}

	if len(constraints) == 0 {
		results = append(results, verificationCheckResult{
			Name:    "foreign_key_integrity",
			Passed:  true,
			Message: "No foreign key constraints found to verify",
		})
		return results
	}

	// Verify each foreign key constraint
	for _, fk := range constraints {
		checkResult := v.verifyForeignKeyConstraint(fk.constraintName, fk.fromTable, fk.fromColumn, fk.toTable, fk.toColumn)
		results = append(results, checkResult)
	}

	return results
}

// verifyForeignKeyConstraint checks that all values in from_column exist in to_column.
func (v *integrityVerifier) verifyForeignKeyConstraint(constraintName, fromTable, fromColumn, toTable, toColumn string) verificationCheckResult {
	checkName := fmt.Sprintf("fk_%s", constraintName)

	// Quote identifiers safely
	quotedFromTable, err := quoteIdentifier(fromTable)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Invalid from_table name: %s", fromTable),
		}
	}
	quotedFromColumn, err := quoteIdentifier(fromColumn)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Invalid from_column name: %s", fromColumn),
		}
	}
	quotedToTable, err := quoteIdentifier(toTable)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Invalid to_table name: %s", toTable),
		}
	}
	quotedToColumn, err := quoteIdentifier(toColumn)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Invalid to_column name: %s", toColumn),
		}
	}

	// Query to find orphaned references (values in from_column that don't exist in to_column)
	// We check in Supabase since that's where the FK constraints actually exist
	orphanQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s f
		WHERE f.%s IS NOT NULL
		AND NOT EXISTS (
			SELECT 1 FROM %s t WHERE t.%s = f.%s
		)
	`, quotedFromTable, quotedFromColumn, quotedToTable, quotedToColumn, quotedFromColumn)

	var orphanCount int64
	err = v.supabaseDB.QueryRow(orphanQuery).Scan(&orphanCount)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to check foreign key %s: %v", constraintName, err),
			Details: map[string]interface{}{
				"constraint":  constraintName,
				"from_table":  fromTable,
				"from_column": fromColumn,
				"to_table":    toTable,
				"to_column":   toColumn,
			},
		}
	}

	// Get total count of non-null foreign key references for context
	totalQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s IS NOT NULL`, quotedFromTable, quotedFromColumn)
	var totalCount int64
	if err := v.supabaseDB.QueryRow(totalQuery).Scan(&totalCount); err != nil {
		totalCount = -1 // Unknown
	}

	if orphanCount == 0 {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  true,
			Message: fmt.Sprintf("FK %s: %s.%s -> %s.%s (all %d references valid)", constraintName, fromTable, fromColumn, toTable, toColumn, totalCount),
			Details: map[string]interface{}{
				"constraint":      constraintName,
				"from_table":      fromTable,
				"from_column":     fromColumn,
				"to_table":        toTable,
				"to_column":       toColumn,
				"total_references": totalCount,
				"orphaned_count":  0,
			},
		}
	}

	// Get sample of orphaned values for debugging
	sampleQuery := fmt.Sprintf(`
		SELECT DISTINCT f.%s
		FROM %s f
		WHERE f.%s IS NOT NULL
		AND NOT EXISTS (
			SELECT 1 FROM %s t WHERE t.%s = f.%s
		)
		LIMIT 5
	`, quotedFromColumn, quotedFromTable, quotedFromColumn, quotedToTable, quotedToColumn, quotedFromColumn)

	sampleRows, err := v.supabaseDB.Query(sampleQuery)
	var orphanedSamples []interface{}
	if err == nil {
		defer sampleRows.Close()
		for sampleRows.Next() {
			var val interface{}
			if err := sampleRows.Scan(&val); err == nil {
				orphanedSamples = append(orphanedSamples, val)
			}
		}
		// Check for iteration errors (non-critical for verification result)
		_ = sampleRows.Err()
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  false,
		Message: fmt.Sprintf("FK %s: %d orphaned references in %s.%s (missing from %s.%s)", constraintName, orphanCount, fromTable, fromColumn, toTable, toColumn),
		Details: map[string]interface{}{
			"constraint":       constraintName,
			"from_table":       fromTable,
			"from_column":      fromColumn,
			"to_table":         toTable,
			"to_column":        toColumn,
			"total_references": totalCount,
			"orphaned_count":   orphanCount,
			"orphaned_samples": orphanedSamples,
		},
	}
}

// Helper methods for integrityVerifier

func (v *integrityVerifier) getSbliteRowCount(tableName string) (int64, error) {
	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		return 0, err
	}

	var count int64
	err = v.sbliteDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quotedTable)).Scan(&count)
	return count, err
}

func (v *integrityVerifier) getSupabaseRowCount(tableName string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		return 0, err
	}

	var count int64
	err = v.supabaseDB.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM public.%s", quotedTable)).Scan(&count)
	return count, err
}

func (v *integrityVerifier) getOrderColumn(tableName string) (string, error) {
	// Try to find primary key column from _columns metadata
	var columnName string
	err := v.sbliteDB.QueryRow(`
		SELECT column_name FROM _columns
		WHERE table_name = ? AND is_primary_key = 1
		ORDER BY ordinal_position LIMIT 1
	`, tableName).Scan(&columnName)

	if err == nil {
		return columnName, nil
	}

	// Fallback: get first column
	err = v.sbliteDB.QueryRow(`
		SELECT column_name FROM _columns
		WHERE table_name = ?
		ORDER BY ordinal_position LIMIT 1
	`, tableName).Scan(&columnName)

	if err == nil {
		return columnName, nil
	}

	// Final fallback: use SQLite pragma
	rows, err := v.sbliteDB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return "", err
		}
		return name, nil
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error reading table_info: %w", err)
	}

	return "", fmt.Errorf("no columns found for table %s", tableName)
}

func (v *integrityVerifier) getSampleRows(db *sql.DB, tableName, orderColumn, orderDir string, limit int, isPostgres bool) ([]map[string]interface{}, error) {
	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		return nil, err
	}
	quotedColumn, err := quoteIdentifier(orderColumn)
	if err != nil {
		return nil, err
	}

	var query string
	if isPostgres {
		query = fmt.Sprintf("SELECT * FROM public.%s ORDER BY %s %s LIMIT %d", quotedTable, quotedColumn, orderDir, limit)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s ORDER BY %s %s LIMIT %d", quotedTable, quotedColumn, orderDir, limit)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (v *integrityVerifier) getRandomRows(db *sql.DB, tableName string, limit int, isPostgres bool) ([]map[string]interface{}, error) {
	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		return nil, err
	}

	var query string
	if isPostgres {
		query = fmt.Sprintf("SELECT * FROM public.%s ORDER BY RANDOM() LIMIT %d", quotedTable, limit)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s ORDER BY RANDOM() LIMIT %d", quotedTable, limit)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (v *integrityVerifier) getRowsByKeys(db *sql.DB, tableName, keyColumn string, keyValues []interface{}, isPostgres bool) ([]map[string]interface{}, error) {
	if len(keyValues) == 0 {
		return nil, nil
	}

	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		return nil, err
	}
	quotedColumn, err := quoteIdentifier(keyColumn)
	if err != nil {
		return nil, err
	}

	// Build placeholders
	placeholders := make([]string, len(keyValues))
	for i := range keyValues {
		if isPostgres {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		} else {
			placeholders[i] = "?"
		}
	}

	var query string
	if isPostgres {
		query = fmt.Sprintf("SELECT * FROM public.%s WHERE %s IN (%s) ORDER BY %s",
			quotedTable, quotedColumn, strings.Join(placeholders, ","), quotedColumn)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s) ORDER BY %s",
			quotedTable, quotedColumn, strings.Join(placeholders, ","), quotedColumn)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, keyValues...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (v *integrityVerifier) getSbliteBuckets() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT id FROM storage_buckets ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		buckets = append(buckets, id)
	}
	return buckets, rows.Err()
}

func (v *integrityVerifier) getSbliteObjectCount(bucket string) (int64, error) {
	var count int64
	err := v.sbliteDB.QueryRow("SELECT COUNT(*) FROM storage_objects WHERE bucket_id = ?", bucket).Scan(&count)
	return count, err
}

func (v *integrityVerifier) getSupabaseObjectCount(bucket string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var count int64
	err := v.supabaseDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM storage.objects WHERE bucket_id = $1", bucket).Scan(&count)
	return count, err
}

func (v *integrityVerifier) getSbliteUserCount() (int64, error) {
	var count int64
	err := v.sbliteDB.QueryRow("SELECT COUNT(*) FROM auth_users").Scan(&count)
	return count, err
}

func (v *integrityVerifier) getSupabaseUserCount() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var count int64
	err := v.supabaseDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM auth.users").Scan(&count)
	return count, err
}

// scanRowsToMaps converts sql.Rows to a slice of maps.
func scanRowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}

	for rows.Next() {
		// Create slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Convert to map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for comparison
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		result = append(result, rowMap)
	}

	return result, rows.Err()
}

// compareRowSets compares two sets of rows and returns mismatches.
func (v *integrityVerifier) compareRowSets(sbliteRows, supabaseRows []map[string]interface{}) []map[string]interface{} {
	var mismatches []map[string]interface{}

	// Build lookup map from Supabase rows by JSON serialization
	supabaseMap := make(map[string]map[string]interface{})
	for _, row := range supabaseRows {
		key := rowToKey(row)
		supabaseMap[key] = row
	}

	// Check each sblite row
	for _, sbliteRow := range sbliteRows {
		key := rowToKey(sbliteRow)
		if _, found := supabaseMap[key]; !found {
			// Try to find a partial match for better reporting
			mismatch := map[string]interface{}{
				"type":       "missing_or_different",
				"sblite_row": sbliteRow,
			}

			// Look for matching row by comparing normalized values
			for _, supabaseRow := range supabaseRows {
				if rowsPartialMatch(sbliteRow, supabaseRow) {
					mismatch["type"] = "different"
					mismatch["supabase_row"] = supabaseRow
					mismatch["differences"] = findRowDifferences(sbliteRow, supabaseRow)
					break
				}
			}

			mismatches = append(mismatches, mismatch)
		}
	}

	return mismatches
}

// rowToKey creates a comparable key from a row map.
func rowToKey(row map[string]interface{}) string {
	// Normalize values for comparison
	normalized := make(map[string]interface{})
	for k, v := range row {
		normalized[k] = normalizeValue(v)
	}
	b, _ := json.Marshal(normalized)
	return string(b)
}

// normalizeValue normalizes a value for comparison between SQLite and PostgreSQL.
func normalizeValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []byte:
		return string(val)
	case int64:
		return val
	case float64:
		// Handle integer floats
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case bool:
		// SQLite stores booleans as 0/1
		if val {
			return int64(1)
		}
		return int64(0)
	case time.Time:
		return val.UTC().Format(time.RFC3339)
	case string:
		// Try to parse as time and normalize
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		if t, err := time.Parse("2006-01-02T15:04:05Z07:00", val); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		return val
	default:
		return v
	}
}

// rowsPartialMatch checks if two rows have the same primary key / first column.
func rowsPartialMatch(row1, row2 map[string]interface{}) bool {
	// Find first common column
	for k := range row1 {
		if v2, ok := row2[k]; ok {
			n1 := normalizeValue(row1[k])
			n2 := normalizeValue(v2)
			if fmt.Sprintf("%v", n1) == fmt.Sprintf("%v", n2) {
				return true
			}
		}
	}
	return false
}

// findRowDifferences returns the columns that differ between two rows.
func findRowDifferences(row1, row2 map[string]interface{}) []map[string]interface{} {
	var differences []map[string]interface{}

	for k, v1 := range row1 {
		v2, ok := row2[k]
		if !ok {
			differences = append(differences, map[string]interface{}{
				"column":        k,
				"sblite_value":  v1,
				"supabase_note": "column missing",
			})
			continue
		}

		n1 := normalizeValue(v1)
		n2 := normalizeValue(v2)

		if fmt.Sprintf("%v", n1) != fmt.Sprintf("%v", n2) {
			differences = append(differences, map[string]interface{}{
				"column":         k,
				"sblite_value":   v1,
				"supabase_value": v2,
			})
		}
	}

	return differences
}

// FunctionalTestOptions specifies what functional tests to run.
type FunctionalTestOptions struct {
	TestTableName    string `json:"test_table_name"`    // Table for SELECT test
	TestBucketID     string `json:"test_bucket_id"`     // Bucket for storage test
	TestFunctionName string `json:"test_function_name"` // Function to invoke
	TestAuthUser     bool   `json:"test_auth_user"`     // Whether to test user creation
}

// RunFunctionalVerification runs functional verification tests for a migration.
// It performs live tests against the Supabase project to verify functionality.
func (s *Service) RunFunctionalVerification(migrationID string, opts FunctionalTestOptions) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return fmt.Errorf("get migration: %w", err)
	}

	// Verify migration has a project selected
	if m.SupabaseProjectRef == "" {
		return fmt.Errorf("no Supabase project selected for migration")
	}

	// Create verification record
	verification, err := s.state.CreateVerification(migrationID, LayerFunctional)
	if err != nil {
		return fmt.Errorf("create verification: %w", err)
	}

	// Mark as running
	now := time.Now().UTC()
	verification.Status = VerifyRunning
	verification.StartedAt = &now
	if err := s.state.UpdateVerification(verification); err != nil {
		return fmt.Errorf("update verification status: %w", err)
	}

	// Get Supabase client for API access
	client, err := s.getSupabaseClient(migrationID)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get supabase client: %w", err))
		return fmt.Errorf("get supabase client: %w", err)
	}

	// Get API keys for storage and function tests
	apiKeys, err := client.GetAPIKeys(m.SupabaseProjectRef)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get api keys: %w", err))
		return fmt.Errorf("get api keys: %w", err)
	}

	// Find the required keys
	var serviceKey, anonKey string
	for _, key := range apiKeys {
		switch key.Name {
		case "service_role":
			serviceKey = key.APIKey
		case "anon":
			anonKey = key.APIKey
		}
	}

	if serviceKey == "" {
		s.markVerificationFailed(verification, fmt.Errorf("service_role key not found"))
		return fmt.Errorf("service_role key not found")
	}

	// Get Postgres connection for query test
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("get postgres connection: %w", err))
		return fmt.Errorf("get postgres connection: %w", err)
	}
	defer pgDB.Close()

	// Create verifier
	verifier := newFunctionalVerifier(pgDB, m.SupabaseProjectRef, serviceKey, anonKey, opts)

	// Run checks
	result, err := verifier.runFunctionalChecks()
	if err != nil {
		s.markVerificationFailed(verification, err)
		return fmt.Errorf("run functional checks: %w", err)
	}

	// Store results
	resultsJSON, err := json.Marshal(result)
	if err != nil {
		s.markVerificationFailed(verification, fmt.Errorf("marshal results: %w", err))
		return fmt.Errorf("marshal results: %w", err)
	}

	completedAt := time.Now().UTC()
	verification.CompletedAt = &completedAt
	verification.Results = resultsJSON

	if result.Summary.Failed > 0 {
		verification.Status = VerifyFailed
	} else {
		verification.Status = VerifyPassed
	}

	if err := s.state.UpdateVerification(verification); err != nil {
		return fmt.Errorf("update verification: %w", err)
	}

	return nil
}

// functionalVerifier performs functional verification tests against Supabase.
type functionalVerifier struct {
	supabaseDB  *sql.DB
	projectRef  string
	serviceKey  string
	anonKey     string
	opts        FunctionalTestOptions
	httpClient  *http.Client
}

func newFunctionalVerifier(
	supabaseDB *sql.DB,
	projectRef string,
	serviceKey string,
	anonKey string,
	opts FunctionalTestOptions,
) *functionalVerifier {
	return &functionalVerifier{
		supabaseDB: supabaseDB,
		projectRef: projectRef,
		serviceKey: serviceKey,
		anonKey:    anonKey,
		opts:       opts,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (v *functionalVerifier) runFunctionalChecks() (*verificationResult, error) {
	result := &verificationResult{
		Layer:  LayerFunctional,
		Status: VerifyRunning,
		Checks: []verificationCheckResult{},
	}

	// Run test query if table name provided
	if v.opts.TestTableName != "" {
		check := v.testQuery(v.opts.TestTableName)
		result.Checks = append(result.Checks, check)
	}

	// Run storage upload/download test if bucket ID provided
	if v.opts.TestBucketID != "" {
		check := v.testStorageUploadDownload(v.opts.TestBucketID)
		result.Checks = append(result.Checks, check)
	}

	// Run function invocation test if function name provided
	if v.opts.TestFunctionName != "" {
		check := v.testFunctionInvocation(v.opts.TestFunctionName)
		result.Checks = append(result.Checks, check)
	}

	// Run auth flow test if requested
	if v.opts.TestAuthUser {
		check := v.testAuthFlow()
		result.Checks = append(result.Checks, check)
	}

	// Calculate summary
	for _, check := range result.Checks {
		result.Summary.Total++
		if check.Passed {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}

	// Set final status
	if result.Summary.Failed > 0 {
		result.Status = VerifyFailed
	} else {
		result.Status = VerifyPassed
	}

	return result, nil
}

// testQuery executes a simple SELECT query against the specified table.
func (v *functionalVerifier) testQuery(tableName string) verificationCheckResult {
	checkName := fmt.Sprintf("query_test_%s", tableName)

	// Validate and quote the table name
	quotedTable, err := quoteIdentifier(tableName)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Invalid table name: %s", tableName),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute a simple SELECT query
	query := fmt.Sprintf("SELECT * FROM public.%s LIMIT 1", quotedTable)
	rows, err := v.supabaseDB.QueryContext(ctx, query)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Query failed on table %s: %v", tableName, err),
		}
	}
	defer rows.Close()

	// Get column names to verify table structure
	columns, err := rows.Columns()
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to get columns from %s: %v", tableName, err),
		}
	}

	// Count rows returned
	rowCount := 0
	for rows.Next() {
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Error iterating results from %s: %v", tableName, err),
		}
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  true,
		Message: fmt.Sprintf("Query executed successfully on table %s (%d columns, %d rows returned)", tableName, len(columns), rowCount),
		Details: map[string]interface{}{
			"table":        tableName,
			"column_count": len(columns),
			"row_count":    rowCount,
		},
	}
}

// testStorageUploadDownload tests storage API by uploading, downloading, verifying, and deleting a test file.
func (v *functionalVerifier) testStorageUploadDownload(bucketID string) verificationCheckResult {
	checkName := fmt.Sprintf("storage_test_%s", bucketID)

	// Generate unique test file name and content
	timestamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("test-verify-%d.txt", timestamp)
	fileContent := fmt.Sprintf("verification-test-%d", timestamp)
	filePath := fileName

	baseURL := fmt.Sprintf("https://%s.supabase.co/storage/v1", v.projectRef)

	// Step 1: Upload test file
	uploadURL := fmt.Sprintf("%s/object/%s/%s", baseURL, bucketID, filePath)
	uploadReq, err := http.NewRequest(http.MethodPost, uploadURL, strings.NewReader(fileContent))
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to create upload request: %v", err),
		}
	}
	uploadReq.Header.Set("Authorization", "Bearer "+v.serviceKey)
	uploadReq.Header.Set("Content-Type", "text/plain")

	uploadResp, err := v.httpClient.Do(uploadReq)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Upload request failed: %v", err),
		}
	}
	uploadBody, err := io.ReadAll(uploadResp.Body)
	uploadResp.Body.Close()
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to read upload response: %v", err),
		}
	}

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Upload failed with status %d: %s", uploadResp.StatusCode, string(uploadBody)),
			Details: map[string]interface{}{
				"bucket":    bucketID,
				"file":      filePath,
				"operation": "upload",
			},
		}
	}

	// Step 2: Download and verify the file
	downloadURL := fmt.Sprintf("%s/object/%s/%s", baseURL, bucketID, filePath)
	downloadReq, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		// Try to clean up
		v.deleteStorageObject(bucketID, filePath)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to create download request: %v", err),
		}
	}
	downloadReq.Header.Set("Authorization", "Bearer "+v.serviceKey)

	downloadResp, err := v.httpClient.Do(downloadReq)
	if err != nil {
		v.deleteStorageObject(bucketID, filePath)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Download request failed: %v", err),
		}
	}
	downloadedContent, err := io.ReadAll(downloadResp.Body)
	downloadResp.Body.Close()
	if err != nil {
		v.deleteStorageObject(bucketID, filePath)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to read download response: %v", err),
		}
	}

	if downloadResp.StatusCode != http.StatusOK {
		v.deleteStorageObject(bucketID, filePath)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Download failed with status %d", downloadResp.StatusCode),
			Details: map[string]interface{}{
				"bucket":    bucketID,
				"file":      filePath,
				"operation": "download",
			},
		}
	}

	// Step 3: Verify content matches
	if string(downloadedContent) != fileContent {
		v.deleteStorageObject(bucketID, filePath)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: "Downloaded content does not match uploaded content",
			Details: map[string]interface{}{
				"bucket":           bucketID,
				"file":             filePath,
				"expected_content": fileContent,
				"actual_content":   string(downloadedContent),
			},
		}
	}

	// Step 4: Delete the test file
	if err := v.deleteStorageObject(bucketID, filePath); err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to delete test file: %v", err),
			Details: map[string]interface{}{
				"bucket":    bucketID,
				"file":      filePath,
				"operation": "delete",
			},
		}
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  true,
		Message: fmt.Sprintf("Storage API test passed for bucket %s (upload, download, verify, delete)", bucketID),
		Details: map[string]interface{}{
			"bucket":        bucketID,
			"test_file":     filePath,
			"content_size":  len(fileContent),
			"content_match": true,
		},
	}
}

// deleteStorageObject deletes an object from Supabase storage.
func (v *functionalVerifier) deleteStorageObject(bucketID, path string) error {
	baseURL := fmt.Sprintf("https://%s.supabase.co/storage/v1", v.projectRef)
	deleteURL := fmt.Sprintf("%s/object/%s/%s", baseURL, bucketID, path)

	deleteReq, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+v.serviceKey)

	deleteResp, err := v.httpClient.Do(deleteReq)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK && deleteResp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete failed with status %d", deleteResp.StatusCode)
	}

	return nil
}

// testFunctionInvocation tests invoking an edge function with a simple payload.
func (v *functionalVerifier) testFunctionInvocation(funcName string) verificationCheckResult {
	checkName := fmt.Sprintf("function_test_%s", funcName)

	// Supabase edge function URL format
	funcURL := fmt.Sprintf("https://%s.supabase.co/functions/v1/%s", v.projectRef, funcName)

	// Create request with test payload
	payload := []byte(`{"test": true}`)
	req, err := http.NewRequest(http.MethodPost, funcURL, bytes.NewReader(payload))
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	req.Header.Set("Authorization", "Bearer "+v.anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Function invocation failed: %v", err),
		}
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to read function response: %v", err),
		}
	}

	// Accept any 2xx response as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  true,
			Message: fmt.Sprintf("Function %s invoked successfully (status %d)", funcName, resp.StatusCode),
			Details: map[string]interface{}{
				"function":    funcName,
				"status_code": resp.StatusCode,
				"response":    string(respBody),
			},
		}
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  false,
		Message: fmt.Sprintf("Function %s returned non-2xx status: %d", funcName, resp.StatusCode),
		Details: map[string]interface{}{
			"function":    funcName,
			"status_code": resp.StatusCode,
			"response":    string(respBody),
		},
	}
}

// testAuthFlow tests the auth API by creating a temp user, signing in, and deleting.
func (v *functionalVerifier) testAuthFlow() verificationCheckResult {
	checkName := "auth_test"

	// Generate unique test email
	timestamp := time.Now().UnixNano()
	testEmail := fmt.Sprintf("test-verify-%d@sblite-migration-test.local", timestamp)
	testPassword := fmt.Sprintf("testpass%d!", timestamp)

	baseURL := fmt.Sprintf("https://%s.supabase.co", v.projectRef)
	authURL := baseURL + "/auth/v1"

	// Step 1: Create test user via admin API
	signupPayload := map[string]string{
		"email":    testEmail,
		"password": testPassword,
	}
	signupBody, _ := json.Marshal(signupPayload)

	signupReq, err := http.NewRequest(http.MethodPost, authURL+"/admin/users", bytes.NewReader(signupBody))
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to create signup request: %v", err),
		}
	}
	signupReq.Header.Set("Authorization", "Bearer "+v.serviceKey)
	signupReq.Header.Set("Content-Type", "application/json")
	signupReq.Header.Set("apikey", v.serviceKey)

	signupResp, err := v.httpClient.Do(signupReq)
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Signup request failed: %v", err),
		}
	}
	signupRespBody, err := io.ReadAll(signupResp.Body)
	signupResp.Body.Close()
	if err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to read signup response: %v", err),
		}
	}

	if signupResp.StatusCode != http.StatusOK && signupResp.StatusCode != http.StatusCreated {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("User creation failed with status %d: %s", signupResp.StatusCode, string(signupRespBody)),
			Details: map[string]interface{}{
				"operation": "create_user",
			},
		}
	}

	// Parse user ID from response
	var signupResult struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(signupRespBody, &signupResult); err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to parse user creation response: %v", err),
		}
	}

	userID := signupResult.ID
	if userID == "" {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: "User creation did not return a user ID",
		}
	}

	// Step 2: Sign in with password grant
	tokenPayload := map[string]string{
		"email":    testEmail,
		"password": testPassword,
	}
	tokenBody, _ := json.Marshal(tokenPayload)

	tokenReq, err := http.NewRequest(http.MethodPost, authURL+"/token?grant_type=password", bytes.NewReader(tokenBody))
	if err != nil {
		v.deleteTestUser(userID)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to create token request: %v", err),
		}
	}
	tokenReq.Header.Set("Content-Type", "application/json")
	tokenReq.Header.Set("apikey", v.anonKey)

	tokenResp, err := v.httpClient.Do(tokenReq)
	if err != nil {
		v.deleteTestUser(userID)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Token request failed: %v", err),
		}
	}
	tokenRespBody, err := io.ReadAll(tokenResp.Body)
	tokenResp.Body.Close()
	if err != nil {
		v.deleteTestUser(userID)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to read token response: %v", err),
		}
	}

	if tokenResp.StatusCode != http.StatusOK {
		v.deleteTestUser(userID)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Sign in failed with status %d: %s", tokenResp.StatusCode, string(tokenRespBody)),
			Details: map[string]interface{}{
				"operation": "sign_in",
			},
		}
	}

	// Verify we got an access token
	var tokenResult struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(tokenRespBody, &tokenResult); err != nil || tokenResult.AccessToken == "" {
		v.deleteTestUser(userID)
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: "Sign in did not return an access token",
		}
	}

	// Step 3: Delete the test user
	if err := v.deleteTestUser(userID); err != nil {
		return verificationCheckResult{
			Name:    checkName,
			Passed:  false,
			Message: fmt.Sprintf("Failed to delete test user: %v", err),
			Details: map[string]interface{}{
				"operation": "delete_user",
				"user_id":   userID,
			},
		}
	}

	return verificationCheckResult{
		Name:    checkName,
		Passed:  true,
		Message: "Auth API test passed (create user, sign in, delete user)",
		Details: map[string]interface{}{
			"test_email":     testEmail,
			"operations":     []string{"create_user", "sign_in", "delete_user"},
			"all_successful": true,
		},
	}
}

// deleteTestUser deletes a test user via the Supabase admin API.
func (v *functionalVerifier) deleteTestUser(userID string) error {
	authURL := fmt.Sprintf("https://%s.supabase.co/auth/v1", v.projectRef)
	deleteURL := fmt.Sprintf("%s/admin/users/%s", authURL, userID)

	deleteReq, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+v.serviceKey)
	deleteReq.Header.Set("apikey", v.serviceKey)

	deleteResp, err := v.httpClient.Do(deleteReq)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK && deleteResp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete failed with status %d", deleteResp.StatusCode)
	}

	return nil
}
