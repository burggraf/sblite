package migration

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// migrationStateSchema is copied from internal/db/migrations.go for test isolation
const testMigrationStateSchema = `
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

CREATE TABLE IF NOT EXISTS _migration_verifications (
    id TEXT PRIMARY KEY,
    migration_id TEXT NOT NULL REFERENCES _migrations(id) ON DELETE CASCADE,
    layer TEXT NOT NULL CHECK (layer IN ('basic', 'integrity', 'functional')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'passed', 'failed')),
    started_at TEXT,
    completed_at TEXT,
    results TEXT
);
`

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	_, err = db.Exec(testMigrationStateSchema)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestNewStateStore(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)
	if store == nil {
		t.Fatal("NewStateStore returned nil")
	}
	if store.db != db {
		t.Error("StateStore does not hold the provided db")
	}
}

func TestCreateMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	// Verify returned migration
	if m.ID == "" {
		t.Error("expected non-empty ID")
	}
	if m.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, m.Status)
	}
	if m.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if m.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}

	// Verify in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE id = ?", m.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row in database, got %d", count)
	}
}

func TestGetMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	// Create a migration first
	created, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	// Retrieve it
	retrieved, err := store.GetMigration(created.ID)
	if err != nil {
		t.Fatalf("GetMigration failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetMigration returned nil for existing migration")
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, retrieved.ID)
	}
	if retrieved.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, retrieved.Status)
	}
}

func TestGetMigrationNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	// Try to get non-existent migration
	m, err := store.GetMigration("nonexistent-uuid")
	if err != nil {
		t.Fatalf("GetMigration should not return error for not found: %v", err)
	}
	if m != nil {
		t.Error("GetMigration should return nil for not found")
	}
}

func TestUpdateMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	// Create a migration
	m, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	// Update fields
	m.Status = StatusInProgress
	m.SupabaseProjectRef = "test-project-ref"
	m.SupabaseProjectName = "Test Project"
	m.ErrorMessage = "test error"
	m.CredentialsEncrypted = []byte("encrypted-creds")
	now := time.Now().UTC()
	m.CompletedAt = &now

	err = store.UpdateMigration(m)
	if err != nil {
		t.Fatalf("UpdateMigration failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetMigration(m.ID)
	if err != nil {
		t.Fatalf("GetMigration failed: %v", err)
	}

	if retrieved.Status != StatusInProgress {
		t.Errorf("expected status %q, got %q", StatusInProgress, retrieved.Status)
	}
	if retrieved.SupabaseProjectRef != "test-project-ref" {
		t.Errorf("expected project ref %q, got %q", "test-project-ref", retrieved.SupabaseProjectRef)
	}
	if retrieved.SupabaseProjectName != "Test Project" {
		t.Errorf("expected project name %q, got %q", "Test Project", retrieved.SupabaseProjectName)
	}
	if retrieved.ErrorMessage != "test error" {
		t.Errorf("expected error message %q, got %q", "test error", retrieved.ErrorMessage)
	}
	if string(retrieved.CredentialsEncrypted) != "encrypted-creds" {
		t.Errorf("expected credentials %q, got %q", "encrypted-creds", string(retrieved.CredentialsEncrypted))
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
}

func TestUpdateMigrationUpdatesTimestamp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	originalUpdatedAt := m.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	m.Status = StatusCompleted
	err = store.UpdateMigration(m)
	if err != nil {
		t.Fatalf("UpdateMigration failed: %v", err)
	}

	if !m.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after UpdateMigration")
	}
}

func TestListMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	// Create multiple migrations and manually set distinct timestamps
	// to test ordering (RFC3339 only has second precision)
	m1, _ := store.CreateMigration()
	m2, _ := store.CreateMigration()
	m3, _ := store.CreateMigration()

	// Manually update timestamps to ensure distinct ordering
	db.Exec("UPDATE _migrations SET created_at = '2024-01-01T10:00:00Z' WHERE id = ?", m1.ID)
	db.Exec("UPDATE _migrations SET created_at = '2024-01-01T11:00:00Z' WHERE id = ?", m2.ID)
	db.Exec("UPDATE _migrations SET created_at = '2024-01-01T12:00:00Z' WHERE id = ?", m3.ID)

	migrations, err := store.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations failed: %v", err)
	}

	if len(migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migrations))
	}

	// Should be ordered by created_at DESC (most recent first)
	if migrations[0].ID != m3.ID {
		t.Errorf("expected first migration ID %q (most recent), got %q", m3.ID, migrations[0].ID)
	}
	if migrations[1].ID != m2.ID {
		t.Errorf("expected second migration ID %q, got %q", m2.ID, migrations[1].ID)
	}
	if migrations[2].ID != m1.ID {
		t.Errorf("expected third migration ID %q (oldest), got %q", m1.ID, migrations[2].ID)
	}
}

func TestListMigrationsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	migrations, err := store.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations failed: %v", err)
	}

	// Note: In Go, nil slice and empty slice are functionally equivalent
	// len(nil) == 0, so this is the idiomatic check
	if len(migrations) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(migrations))
	}
}

func TestDeleteMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	err = store.DeleteMigration(m.ID)
	if err != nil {
		t.Fatalf("DeleteMigration failed: %v", err)
	}

	// Verify deleted
	retrieved, err := store.GetMigration(m.ID)
	if err != nil {
		t.Fatalf("GetMigration failed: %v", err)
	}
	if retrieved != nil {
		t.Error("expected migration to be deleted")
	}
}

func TestDeleteMigrationNonexistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	// Deleting non-existent migration should not error
	err := store.DeleteMigration("nonexistent-uuid")
	if err != nil {
		t.Errorf("DeleteMigration should not error for nonexistent ID: %v", err)
	}
}

func TestCreateItem(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	// Create migration first
	m, err := store.CreateMigration()
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}

	item, err := store.CreateItem(m.ID, ItemSchema, "users")
	if err != nil {
		t.Fatalf("CreateItem failed: %v", err)
	}

	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
	if item.MigrationID != m.ID {
		t.Errorf("expected migration ID %q, got %q", m.ID, item.MigrationID)
	}
	if item.ItemType != ItemSchema {
		t.Errorf("expected item type %q, got %q", ItemSchema, item.ItemType)
	}
	if item.ItemName != "users" {
		t.Errorf("expected item name %q, got %q", "users", item.ItemName)
	}
	if item.Status != ItemPending {
		t.Errorf("expected status %q, got %q", ItemPending, item.Status)
	}
}

func TestCreateItemDifferentTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	itemTypes := []ItemType{
		ItemSchema,
		ItemData,
		ItemUsers,
		ItemIdentities,
		ItemRLS,
		ItemStorageBuckets,
		ItemStorageFiles,
		ItemFunctions,
		ItemSecrets,
		ItemAuthConfig,
		ItemOAuthConfig,
		ItemEmailTemplates,
	}

	for _, itemType := range itemTypes {
		item, err := store.CreateItem(m.ID, itemType, string(itemType)+"-name")
		if err != nil {
			t.Errorf("CreateItem failed for type %q: %v", itemType, err)
		}
		if item.ItemType != itemType {
			t.Errorf("expected item type %q, got %q", itemType, item.ItemType)
		}
	}
}

func TestGetItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	// Create multiple items
	item1, _ := store.CreateItem(m.ID, ItemSchema, "table1")
	item2, _ := store.CreateItem(m.ID, ItemData, "table1")
	item3, _ := store.CreateItem(m.ID, ItemUsers, "")

	items, err := store.GetItems(m.ID)
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify items are returned (ordered by id)
	foundIDs := make(map[string]bool)
	for _, item := range items {
		foundIDs[item.ID] = true
	}
	if !foundIDs[item1.ID] || !foundIDs[item2.ID] || !foundIDs[item3.ID] {
		t.Error("not all items were returned")
	}
}

func TestGetItemsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	items, err := store.GetItems(m.ID)
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}

	// Note: In Go, nil slice and empty slice are functionally equivalent
	// len(nil) == 0, so this is the idiomatic check
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestGetItemsNonexistentMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	items, err := store.GetItems("nonexistent-uuid")
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items for nonexistent migration, got %d", len(items))
	}
}

func TestUpdateItem(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	item, _ := store.CreateItem(m.ID, ItemSchema, "users")

	// Update item fields
	now := time.Now().UTC()
	item.Status = ItemCompleted
	item.StartedAt = &now
	item.CompletedAt = &now
	item.ErrorMessage = "test error"
	item.RollbackInfo = "rollback info"
	item.Metadata = json.RawMessage(`{"table":"users","rows":100}`)

	err := store.UpdateItem(item)
	if err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	// Retrieve and verify
	items, _ := store.GetItems(m.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	retrieved := items[0]
	if retrieved.Status != ItemCompleted {
		t.Errorf("expected status %q, got %q", ItemCompleted, retrieved.Status)
	}
	if retrieved.StartedAt == nil {
		t.Error("expected non-nil StartedAt")
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
	if retrieved.ErrorMessage != "test error" {
		t.Errorf("expected error message %q, got %q", "test error", retrieved.ErrorMessage)
	}
	if retrieved.RollbackInfo != "rollback info" {
		t.Errorf("expected rollback info %q, got %q", "rollback info", retrieved.RollbackInfo)
	}
	if string(retrieved.Metadata) != `{"table":"users","rows":100}` {
		t.Errorf("expected metadata %q, got %q", `{"table":"users","rows":100}`, string(retrieved.Metadata))
	}
}

func TestGetCompletedItemsReverse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	// Create items with different statuses
	item1, _ := store.CreateItem(m.ID, ItemSchema, "table1")
	item2, _ := store.CreateItem(m.ID, ItemData, "table1")
	item3, _ := store.CreateItem(m.ID, ItemUsers, "")
	item4, _ := store.CreateItem(m.ID, ItemRLS, "")

	// Mark some as completed with different completion times
	now := time.Now().UTC()

	item1.Status = ItemCompleted
	t1 := now.Add(-3 * time.Second)
	item1.CompletedAt = &t1
	store.UpdateItem(item1)

	item2.Status = ItemCompleted
	t2 := now.Add(-1 * time.Second)
	item2.CompletedAt = &t2
	store.UpdateItem(item2)

	item3.Status = ItemFailed // Not completed
	store.UpdateItem(item3)

	item4.Status = ItemCompleted
	t4 := now.Add(-2 * time.Second)
	item4.CompletedAt = &t4
	store.UpdateItem(item4)

	items, err := store.GetCompletedItemsReverse(m.ID)
	if err != nil {
		t.Fatalf("GetCompletedItemsReverse failed: %v", err)
	}

	// Should only return completed items
	if len(items) != 3 {
		t.Fatalf("expected 3 completed items, got %d", len(items))
	}

	// Should be ordered by completed_at DESC (most recent first)
	if items[0].ID != item2.ID {
		t.Errorf("expected first item ID %q (most recent), got %q", item2.ID, items[0].ID)
	}
	if items[1].ID != item4.ID {
		t.Errorf("expected second item ID %q, got %q", item4.ID, items[1].ID)
	}
	if items[2].ID != item1.ID {
		t.Errorf("expected third item ID %q (oldest), got %q", item1.ID, items[2].ID)
	}
}

func TestGetCompletedItemsReverseEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	// Create items but don't complete any
	store.CreateItem(m.ID, ItemSchema, "table1")

	items, err := store.GetCompletedItemsReverse(m.ID)
	if err != nil {
		t.Fatalf("GetCompletedItemsReverse failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 completed items, got %d", len(items))
	}
}

func TestCreateVerification(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	v, err := store.CreateVerification(m.ID, LayerBasic)
	if err != nil {
		t.Fatalf("CreateVerification failed: %v", err)
	}

	if v.ID == "" {
		t.Error("expected non-empty ID")
	}
	if v.MigrationID != m.ID {
		t.Errorf("expected migration ID %q, got %q", m.ID, v.MigrationID)
	}
	if v.Layer != LayerBasic {
		t.Errorf("expected layer %q, got %q", LayerBasic, v.Layer)
	}
	if v.Status != VerifyPending {
		t.Errorf("expected status %q, got %q", VerifyPending, v.Status)
	}
}

func TestCreateVerificationDifferentLayers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	layers := []VerificationLayer{LayerBasic, LayerIntegrity, LayerFunctional}

	for _, layer := range layers {
		v, err := store.CreateVerification(m.ID, layer)
		if err != nil {
			t.Errorf("CreateVerification failed for layer %q: %v", layer, err)
		}
		if v.Layer != layer {
			t.Errorf("expected layer %q, got %q", layer, v.Layer)
		}
	}
}

func TestGetVerifications(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	v1, _ := store.CreateVerification(m.ID, LayerBasic)
	v2, _ := store.CreateVerification(m.ID, LayerIntegrity)
	v3, _ := store.CreateVerification(m.ID, LayerFunctional)

	verifications, err := store.GetVerifications(m.ID)
	if err != nil {
		t.Fatalf("GetVerifications failed: %v", err)
	}

	if len(verifications) != 3 {
		t.Fatalf("expected 3 verifications, got %d", len(verifications))
	}

	// Verify all are present
	foundIDs := make(map[string]bool)
	for _, v := range verifications {
		foundIDs[v.ID] = true
	}
	if !foundIDs[v1.ID] || !foundIDs[v2.ID] || !foundIDs[v3.ID] {
		t.Error("not all verifications were returned")
	}
}

func TestGetVerificationsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	verifications, err := store.GetVerifications(m.ID)
	if err != nil {
		t.Fatalf("GetVerifications failed: %v", err)
	}

	// Note: In Go, nil slice and empty slice are functionally equivalent
	// len(nil) == 0, so this is the idiomatic check
	if len(verifications) != 0 {
		t.Errorf("expected 0 verifications, got %d", len(verifications))
	}
}

func TestGetVerificationsNonexistentMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	verifications, err := store.GetVerifications("nonexistent-uuid")
	if err != nil {
		t.Fatalf("GetVerifications failed: %v", err)
	}

	if len(verifications) != 0 {
		t.Errorf("expected 0 verifications for nonexistent migration, got %d", len(verifications))
	}
}

func TestUpdateVerification(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	v, _ := store.CreateVerification(m.ID, LayerBasic)

	// Update verification fields
	now := time.Now().UTC()
	v.Status = VerifyPassed
	v.StartedAt = &now
	v.CompletedAt = &now
	v.Results = json.RawMessage(`{"checks":5,"passed":5}`)

	err := store.UpdateVerification(v)
	if err != nil {
		t.Fatalf("UpdateVerification failed: %v", err)
	}

	// Retrieve and verify
	verifications, _ := store.GetVerifications(m.ID)
	if len(verifications) != 1 {
		t.Fatalf("expected 1 verification, got %d", len(verifications))
	}

	retrieved := verifications[0]
	if retrieved.Status != VerifyPassed {
		t.Errorf("expected status %q, got %q", VerifyPassed, retrieved.Status)
	}
	if retrieved.StartedAt == nil {
		t.Error("expected non-nil StartedAt")
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
	if string(retrieved.Results) != `{"checks":5,"passed":5}` {
		t.Errorf("expected results %q, got %q", `{"checks":5,"passed":5}`, string(retrieved.Results))
	}
}

func TestVerificationStatusTransitions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	v, _ := store.CreateVerification(m.ID, LayerBasic)

	// Test all status transitions
	statuses := []VerificationStatus{VerifyPending, VerifyRunning, VerifyPassed, VerifyFailed}

	for _, status := range statuses {
		v.Status = status
		err := store.UpdateVerification(v)
		if err != nil {
			t.Errorf("UpdateVerification failed for status %q: %v", status, err)
		}

		verifications, _ := store.GetVerifications(m.ID)
		if verifications[0].Status != status {
			t.Errorf("expected status %q, got %q", status, verifications[0].Status)
		}
	}
}

func TestMigrationStatusTransitions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	statuses := []MigrationStatus{StatusPending, StatusInProgress, StatusCompleted, StatusFailed, StatusRolledBack}

	for _, status := range statuses {
		m.Status = status
		err := store.UpdateMigration(m)
		if err != nil {
			t.Errorf("UpdateMigration failed for status %q: %v", status, err)
		}

		retrieved, _ := store.GetMigration(m.ID)
		if retrieved.Status != status {
			t.Errorf("expected status %q, got %q", status, retrieved.Status)
		}
	}
}

func TestItemStatusTransitions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	item, _ := store.CreateItem(m.ID, ItemSchema, "test")

	statuses := []ItemStatus{ItemPending, ItemInProgress, ItemCompleted, ItemFailed, ItemSkipped, ItemRolledBack}

	for _, status := range statuses {
		item.Status = status
		err := store.UpdateItem(item)
		if err != nil {
			t.Errorf("UpdateItem failed for status %q: %v", status, err)
		}

		items, _ := store.GetItems(m.ID)
		if items[0].Status != status {
			t.Errorf("expected status %q, got %q", status, items[0].Status)
		}
	}
}

func TestCascadeDeleteItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	store.CreateItem(m.ID, ItemSchema, "table1")
	store.CreateItem(m.ID, ItemData, "table1")

	// Verify items exist
	items, _ := store.GetItems(m.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Delete migration
	store.DeleteMigration(m.ID)

	// Items should be cascade deleted
	items, _ = store.GetItems(m.ID)
	if len(items) != 0 {
		t.Errorf("expected items to be cascade deleted, got %d", len(items))
	}
}

func TestCascadeDeleteVerifications(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	store.CreateVerification(m.ID, LayerBasic)
	store.CreateVerification(m.ID, LayerIntegrity)

	// Verify verifications exist
	verifications, _ := store.GetVerifications(m.ID)
	if len(verifications) != 2 {
		t.Fatalf("expected 2 verifications, got %d", len(verifications))
	}

	// Delete migration
	store.DeleteMigration(m.ID)

	// Verifications should be cascade deleted
	verifications, _ = store.GetVerifications(m.ID)
	if len(verifications) != 0 {
		t.Errorf("expected verifications to be cascade deleted, got %d", len(verifications))
	}
}

func TestListMigrationsWithAllFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	m.Status = StatusCompleted
	m.SupabaseProjectRef = "test-ref"
	m.SupabaseProjectName = "Test Name"
	m.ErrorMessage = "test error"
	now := time.Now().UTC()
	m.CompletedAt = &now
	store.UpdateMigration(m)

	migrations, err := store.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations failed: %v", err)
	}

	if len(migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migrations))
	}

	retrieved := migrations[0]
	if retrieved.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, retrieved.Status)
	}
	if retrieved.SupabaseProjectRef != "test-ref" {
		t.Errorf("expected project ref %q, got %q", "test-ref", retrieved.SupabaseProjectRef)
	}
	if retrieved.SupabaseProjectName != "Test Name" {
		t.Errorf("expected project name %q, got %q", "Test Name", retrieved.SupabaseProjectName)
	}
	if retrieved.ErrorMessage != "test error" {
		t.Errorf("expected error message %q, got %q", "test error", retrieved.ErrorMessage)
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
}

func TestItemWithNullTimestamps(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	item, _ := store.CreateItem(m.ID, ItemSchema, "test")

	// Item should have null timestamps initially
	items, _ := store.GetItems(m.ID)
	if items[0].StartedAt != nil {
		t.Error("expected nil StartedAt for new item")
	}
	if items[0].CompletedAt != nil {
		t.Error("expected nil CompletedAt for new item")
	}

	// Update with only StartedAt
	now := time.Now().UTC()
	item.StartedAt = &now
	store.UpdateItem(item)

	items, _ = store.GetItems(m.ID)
	if items[0].StartedAt == nil {
		t.Error("expected non-nil StartedAt after update")
	}
	if items[0].CompletedAt != nil {
		t.Error("expected nil CompletedAt after partial update")
	}
}

func TestVerificationWithNullTimestamps(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	v, _ := store.CreateVerification(m.ID, LayerBasic)

	// Verification should have null timestamps initially
	verifications, _ := store.GetVerifications(m.ID)
	if verifications[0].StartedAt != nil {
		t.Error("expected nil StartedAt for new verification")
	}
	if verifications[0].CompletedAt != nil {
		t.Error("expected nil CompletedAt for new verification")
	}

	// Update with only StartedAt
	now := time.Now().UTC()
	v.StartedAt = &now
	store.UpdateVerification(v)

	verifications, _ = store.GetVerifications(m.ID)
	if verifications[0].StartedAt == nil {
		t.Error("expected non-nil StartedAt after update")
	}
	if verifications[0].CompletedAt != nil {
		t.Error("expected nil CompletedAt after partial update")
	}
}

func TestMigrationWithNullCompletedAt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()

	// Migration should have null CompletedAt initially
	retrieved, _ := store.GetMigration(m.ID)
	if retrieved.CompletedAt != nil {
		t.Error("expected nil CompletedAt for new migration")
	}

	// Update without setting CompletedAt
	m.Status = StatusInProgress
	store.UpdateMigration(m)

	retrieved, _ = store.GetMigration(m.ID)
	if retrieved.CompletedAt != nil {
		t.Error("expected nil CompletedAt after update without setting it")
	}
}

func TestEmptyMetadataHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	item, _ := store.CreateItem(m.ID, ItemSchema, "test")

	// Metadata should be nil initially
	items, _ := store.GetItems(m.ID)
	if items[0].Metadata != nil {
		t.Error("expected nil Metadata for new item")
	}

	// Set empty JSON object
	item.Metadata = json.RawMessage(`{}`)
	store.UpdateItem(item)

	items, _ = store.GetItems(m.ID)
	if string(items[0].Metadata) != `{}` {
		t.Errorf("expected metadata %q, got %q", `{}`, string(items[0].Metadata))
	}
}

func TestEmptyResultsHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStateStore(db)

	m, _ := store.CreateMigration()
	v, _ := store.CreateVerification(m.ID, LayerBasic)

	// Results should be nil initially
	verifications, _ := store.GetVerifications(m.ID)
	if verifications[0].Results != nil {
		t.Error("expected nil Results for new verification")
	}

	// Set empty JSON object
	v.Results = json.RawMessage(`{}`)
	store.UpdateVerification(v)

	verifications, _ = store.GetVerifications(m.ID)
	if string(verifications[0].Results) != `{}` {
		t.Errorf("expected results %q, got %q", `{}`, string(verifications[0].Results))
	}
}
