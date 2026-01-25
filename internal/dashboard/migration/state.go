// Package migration provides state management for Supabase migration tracking.
package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MigrationStatus represents the overall status of a migration.
type MigrationStatus string

const (
	StatusPending    MigrationStatus = "pending"
	StatusInProgress MigrationStatus = "in_progress"
	StatusCompleted  MigrationStatus = "completed"
	StatusFailed     MigrationStatus = "failed"
	StatusRolledBack MigrationStatus = "rolled_back"
)

// ItemStatus represents the status of a migration item.
type ItemStatus string

const (
	ItemPending    ItemStatus = "pending"
	ItemInProgress ItemStatus = "in_progress"
	ItemCompleted  ItemStatus = "completed"
	ItemFailed     ItemStatus = "failed"
	ItemSkipped    ItemStatus = "skipped"
	ItemRolledBack ItemStatus = "rolled_back"
)

// ItemType represents the type of migration item.
type ItemType string

const (
	ItemSchema         ItemType = "schema"
	ItemData           ItemType = "data"
	ItemUsers          ItemType = "users"
	ItemIdentities     ItemType = "identities"
	ItemRLS            ItemType = "rls"
	ItemStorageBuckets ItemType = "storage_buckets"
	ItemStorageFiles   ItemType = "storage_files"
	ItemFunctions      ItemType = "functions"
	ItemSecrets        ItemType = "secrets"
	ItemAuthConfig     ItemType = "auth_config"
	ItemOAuthConfig    ItemType = "oauth_config"
	ItemEmailTemplates ItemType = "email_templates"
)

// VerificationLayer represents the layer of verification.
type VerificationLayer string

const (
	LayerBasic      VerificationLayer = "basic"
	LayerIntegrity  VerificationLayer = "integrity"
	LayerFunctional VerificationLayer = "functional"
)

// VerificationStatus represents the status of a verification.
type VerificationStatus string

const (
	VerifyPending VerificationStatus = "pending"
	VerifyRunning VerificationStatus = "running"
	VerifyPassed  VerificationStatus = "passed"
	VerifyFailed  VerificationStatus = "failed"
)

// Migration represents a migration operation to Supabase.
type Migration struct {
	ID                   string          `json:"id"`
	Status               MigrationStatus `json:"status"`
	SupabaseProjectRef   string          `json:"supabase_project_ref"`
	SupabaseProjectName  string          `json:"supabase_project_name"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	CompletedAt          *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage         string          `json:"error_message,omitempty"`
	CredentialsEncrypted []byte          `json:"-"`
}

// MigrationItem represents a single item within a migration.
type MigrationItem struct {
	ID           string          `json:"id"`
	MigrationID  string          `json:"migration_id"`
	ItemType     ItemType        `json:"item_type"`
	ItemName     string          `json:"item_name"`
	Status       ItemStatus      `json:"status"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	RollbackInfo string          `json:"rollback_info,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// Verification represents a verification run for a migration.
type Verification struct {
	ID          string             `json:"id"`
	MigrationID string             `json:"migration_id"`
	Layer       VerificationLayer  `json:"layer"`
	Status      VerificationStatus `json:"status"`
	StartedAt   *time.Time         `json:"started_at,omitempty"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
	Results     json.RawMessage    `json:"results,omitempty"`
}

// StateStore manages migration state in the database.
type StateStore struct {
	db *sql.DB
}

// NewStateStore creates a new StateStore.
func NewStateStore(db *sql.DB) *StateStore {
	return &StateStore{db: db}
}

// CreateMigration creates a new migration with a UUID and pending status.
func (s *StateStore) CreateMigration() (*Migration, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO _migrations (id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, id, StatusPending, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("create migration: %w", err)
	}

	return &Migration{
		ID:        id,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetMigration retrieves a migration by ID. Returns nil, nil if not found.
func (s *StateStore) GetMigration(id string) (*Migration, error) {
	row := s.db.QueryRow(`
		SELECT id, status, supabase_project_ref, supabase_project_name,
		       created_at, updated_at, completed_at, error_message, credentials_encrypted
		FROM _migrations
		WHERE id = ?
	`, id)

	var m Migration
	var projectRef, projectName sql.NullString
	var completedAt sql.NullString
	var errorMsg sql.NullString
	var createdAtStr, updatedAtStr string
	var credentials []byte

	err := row.Scan(
		&m.ID, &m.Status, &projectRef, &projectName,
		&createdAtStr, &updatedAtStr, &completedAt, &errorMsg, &credentials,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get migration: %w", err)
	}

	m.SupabaseProjectRef = projectRef.String
	m.SupabaseProjectName = projectName.String
	m.ErrorMessage = errorMsg.String
	m.CredentialsEncrypted = credentials

	m.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at: %w", err)
		}
		m.CompletedAt = &t
	}

	return &m, nil
}

// UpdateMigration updates all fields of a migration.
func (s *StateStore) UpdateMigration(m *Migration) error {
	m.UpdatedAt = time.Now().UTC()

	var completedAt *string
	if m.CompletedAt != nil {
		s := m.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := s.db.Exec(`
		UPDATE _migrations
		SET status = ?, supabase_project_ref = ?, supabase_project_name = ?,
		    updated_at = ?, completed_at = ?, error_message = ?, credentials_encrypted = ?
		WHERE id = ?
	`, m.Status, m.SupabaseProjectRef, m.SupabaseProjectName,
		m.UpdatedAt.Format(time.RFC3339), completedAt, m.ErrorMessage, m.CredentialsEncrypted,
		m.ID)
	if err != nil {
		return fmt.Errorf("update migration: %w", err)
	}

	return nil
}

// ListMigrations returns all migrations ordered by created_at DESC.
func (s *StateStore) ListMigrations() ([]*Migration, error) {
	rows, err := s.db.Query(`
		SELECT id, status, supabase_project_ref, supabase_project_name,
		       created_at, updated_at, completed_at, error_message
		FROM _migrations
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	defer rows.Close()

	migrations := make([]*Migration, 0)
	for rows.Next() {
		var m Migration
		var projectRef, projectName sql.NullString
		var completedAt sql.NullString
		var errorMsg sql.NullString
		var createdAtStr, updatedAtStr string

		err := rows.Scan(
			&m.ID, &m.Status, &projectRef, &projectName,
			&createdAtStr, &updatedAtStr, &completedAt, &errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("scan migration: %w", err)
		}

		m.SupabaseProjectRef = projectRef.String
		m.SupabaseProjectName = projectName.String
		m.ErrorMessage = errorMsg.String

		m.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}

		m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}

		if completedAt.Valid {
			t, err := time.Parse(time.RFC3339, completedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse completed_at: %w", err)
			}
			m.CompletedAt = &t
		}

		migrations = append(migrations, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migrations: %w", err)
	}

	return migrations, nil
}

// DeleteMigration deletes a migration by ID.
func (s *StateStore) DeleteMigration(id string) error {
	_, err := s.db.Exec(`DELETE FROM _migrations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete migration: %w", err)
	}
	return nil
}

// CreateItem creates a new migration item with pending status.
func (s *StateStore) CreateItem(migrationID string, itemType ItemType, itemName string) (*MigrationItem, error) {
	id := uuid.New().String()

	_, err := s.db.Exec(`
		INSERT INTO _migration_items (id, migration_id, item_type, item_name, status)
		VALUES (?, ?, ?, ?, ?)
	`, id, migrationID, itemType, itemName, ItemPending)
	if err != nil {
		return nil, fmt.Errorf("create migration item: %w", err)
	}

	return &MigrationItem{
		ID:          id,
		MigrationID: migrationID,
		ItemType:    itemType,
		ItemName:    itemName,
		Status:      ItemPending,
	}, nil
}

// GetItems retrieves all items for a migration ordered by id.
func (s *StateStore) GetItems(migrationID string) ([]*MigrationItem, error) {
	rows, err := s.db.Query(`
		SELECT id, migration_id, item_type, item_name, status,
		       started_at, completed_at, error_message, rollback_info, metadata
		FROM _migration_items
		WHERE migration_id = ?
		ORDER BY id
	`, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get migration items: %w", err)
	}
	defer rows.Close()

	var items []*MigrationItem
	for rows.Next() {
		item, err := scanMigrationItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration items: %w", err)
	}

	return items, nil
}

// UpdateItem updates all fields of a migration item.
func (s *StateStore) UpdateItem(item *MigrationItem) error {
	var startedAt, completedAt *string
	if item.StartedAt != nil {
		s := item.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if item.CompletedAt != nil {
		s := item.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	var metadata *string
	if item.Metadata != nil {
		s := string(item.Metadata)
		metadata = &s
	}

	_, err := s.db.Exec(`
		UPDATE _migration_items
		SET status = ?, started_at = ?, completed_at = ?,
		    error_message = ?, rollback_info = ?, metadata = ?
		WHERE id = ?
	`, item.Status, startedAt, completedAt,
		item.ErrorMessage, item.RollbackInfo, metadata,
		item.ID)
	if err != nil {
		return fmt.Errorf("update migration item: %w", err)
	}

	return nil
}

// GetCompletedItemsReverse retrieves completed items for rollback, ordered by completed_at DESC.
func (s *StateStore) GetCompletedItemsReverse(migrationID string) ([]*MigrationItem, error) {
	rows, err := s.db.Query(`
		SELECT id, migration_id, item_type, item_name, status,
		       started_at, completed_at, error_message, rollback_info, metadata
		FROM _migration_items
		WHERE migration_id = ? AND status = ?
		ORDER BY completed_at DESC
	`, migrationID, ItemCompleted)
	if err != nil {
		return nil, fmt.Errorf("get completed migration items: %w", err)
	}
	defer rows.Close()

	var items []*MigrationItem
	for rows.Next() {
		item, err := scanMigrationItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration items: %w", err)
	}

	return items, nil
}

// scanMigrationItem scans a migration item from a row.
func scanMigrationItem(rows *sql.Rows) (*MigrationItem, error) {
	var item MigrationItem
	var startedAt, completedAt sql.NullString
	var errorMsg, rollbackInfo sql.NullString
	var metadata sql.NullString

	err := rows.Scan(
		&item.ID, &item.MigrationID, &item.ItemType, &item.ItemName, &item.Status,
		&startedAt, &completedAt, &errorMsg, &rollbackInfo, &metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("scan migration item: %w", err)
	}

	item.ErrorMessage = errorMsg.String
	item.RollbackInfo = rollbackInfo.String

	if metadata.Valid && metadata.String != "" {
		item.Metadata = json.RawMessage(metadata.String)
	}

	if startedAt.Valid {
		t, err := time.Parse(time.RFC3339, startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse started_at: %w", err)
		}
		item.StartedAt = &t
	}

	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at: %w", err)
		}
		item.CompletedAt = &t
	}

	return &item, nil
}

// CreateVerification creates a new verification record.
func (s *StateStore) CreateVerification(migrationID string, layer VerificationLayer) (*Verification, error) {
	id := uuid.New().String()

	_, err := s.db.Exec(`
		INSERT INTO _migration_verifications (id, migration_id, layer, status)
		VALUES (?, ?, ?, ?)
	`, id, migrationID, layer, VerifyPending)
	if err != nil {
		return nil, fmt.Errorf("create verification: %w", err)
	}

	return &Verification{
		ID:          id,
		MigrationID: migrationID,
		Layer:       layer,
		Status:      VerifyPending,
	}, nil
}

// GetVerifications retrieves all verifications for a migration.
func (s *StateStore) GetVerifications(migrationID string) ([]*Verification, error) {
	rows, err := s.db.Query(`
		SELECT id, migration_id, layer, status, started_at, completed_at, results
		FROM _migration_verifications
		WHERE migration_id = ?
		ORDER BY id
	`, migrationID)
	if err != nil {
		return nil, fmt.Errorf("get verifications: %w", err)
	}
	defer rows.Close()

	var verifications []*Verification
	for rows.Next() {
		var v Verification
		var startedAt, completedAt sql.NullString
		var results sql.NullString

		err := rows.Scan(
			&v.ID, &v.MigrationID, &v.Layer, &v.Status,
			&startedAt, &completedAt, &results,
		)
		if err != nil {
			return nil, fmt.Errorf("scan verification: %w", err)
		}

		if results.Valid && results.String != "" {
			v.Results = json.RawMessage(results.String)
		}

		if startedAt.Valid {
			t, err := time.Parse(time.RFC3339, startedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse started_at: %w", err)
			}
			v.StartedAt = &t
		}

		if completedAt.Valid {
			t, err := time.Parse(time.RFC3339, completedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse completed_at: %w", err)
			}
			v.CompletedAt = &t
		}

		verifications = append(verifications, &v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate verifications: %w", err)
	}

	return verifications, nil
}

// UpdateVerification updates a verification record.
func (s *StateStore) UpdateVerification(v *Verification) error {
	var startedAt, completedAt *string
	if v.StartedAt != nil {
		s := v.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if v.CompletedAt != nil {
		s := v.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	var results *string
	if v.Results != nil {
		s := string(v.Results)
		results = &s
	}

	_, err := s.db.Exec(`
		UPDATE _migration_verifications
		SET status = ?, started_at = ?, completed_at = ?, results = ?
		WHERE id = ?
	`, v.Status, startedAt, completedAt, results, v.ID)
	if err != nil {
		return fmt.Errorf("update verification: %w", err)
	}

	return nil
}
