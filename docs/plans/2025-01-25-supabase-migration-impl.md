# Supabase Migration Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an Export & Migration Center in the sblite dashboard that enables users to migrate to Supabase via automated one-click migration or manual export packages.

**Architecture:** Tabbed dashboard UI (Export, Migrate, Verify, History) backed by a migration service that orchestrates exports, Supabase Management API calls, and verification. State persisted in SQLite for resume capability.

**Tech Stack:** Go (backend), Vanilla JS (frontend), Supabase Management API, PostgreSQL direct connection

**Design Document:** `docs/plans/2025-01-25-supabase-migration-design.md`

---

## Phase 1: Foundation - Database Schema & State Management

### Task 1.1: Add Migration Tables to Database Schema

**Files:**
- Modify: `internal/db/migrations.go`

**Step 1: Add migration schema constant**

Add after the existing schema constants (around line 250):

```go
const migrationSchema = `
CREATE TABLE IF NOT EXISTS _migrations (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'rolled_back')),
    supabase_project_ref TEXT,
    supabase_project_name TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT,
    error_message TEXT,
    credentials_encrypted TEXT
);

CREATE INDEX IF NOT EXISTS idx_migrations_status ON _migrations(status);
CREATE INDEX IF NOT EXISTS idx_migrations_created_at ON _migrations(created_at DESC);

CREATE TABLE IF NOT EXISTS _migration_items (
    id TEXT PRIMARY KEY,
    migration_id TEXT NOT NULL REFERENCES _migrations(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL CHECK (item_type IN ('schema', 'data', 'users', 'identities', 'rls', 'storage_buckets', 'storage_files', 'functions', 'secrets', 'auth_config', 'oauth_config', 'email_templates')),
    item_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'skipped', 'rolled_back')),
    started_at TEXT,
    completed_at TEXT,
    error_message TEXT,
    rollback_info TEXT,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_migration_items_migration_id ON _migration_items(migration_id);
CREATE INDEX IF NOT EXISTS idx_migration_items_status ON _migration_items(status);

CREATE TABLE IF NOT EXISTS _migration_verifications (
    id TEXT PRIMARY KEY,
    migration_id TEXT NOT NULL REFERENCES _migrations(id) ON DELETE CASCADE,
    layer TEXT NOT NULL CHECK (layer IN ('basic', 'integrity', 'functional')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'passed', 'failed')),
    started_at TEXT,
    completed_at TEXT,
    results TEXT
);

CREATE INDEX IF NOT EXISTS idx_migration_verifications_migration_id ON _migration_verifications(migration_id);
`
```

**Step 2: Add migrationSchema to RunMigrations function**

Find the `RunMigrations` function and add `migrationSchema` to the list of schemas executed:

```go
// In RunMigrations(), add to the schemas slice:
schemas := []string{
    authSchema,
    rlsSchema,
    emailSchema,
    // ... other existing schemas ...
    migrationSchema,  // Add this line
}
```

**Step 3: Run tests**

Run: `go test ./internal/db/... -v`
Expected: All existing tests pass

**Step 4: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(migration): add database schema for migration state tracking"
```

---

### Task 1.2: Create Migration State Package

**Files:**
- Create: `internal/dashboard/migration/state.go`

**Step 1: Create the migration package directory**

```bash
mkdir -p internal/dashboard/migration
```

**Step 2: Write state.go with types and CRUD operations**

```go
// Package migration provides Supabase migration functionality.
package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MigrationStatus represents the status of a migration.
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

// ItemType represents the type of item being migrated.
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

// Migration represents a migration session.
type Migration struct {
	ID                   string          `json:"id"`
	Status               MigrationStatus `json:"status"`
	SupabaseProjectRef   string          `json:"supabase_project_ref,omitempty"`
	SupabaseProjectName  string          `json:"supabase_project_name,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	CompletedAt          *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage         string          `json:"error_message,omitempty"`
	CredentialsEncrypted string          `json:"-"` // Never expose in JSON
}

// MigrationItem represents an individual item within a migration.
type MigrationItem struct {
	ID           string     `json:"id"`
	MigrationID  string     `json:"migration_id"`
	ItemType     ItemType   `json:"item_type"`
	ItemName     string     `json:"item_name,omitempty"`
	Status       ItemStatus `json:"status"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	RollbackInfo string     `json:"rollback_info,omitempty"`
	Metadata     string     `json:"metadata,omitempty"`
}

// VerificationLayer represents a verification layer type.
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

// Verification represents a verification result.
type Verification struct {
	ID          string             `json:"id"`
	MigrationID string             `json:"migration_id"`
	Layer       VerificationLayer  `json:"layer"`
	Status      VerificationStatus `json:"status"`
	StartedAt   *time.Time         `json:"started_at,omitempty"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
	Results     json.RawMessage    `json:"results,omitempty"`
}

// StateStore manages migration state persistence.
type StateStore struct {
	db *sql.DB
}

// NewStateStore creates a new StateStore.
func NewStateStore(db *sql.DB) *StateStore {
	return &StateStore{db: db}
}

// CreateMigration creates a new migration session.
func (s *StateStore) CreateMigration() (*Migration, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO _migrations (id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, id, StatusPending, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create migration: %w", err)
	}

	return &Migration{
		ID:        id,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetMigration retrieves a migration by ID.
func (s *StateStore) GetMigration(id string) (*Migration, error) {
	var m Migration
	var createdAt, updatedAt string
	var completedAt, projectRef, projectName, errorMsg, creds sql.NullString

	err := s.db.QueryRow(`
		SELECT id, status, supabase_project_ref, supabase_project_name,
		       created_at, updated_at, completed_at, error_message, credentials_encrypted
		FROM _migrations WHERE id = ?
	`, id).Scan(&m.ID, &m.Status, &projectRef, &projectName,
		&createdAt, &updatedAt, &completedAt, &errorMsg, &creds)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get migration: %w", err)
	}

	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		m.CompletedAt = &t
	}
	m.SupabaseProjectRef = projectRef.String
	m.SupabaseProjectName = projectName.String
	m.ErrorMessage = errorMsg.String
	m.CredentialsEncrypted = creds.String

	return &m, nil
}

// UpdateMigration updates a migration's fields.
func (s *StateStore) UpdateMigration(m *Migration) error {
	now := time.Now().UTC()
	m.UpdatedAt = now

	var completedAt interface{}
	if m.CompletedAt != nil {
		completedAt = m.CompletedAt.Format(time.RFC3339)
	}

	_, err := s.db.Exec(`
		UPDATE _migrations SET
			status = ?,
			supabase_project_ref = ?,
			supabase_project_name = ?,
			updated_at = ?,
			completed_at = ?,
			error_message = ?,
			credentials_encrypted = ?
		WHERE id = ?
	`, m.Status, m.SupabaseProjectRef, m.SupabaseProjectName,
		now.Format(time.RFC3339), completedAt, m.ErrorMessage,
		m.CredentialsEncrypted, m.ID)
	if err != nil {
		return fmt.Errorf("failed to update migration: %w", err)
	}
	return nil
}

// ListMigrations lists all migrations, ordered by creation date descending.
func (s *StateStore) ListMigrations() ([]*Migration, error) {
	rows, err := s.db.Query(`
		SELECT id, status, supabase_project_ref, supabase_project_name,
		       created_at, updated_at, completed_at, error_message
		FROM _migrations ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list migrations: %w", err)
	}
	defer rows.Close()

	var migrations []*Migration
	for rows.Next() {
		var m Migration
		var createdAt, updatedAt string
		var completedAt, projectRef, projectName, errorMsg sql.NullString

		err := rows.Scan(&m.ID, &m.Status, &projectRef, &projectName,
			&createdAt, &updatedAt, &completedAt, &errorMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}

		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			m.CompletedAt = &t
		}
		m.SupabaseProjectRef = projectRef.String
		m.SupabaseProjectName = projectName.String
		m.ErrorMessage = errorMsg.String

		migrations = append(migrations, &m)
	}
	return migrations, nil
}

// DeleteMigration deletes a migration and all its items.
func (s *StateStore) DeleteMigration(id string) error {
	_, err := s.db.Exec(`DELETE FROM _migrations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete migration: %w", err)
	}
	return nil
}

// CreateItem creates a new migration item.
func (s *StateStore) CreateItem(migrationID string, itemType ItemType, itemName string) (*MigrationItem, error) {
	id := uuid.New().String()

	_, err := s.db.Exec(`
		INSERT INTO _migration_items (id, migration_id, item_type, item_name, status)
		VALUES (?, ?, ?, ?, ?)
	`, id, migrationID, itemType, itemName, ItemPending)
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	return &MigrationItem{
		ID:          id,
		MigrationID: migrationID,
		ItemType:    itemType,
		ItemName:    itemName,
		Status:      ItemPending,
	}, nil
}

// GetItems retrieves all items for a migration.
func (s *StateStore) GetItems(migrationID string) ([]*MigrationItem, error) {
	rows, err := s.db.Query(`
		SELECT id, migration_id, item_type, item_name, status,
		       started_at, completed_at, error_message, rollback_info, metadata
		FROM _migration_items WHERE migration_id = ?
		ORDER BY id
	`, migrationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*MigrationItem
	for rows.Next() {
		var item MigrationItem
		var startedAt, completedAt, errorMsg, rollbackInfo, metadata sql.NullString

		err := rows.Scan(&item.ID, &item.MigrationID, &item.ItemType, &item.ItemName,
			&item.Status, &startedAt, &completedAt, &errorMsg, &rollbackInfo, &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}

		if startedAt.Valid {
			t, _ := time.Parse(time.RFC3339, startedAt.String)
			item.StartedAt = &t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			item.CompletedAt = &t
		}
		item.ErrorMessage = errorMsg.String
		item.RollbackInfo = rollbackInfo.String
		item.Metadata = metadata.String

		items = append(items, &item)
	}
	return items, nil
}

// UpdateItem updates a migration item.
func (s *StateStore) UpdateItem(item *MigrationItem) error {
	var startedAt, completedAt interface{}
	if item.StartedAt != nil {
		startedAt = item.StartedAt.Format(time.RFC3339)
	}
	if item.CompletedAt != nil {
		completedAt = item.CompletedAt.Format(time.RFC3339)
	}

	_, err := s.db.Exec(`
		UPDATE _migration_items SET
			status = ?,
			started_at = ?,
			completed_at = ?,
			error_message = ?,
			rollback_info = ?,
			metadata = ?
		WHERE id = ?
	`, item.Status, startedAt, completedAt, item.ErrorMessage,
		item.RollbackInfo, item.Metadata, item.ID)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}
	return nil
}

// GetCompletedItemsReverse gets completed items in reverse order for rollback.
func (s *StateStore) GetCompletedItemsReverse(migrationID string) ([]*MigrationItem, error) {
	rows, err := s.db.Query(`
		SELECT id, migration_id, item_type, item_name, status,
		       started_at, completed_at, error_message, rollback_info, metadata
		FROM _migration_items
		WHERE migration_id = ? AND status = ?
		ORDER BY completed_at DESC
	`, migrationID, ItemCompleted)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed items: %w", err)
	}
	defer rows.Close()

	var items []*MigrationItem
	for rows.Next() {
		var item MigrationItem
		var startedAt, completedAt, errorMsg, rollbackInfo, metadata sql.NullString

		err := rows.Scan(&item.ID, &item.MigrationID, &item.ItemType, &item.ItemName,
			&item.Status, &startedAt, &completedAt, &errorMsg, &rollbackInfo, &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}

		if startedAt.Valid {
			t, _ := time.Parse(time.RFC3339, startedAt.String)
			item.StartedAt = &t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			item.CompletedAt = &t
		}
		item.ErrorMessage = errorMsg.String
		item.RollbackInfo = rollbackInfo.String
		item.Metadata = metadata.String

		items = append(items, &item)
	}
	return items, nil
}
```

**Step 3: Run tests**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/dashboard/migration/state.go
git commit -m "feat(migration): add state management for migration tracking"
```

---

### Task 1.3: Create State Store Tests

**Files:**
- Create: `internal/dashboard/migration/state_test.go`

**Step 1: Write comprehensive tests**

```go
package migration

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create migration tables
	_, err = db.Exec(`
		CREATE TABLE _migrations (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'pending',
			supabase_project_ref TEXT,
			supabase_project_name TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT,
			error_message TEXT,
			credentials_encrypted TEXT
		);

		CREATE TABLE _migration_items (
			id TEXT PRIMARY KEY,
			migration_id TEXT NOT NULL,
			item_type TEXT NOT NULL,
			item_name TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT,
			completed_at TEXT,
			error_message TEXT,
			rollback_info TEXT,
			metadata TEXT
		);

		CREATE TABLE _migration_verifications (
			id TEXT PRIMARY KEY,
			migration_id TEXT NOT NULL,
			layer TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT,
			completed_at TEXT,
			results TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return db
}

func TestCreateMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	if m.ID == "" {
		t.Error("expected non-empty ID")
	}
	if m.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, m.Status)
	}
}

func TestGetMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	created, _ := store.CreateMigration()
	got, err := store.GetMigration(created.ID)
	if err != nil {
		t.Fatalf("GetMigration failed: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, got.ID)
	}
}

func TestGetMigrationNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	got, err := store.GetMigration("nonexistent")
	if err != nil {
		t.Fatalf("GetMigration failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent migration")
	}
}

func TestUpdateMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	m.Status = StatusInProgress
	m.SupabaseProjectRef = "test-project"
	m.SupabaseProjectName = "Test Project"

	err := store.UpdateMigration(m)
	if err != nil {
		t.Fatalf("UpdateMigration failed: %v", err)
	}

	got, _ := store.GetMigration(m.ID)
	if got.Status != StatusInProgress {
		t.Errorf("expected status %s, got %s", StatusInProgress, got.Status)
	}
	if got.SupabaseProjectRef != "test-project" {
		t.Errorf("expected project ref 'test-project', got '%s'", got.SupabaseProjectRef)
	}
}

func TestListMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	store.CreateMigration()
	store.CreateMigration()

	migrations, err := store.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations failed: %v", err)
	}

	if len(migrations) != 2 {
		t.Errorf("expected 2 migrations, got %d", len(migrations))
	}
}

func TestDeleteMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	err := store.DeleteMigration(m.ID)
	if err != nil {
		t.Fatalf("DeleteMigration failed: %v", err)
	}

	got, _ := store.GetMigration(m.ID)
	if got != nil {
		t.Error("expected migration to be deleted")
	}
}

func TestCreateItem(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	item, err := store.CreateItem(m.ID, ItemSchema, "users")
	if err != nil {
		t.Fatalf("CreateItem failed: %v", err)
	}

	if item.ItemType != ItemSchema {
		t.Errorf("expected item type %s, got %s", ItemSchema, item.ItemType)
	}
	if item.ItemName != "users" {
		t.Errorf("expected item name 'users', got '%s'", item.ItemName)
	}
}

func TestGetItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	store.CreateItem(m.ID, ItemSchema, "users")
	store.CreateItem(m.ID, ItemData, "users")

	items, err := store.GetItems(m.ID)
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestUpdateItem(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	item, _ := store.CreateItem(m.ID, ItemSchema, "users")

	item.Status = ItemCompleted
	item.RollbackInfo = `{"action":"DROP TABLE","table":"users"}`

	err := store.UpdateItem(item)
	if err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	items, _ := store.GetItems(m.ID)
	if items[0].Status != ItemCompleted {
		t.Errorf("expected status %s, got %s", ItemCompleted, items[0].Status)
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/dashboard/migration/... -v`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/dashboard/migration/state_test.go
git commit -m "test(migration): add state store unit tests"
```

---

## Phase 2: Supabase Management API Client

### Task 2.1: Create Supabase API Client

**Files:**
- Create: `internal/dashboard/migration/supabase_client.go`

**Step 1: Write the Supabase client**

```go
package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const (
	supabaseAPIBaseURL = "https://api.supabase.com"
)

// SupabaseClient interacts with the Supabase Management API.
type SupabaseClient struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

// NewSupabaseClient creates a new client with the given access token.
func NewSupabaseClient(token string) *SupabaseClient {
	return &SupabaseClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: supabaseAPIBaseURL,
	}
}

// Project represents a Supabase project.
type Project struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	CreatedAt      string `json:"created_at"`
	Database       struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"database"`
}

// APIKey represents a Supabase API key.
type APIKey struct {
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
}

// ListProjects lists all projects accessible to the user.
func (c *SupabaseClient) ListProjects() ([]Project, error) {
	resp, err := c.doRequest("GET", "/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list projects: %s - %s", resp.Status, string(body))
	}

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("failed to decode projects: %w", err)
	}

	return projects, nil
}

// GetProject retrieves a project by reference.
func (c *SupabaseClient) GetProject(ref string) (*Project, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/v1/projects/%s", ref), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get project: %s - %s", resp.Status, string(body))
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return &project, nil
}

// GetAPIKeys retrieves API keys for a project.
func (c *SupabaseClient) GetAPIKeys(projectRef string) ([]APIKey, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/v1/projects/%s/api-keys", projectRef), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get API keys: %s - %s", resp.Status, string(body))
	}

	var keys []APIKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("failed to decode API keys: %w", err)
	}

	return keys, nil
}

// Secret represents a Supabase secret.
type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// CreateSecrets creates secrets for a project.
func (c *SupabaseClient) CreateSecrets(projectRef string, secrets []Secret) error {
	body, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	resp, err := c.doRequest("POST", fmt.Sprintf("/v1/projects/%s/secrets", projectRef), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create secrets: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// DeleteSecrets deletes secrets from a project.
func (c *SupabaseClient) DeleteSecrets(projectRef string, names []string) error {
	body, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("failed to marshal secret names: %w", err)
	}

	resp, err := c.doRequest("DELETE", fmt.Sprintf("/v1/projects/%s/secrets", projectRef), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete secrets: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// FunctionMetadata contains metadata for deploying a function.
type FunctionMetadata struct {
	Name           string `json:"name"`
	EntrypointPath string `json:"entrypoint_path"`
	VerifyJWT      *bool  `json:"verify_jwt,omitempty"`
}

// DeployFunction deploys an edge function.
func (c *SupabaseClient) DeployFunction(projectRef, slug string, metadata FunctionMetadata, fileContent []byte) error {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := writer.WriteField("metadata", string(metadataJSON)); err != nil {
		return fmt.Errorf("failed to write metadata field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("file", "function.zip")
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/v1/projects/%s/functions/deploy?slug=%s", c.baseURL, projectRef, slug)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deploy function: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to deploy function: %s - %s", resp.Status, string(body))
	}

	return nil
}

// DeleteFunction deletes an edge function.
func (c *SupabaseClient) DeleteFunction(projectRef, slug string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/v1/projects/%s/functions/%s", projectRef, slug), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete function: %s - %s", resp.Status, string(body))
	}

	return nil
}

// AuthConfig represents Supabase auth configuration.
type AuthConfig map[string]interface{}

// GetAuthConfig retrieves auth configuration.
func (c *SupabaseClient) GetAuthConfig(projectRef string) (AuthConfig, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/v1/projects/%s/config/auth", projectRef), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get auth config: %s - %s", resp.Status, string(body))
	}

	var config AuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode auth config: %w", err)
	}

	return config, nil
}

// UpdateAuthConfig updates auth configuration.
func (c *SupabaseClient) UpdateAuthConfig(projectRef string, config AuthConfig) error {
	body, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal auth config: %w", err)
	}

	resp, err := c.doRequest("PATCH", fmt.Sprintf("/v1/projects/%s/config/auth", projectRef), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update auth config: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// doRequest performs an HTTP request with authorization.
func (c *SupabaseClient) doRequest(method, path string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// ValidateToken checks if the token is valid by listing projects.
func (c *SupabaseClient) ValidateToken() error {
	_, err := c.ListProjects()
	return err
}
```

**Step 2: Run build**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/dashboard/migration/supabase_client.go
git commit -m "feat(migration): add Supabase Management API client"
```

---

## Phase 3: Export Endpoints (Manual Migration Path)

### Task 3.1: Add RLS Policy Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

Add after existing export handlers (around line 3000):

```go
// handleExportRLS exports RLS policies as PostgreSQL SQL.
func (h *Handler) handleExportRLS(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT table_name, policy_name, command, using_expr, check_expr, enabled
		FROM _rls_policies
		ORDER BY table_name, policy_name
	`)
	if err != nil {
		h.jsonError(w, "Failed to query policies", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("-- RLS Policies exported from sblite\n")
	sb.WriteString("-- Review and adjust before executing in Supabase\n\n")

	// Track which tables need RLS enabled
	tablesWithRLS := make(map[string]bool)

	for rows.Next() {
		var tableName, policyName, command string
		var usingExpr, checkExpr sql.NullString
		var enabled int

		if err := rows.Scan(&tableName, &policyName, &command, &usingExpr, &checkExpr, &enabled); err != nil {
			h.jsonError(w, "Failed to scan policy", http.StatusInternalServerError)
			return
		}

		tablesWithRLS[tableName] = true

		// Skip disabled policies
		if enabled == 0 {
			sb.WriteString(fmt.Sprintf("-- DISABLED: Policy %s on %s\n", policyName, tableName))
			continue
		}

		// Build CREATE POLICY statement
		sb.WriteString(fmt.Sprintf("CREATE POLICY \"%s\" ON \"%s\"\n", policyName, tableName))

		// Map command
		switch command {
		case "ALL":
			sb.WriteString("  FOR ALL\n")
		case "SELECT":
			sb.WriteString("  FOR SELECT\n")
		case "INSERT":
			sb.WriteString("  FOR INSERT\n")
		case "UPDATE":
			sb.WriteString("  FOR UPDATE\n")
		case "DELETE":
			sb.WriteString("  FOR DELETE\n")
		}

		sb.WriteString("  TO authenticated\n")

		if usingExpr.Valid && usingExpr.String != "" {
			sb.WriteString(fmt.Sprintf("  USING (%s)\n", usingExpr.String))
		}

		if checkExpr.Valid && checkExpr.String != "" {
			sb.WriteString(fmt.Sprintf("  WITH CHECK (%s)\n", checkExpr.String))
		}

		sb.WriteString(";\n\n")
	}

	// Add ALTER TABLE statements to enable RLS
	if len(tablesWithRLS) > 0 {
		sb.WriteString("-- Enable RLS on tables\n")
		for tableName := range tablesWithRLS {
			sb.WriteString(fmt.Sprintf("ALTER TABLE \"%s\" ENABLE ROW LEVEL SECURITY;\n", tableName))
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=rls-policies.sql")
	w.Write([]byte(sb.String()))
}
```

**Step 2: Register the route**

Find the export route group (around line 256) and add:

```go
r.Get("/rls", h.handleExportRLS)
```

**Step 3: Test manually**

Run server: `go run . serve --db test.db`
Test: `curl http://localhost:8080/_/api/export/rls`

**Step 4: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add RLS policy export endpoint"
```

---

### Task 3.2: Add Auth Users Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

```go
// handleExportAuthUsers exports auth users as JSON.
func (h *Handler) handleExportAuthUsers(w http.ResponseWriter, r *http.Request) {
	includePasswords := r.URL.Query().Get("include_passwords") == "true"

	rows, err := h.db.Query(`
		SELECT id, email, encrypted_password, email_confirmed_at,
		       raw_app_meta_data, raw_user_meta_data, role, is_anonymous,
		       created_at, updated_at, last_sign_in_at
		FROM auth_users
		WHERE deleted_at IS NULL
		ORDER BY created_at
	`)
	if err != nil {
		h.jsonError(w, "Failed to query users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ExportUser struct {
		ID                string          `json:"id"`
		Email             string          `json:"email,omitempty"`
		EncryptedPassword string          `json:"encrypted_password,omitempty"`
		EmailConfirmedAt  *string         `json:"email_confirmed_at,omitempty"`
		AppMetadata       json.RawMessage `json:"app_metadata"`
		UserMetadata      json.RawMessage `json:"user_metadata"`
		Role              string          `json:"role"`
		IsAnonymous       bool            `json:"is_anonymous"`
		CreatedAt         string          `json:"created_at"`
		UpdatedAt         string          `json:"updated_at"`
		LastSignInAt      *string         `json:"last_sign_in_at,omitempty"`
	}

	var users []ExportUser
	for rows.Next() {
		var u ExportUser
		var encPassword sql.NullString
		var emailConfirmed, lastSignIn sql.NullString
		var appMeta, userMeta string
		var isAnon int

		err := rows.Scan(&u.ID, &u.Email, &encPassword, &emailConfirmed,
			&appMeta, &userMeta, &u.Role, &isAnon, &u.CreatedAt, &u.UpdatedAt, &lastSignIn)
		if err != nil {
			h.jsonError(w, "Failed to scan user", http.StatusInternalServerError)
			return
		}

		if includePasswords && encPassword.Valid {
			u.EncryptedPassword = encPassword.String
		}
		if emailConfirmed.Valid {
			u.EmailConfirmedAt = &emailConfirmed.String
		}
		if lastSignIn.Valid {
			u.LastSignInAt = &lastSignIn.String
		}
		u.AppMetadata = json.RawMessage(appMeta)
		u.UserMetadata = json.RawMessage(userMeta)
		u.IsAnonymous = isAnon == 1

		users = append(users, u)
	}

	export := struct {
		ExportedAt string       `json:"exported_at"`
		Count      int          `json:"count"`
		Users      []ExportUser `json:"users"`
		Note       string       `json:"note"`
	}{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Count:      len(users),
		Users:      users,
		Note:       "Import users into Supabase auth.users table. Bcrypt password hashes are compatible.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=auth-users.json")
	json.NewEncoder(w).Encode(export)
}
```

**Step 2: Register the route**

Add to export routes:

```go
r.Route("/auth", func(r chi.Router) {
	r.Get("/users", h.handleExportAuthUsers)
})
```

**Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add auth users export endpoint"
```

---

### Task 3.3: Add Auth Config Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

```go
// handleExportAuthConfig exports auth configuration as JSON.
func (h *Handler) handleExportAuthConfig(w http.ResponseWriter, r *http.Request) {
	// Query dashboard store for auth settings
	settings := make(map[string]interface{})

	// Get JWT settings
	rows, err := h.db.Query(`SELECT key, value FROM _dashboard WHERE key LIKE 'auth_%' OR key LIKE 'jwt_%' OR key LIKE 'smtp_%'`)
	if err != nil {
		h.jsonError(w, "Failed to query settings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		// Redact sensitive values
		if strings.Contains(key, "secret") || strings.Contains(key, "password") || strings.Contains(key, "pass") {
			settings[key] = "[REDACTED]"
		} else {
			settings[key] = value
		}
	}

	export := struct {
		ExportedAt string                 `json:"exported_at"`
		Settings   map[string]interface{} `json:"settings"`
		Note       string                 `json:"note"`
	}{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Settings:   settings,
		Note:       "Configure these settings in Supabase Dashboard > Authentication > Settings",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=auth-config.json")
	json.NewEncoder(w).Encode(export)
}
```

**Step 2: Register the route**

Add to auth export routes:

```go
r.Get("/config", h.handleExportAuthConfig)
```

**Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add auth config export endpoint"
```

---

### Task 3.4: Add Email Templates Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

```go
// handleExportEmailTemplates exports email templates as JSON.
func (h *Handler) handleExportEmailTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, template_type, subject, body_html, body_text, created_at, updated_at
		FROM auth_email_templates
		ORDER BY template_type
	`)
	if err != nil {
		h.jsonError(w, "Failed to query templates", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Template struct {
		ID           string `json:"id"`
		TemplateType string `json:"template_type"`
		Subject      string `json:"subject"`
		BodyHTML     string `json:"body_html"`
		BodyText     string `json:"body_text"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	var templates []Template
	for rows.Next() {
		var t Template
		var bodyText sql.NullString
		if err := rows.Scan(&t.ID, &t.TemplateType, &t.Subject, &t.BodyHTML, &bodyText, &t.CreatedAt, &t.UpdatedAt); err != nil {
			h.jsonError(w, "Failed to scan template", http.StatusInternalServerError)
			return
		}
		t.BodyText = bodyText.String
		templates = append(templates, t)
	}

	export := struct {
		ExportedAt string     `json:"exported_at"`
		Count      int        `json:"count"`
		Templates  []Template `json:"templates"`
		Note       string     `json:"note"`
	}{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Count:      len(templates),
		Templates:  templates,
		Note:       "Configure email templates in Supabase Dashboard > Authentication > Email Templates",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=email-templates.json")
	json.NewEncoder(w).Encode(export)
}
```

**Step 2: Register the route**

Add to auth export routes:

```go
r.Get("/templates", h.handleExportEmailTemplates)
```

**Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add email templates export endpoint"
```

---

### Task 3.5: Add Storage Buckets Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

```go
// handleExportStorageBuckets exports storage bucket configurations as JSON.
func (h *Handler) handleExportStorageBuckets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, name, public, file_size_limit, allowed_mime_types, created_at, updated_at
		FROM storage_buckets
		ORDER BY name
	`)
	if err != nil {
		h.jsonError(w, "Failed to query buckets", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Bucket struct {
		ID               string   `json:"id"`
		Name             string   `json:"name"`
		Public           bool     `json:"public"`
		FileSizeLimit    *int64   `json:"file_size_limit,omitempty"`
		AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
		CreatedAt        string   `json:"created_at"`
		UpdatedAt        string   `json:"updated_at"`
	}

	var buckets []Bucket
	for rows.Next() {
		var b Bucket
		var isPublic int
		var fileSizeLimit sql.NullInt64
		var allowedMimeTypes sql.NullString

		if err := rows.Scan(&b.ID, &b.Name, &isPublic, &fileSizeLimit, &allowedMimeTypes, &b.CreatedAt, &b.UpdatedAt); err != nil {
			h.jsonError(w, "Failed to scan bucket", http.StatusInternalServerError)
			return
		}

		b.Public = isPublic == 1
		if fileSizeLimit.Valid {
			b.FileSizeLimit = &fileSizeLimit.Int64
		}
		if allowedMimeTypes.Valid && allowedMimeTypes.String != "" {
			json.Unmarshal([]byte(allowedMimeTypes.String), &b.AllowedMimeTypes)
		}

		buckets = append(buckets, b)
	}

	export := struct {
		ExportedAt string   `json:"exported_at"`
		Count      int      `json:"count"`
		Buckets    []Bucket `json:"buckets"`
		Note       string   `json:"note"`
	}{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Count:      len(buckets),
		Buckets:    buckets,
		Note:       "Create buckets in Supabase via SQL: INSERT INTO storage.buckets (id, name, public) VALUES (...)",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=storage-buckets.json")
	json.NewEncoder(w).Encode(export)
}
```

**Step 2: Register the route**

Add to export routes:

```go
r.Route("/storage", func(r chi.Router) {
	r.Get("/buckets", h.handleExportStorageBuckets)
})
```

**Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add storage buckets export endpoint"
```

---

### Task 3.6: Add Edge Functions Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

```go
// handleExportFunctions exports edge functions as a ZIP file.
func (h *Handler) handleExportFunctions(w http.ResponseWriter, r *http.Request) {
	functionsDir := h.serverConfig.FunctionsDir
	if functionsDir == "" {
		functionsDir = "./functions"
	}

	// Check if functions directory exists
	if _, err := os.Stat(functionsDir); os.IsNotExist(err) {
		h.jsonError(w, "Functions directory not found", http.StatusNotFound)
		return
	}

	// Create ZIP in memory
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Walk the functions directory
	err := filepath.Walk(functionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(functionsDir, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Create header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
		}

		return nil
	})

	if err != nil {
		h.jsonError(w, "Failed to create ZIP", http.StatusInternalServerError)
		return
	}

	// Add README
	readme := `# Edge Functions Export

These functions were exported from sblite.

## Deployment to Supabase

1. Install Supabase CLI: npm install -g supabase
2. Login: supabase login
3. Link project: supabase link --project-ref YOUR_PROJECT_REF
4. Deploy: supabase functions deploy --all

## Individual Function Deployment

supabase functions deploy FUNCTION_NAME

## Note

Secrets are NOT included in this export. Set them manually:
supabase secrets set SECRET_NAME=value
`
	readmeWriter, _ := zipWriter.Create("README.md")
	readmeWriter.Write([]byte(readme))

	zipWriter.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=edge-functions.zip")
	w.Write(buf.Bytes())
}
```

**Step 2: Add imports at top of file**

```go
import (
	"archive/zip"
	// ... other imports
)
```

**Step 3: Register the route**

Add to export routes:

```go
r.Get("/functions", h.handleExportFunctions)
```

**Step 4: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add edge functions export endpoint"
```

---

### Task 3.7: Add Secrets Export Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add the handler method**

```go
// handleExportSecrets exports secret names (NOT values) as text.
func (h *Handler) handleExportSecrets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT name FROM _functions_secrets ORDER BY name`)
	if err != nil {
		h.jsonError(w, "Failed to query secrets", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("# Edge Function Secrets\n")
	sb.WriteString("# Values are NOT exported for security reasons.\n")
	sb.WriteString("# Set these secrets in Supabase using:\n")
	sb.WriteString("#   supabase secrets set SECRET_NAME=value\n\n")

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s=\n", name))
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=secrets.env.template")
	w.Write([]byte(sb.String()))
}
```

**Step 2: Register the route**

Add to export routes:

```go
r.Get("/secrets", h.handleExportSecrets)
```

**Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(migration): add secrets export endpoint (names only)"
```

---

## Phase 4: Migration Service Core

### Task 4.1: Create Migration Service

**Files:**
- Create: `internal/dashboard/migration/service.go`

**Step 1: Write the migration service**

```go
package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
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
	return s.state.GetMigration(id)
}

// ListMigrations lists all migrations.
func (s *Service) ListMigrations() ([]*Migration, error) {
	return s.state.ListMigrations()
}

// DeleteMigration deletes a migration.
func (s *Service) DeleteMigration(id string) error {
	return s.state.DeleteMigration(id)
}

// ConnectSupabase stores Supabase credentials and validates them.
func (s *Service) ConnectSupabase(migrationID, token string) error {
	m, err := s.state.GetMigration(migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration not found")
	}

	// Validate token
	client := NewSupabaseClient(token)
	if err := client.ValidateToken(); err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	// Store encrypted token (TODO: implement actual encryption)
	m.CredentialsEncrypted = token // In production, encrypt this
	return s.state.UpdateMigration(m)
}

// ListSupabaseProjects lists projects for the connected account.
func (s *Service) ListSupabaseProjects(migrationID string) ([]Project, error) {
	m, err := s.state.GetMigration(migrationID)
	if err != nil {
		return nil, err
	}
	if m == nil || m.CredentialsEncrypted == "" {
		return nil, fmt.Errorf("not connected to Supabase")
	}

	client := NewSupabaseClient(m.CredentialsEncrypted)
	return client.ListProjects()
}

// SelectProject selects a Supabase project for migration.
func (s *Service) SelectProject(migrationID, projectRef string) error {
	m, err := s.state.GetMigration(migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration not found")
	}

	client := NewSupabaseClient(m.CredentialsEncrypted)
	project, err := client.GetProject(projectRef)
	if err != nil {
		return err
	}

	m.SupabaseProjectRef = projectRef
	m.SupabaseProjectName = project.Name
	return s.state.UpdateMigration(m)
}

// SelectItemsRequest specifies which items to migrate.
type SelectItemsRequest struct {
	Schema         bool     `json:"schema"`
	Data           []string `json:"data"`            // Table names
	Users          bool     `json:"users"`
	Identities     bool     `json:"identities"`
	RLS            bool     `json:"rls"`
	StorageBuckets bool     `json:"storage_buckets"`
	StorageFiles   []string `json:"storage_files"`   // Bucket IDs
	Functions      []string `json:"functions"`       // Function names
	Secrets        bool     `json:"secrets"`
	AuthConfig     bool     `json:"auth_config"`
	OAuthConfig    bool     `json:"oauth_config"`
	EmailTemplates bool     `json:"email_templates"`
}

// SelectItems creates migration items based on selection.
func (s *Service) SelectItems(migrationID string, req SelectItemsRequest) error {
	m, err := s.state.GetMigration(migrationID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("migration not found")
	}

	// Create items based on selection
	if req.Schema {
		if _, err := s.state.CreateItem(migrationID, ItemSchema, ""); err != nil {
			return err
		}
	}

	for _, table := range req.Data {
		if _, err := s.state.CreateItem(migrationID, ItemData, table); err != nil {
			return err
		}
	}

	if req.Users {
		if _, err := s.state.CreateItem(migrationID, ItemUsers, ""); err != nil {
			return err
		}
	}

	if req.Identities {
		if _, err := s.state.CreateItem(migrationID, ItemIdentities, ""); err != nil {
			return err
		}
	}

	if req.RLS {
		if _, err := s.state.CreateItem(migrationID, ItemRLS, ""); err != nil {
			return err
		}
	}

	if req.StorageBuckets {
		if _, err := s.state.CreateItem(migrationID, ItemStorageBuckets, ""); err != nil {
			return err
		}
	}

	for _, bucket := range req.StorageFiles {
		if _, err := s.state.CreateItem(migrationID, ItemStorageFiles, bucket); err != nil {
			return err
		}
	}

	for _, fn := range req.Functions {
		if _, err := s.state.CreateItem(migrationID, ItemFunctions, fn); err != nil {
			return err
		}
	}

	if req.Secrets {
		if _, err := s.state.CreateItem(migrationID, ItemSecrets, ""); err != nil {
			return err
		}
	}

	if req.AuthConfig {
		if _, err := s.state.CreateItem(migrationID, ItemAuthConfig, ""); err != nil {
			return err
		}
	}

	if req.OAuthConfig {
		if _, err := s.state.CreateItem(migrationID, ItemOAuthConfig, ""); err != nil {
			return err
		}
	}

	if req.EmailTemplates {
		if _, err := s.state.CreateItem(migrationID, ItemEmailTemplates, ""); err != nil {
			return err
		}
	}

	return nil
}

// GetItems retrieves all items for a migration.
func (s *Service) GetItems(migrationID string) ([]*MigrationItem, error) {
	return s.state.GetItems(migrationID)
}

// MigrationProgress represents real-time migration progress.
type MigrationProgress struct {
	MigrationID string           `json:"migration_id"`
	Status      MigrationStatus  `json:"status"`
	Items       []*MigrationItem `json:"items"`
	Summary     struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
		Failed    int `json:"failed"`
		Pending   int `json:"pending"`
	} `json:"summary"`
}

// GetProgress returns current migration progress.
func (s *Service) GetProgress(migrationID string) (*MigrationProgress, error) {
	m, err := s.state.GetMigration(migrationID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("migration not found")
	}

	items, err := s.state.GetItems(migrationID)
	if err != nil {
		return nil, err
	}

	progress := &MigrationProgress{
		MigrationID: migrationID,
		Status:      m.Status,
		Items:       items,
	}

	for _, item := range items {
		progress.Summary.Total++
		switch item.Status {
		case ItemCompleted:
			progress.Summary.Completed++
		case ItemFailed:
			progress.Summary.Failed++
		case ItemPending, ItemInProgress:
			progress.Summary.Pending++
		}
	}

	return progress, nil
}
```

**Step 2: Commit**

```bash
git add internal/dashboard/migration/service.go
git commit -m "feat(migration): add migration service core"
```

---

## Phase 5: Continue with remaining tasks...

The implementation plan continues with:

- **Phase 5:** Migration Execution (RunMigration, individual migrators)
- **Phase 6:** Rollback Implementation
- **Phase 7:** Verification System
- **Phase 8:** Dashboard API Handlers
- **Phase 9:** Frontend UI (migration.js, migration.css)
- **Phase 10:** Documentation (migrating-to-supabase.md, README updates)
- **Phase 11:** Integration Tests

---

## Quick Reference: All New Files

```
internal/dashboard/migration/
├── service.go           # Migration orchestrator
├── service_test.go      # Service tests
├── state.go             # State persistence
├── state_test.go        # State tests
├── supabase_client.go   # Management API client
├── supabase_client_test.go
├── rollback.go          # Rollback operations
├── exporters/
│   ├── schema.go        # Schema exporter
│   ├── data.go          # Data exporter
│   ├── auth.go          # Auth exporter
│   ├── storage.go       # Storage exporter
│   ├── functions.go     # Functions exporter
│   └── rls.go           # RLS exporter
└── verification/
    ├── basic.go         # Basic checks
    ├── integrity.go     # Data integrity
    └── functional.go    # Functional tests

internal/dashboard/static/
├── migration.js         # Frontend UI
└── migration.css        # Styles

docs/
└── migrating-to-supabase.md  # User documentation
```

## Quick Reference: All Modified Files

```
internal/db/migrations.go          # Add migration tables
internal/dashboard/handler.go      # Add export + migration API endpoints
internal/dashboard/static/app.js   # Add navigation to Export & Migration
internal/dashboard/static/index.html # Add tab
README.md                          # Add docs link
CLAUDE.md                          # Add API reference
```

