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
