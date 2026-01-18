package functions

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS _functions_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS _functions_secrets (
			name TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS _functions_metadata (
			name TEXT PRIMARY KEY,
			verify_jwt INTEGER DEFAULT 1,
			memory_mb INTEGER,
			timeout_ms INTEGER,
			import_map TEXT,
			env_vars TEXT DEFAULT '{}',
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestStoreConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db, "test-secret")

	// Test GetConfig for non-existent key
	value, err := store.GetConfig("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Errorf("expected empty string for nonexistent key, got %q", value)
	}

	// Test SetConfig and GetConfig
	if err := store.SetConfig("functions_dir", "./my-functions"); err != nil {
		t.Fatal(err)
	}

	value, err = store.GetConfig("functions_dir")
	if err != nil {
		t.Fatal(err)
	}
	if value != "./my-functions" {
		t.Errorf("expected './my-functions', got %q", value)
	}

	// Test updating config
	if err := store.SetConfig("functions_dir", "./updated-functions"); err != nil {
		t.Fatal(err)
	}

	value, err = store.GetConfig("functions_dir")
	if err != nil {
		t.Fatal(err)
	}
	if value != "./updated-functions" {
		t.Errorf("expected './updated-functions', got %q", value)
	}

	// Test GetAllConfig
	if err := store.SetConfig("runtime_port", "8081"); err != nil {
		t.Fatal(err)
	}

	allConfig, err := store.GetAllConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(allConfig) != 2 {
		t.Errorf("expected 2 config entries, got %d", len(allConfig))
	}
	if allConfig["runtime_port"] != "8081" {
		t.Errorf("expected runtime_port='8081', got %q", allConfig["runtime_port"])
	}
}

func TestStoreSecrets(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db, "test-secret-key")

	// Test SetSecret and GetSecret
	if err := store.SetSecret("API_KEY", "super-secret-value"); err != nil {
		t.Fatal(err)
	}

	value, err := store.GetSecret("API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if value != "super-secret-value" {
		t.Errorf("expected 'super-secret-value', got %q", value)
	}

	// Verify the stored value is encrypted (not plain text)
	var storedValue string
	err = db.QueryRow("SELECT value FROM _functions_secrets WHERE name = ?", "API_KEY").Scan(&storedValue)
	if err != nil {
		t.Fatal(err)
	}
	if storedValue == "super-secret-value" {
		t.Error("secret is stored in plain text!")
	}

	// Test updating secret
	if err := store.SetSecret("API_KEY", "updated-secret"); err != nil {
		t.Fatal(err)
	}

	value, err = store.GetSecret("API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if value != "updated-secret" {
		t.Errorf("expected 'updated-secret', got %q", value)
	}

	// Test ListSecrets
	if err := store.SetSecret("DB_PASSWORD", "db-pass"); err != nil {
		t.Fatal(err)
	}

	secrets, err := store.ListSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(secrets))
	}

	// Test DeleteSecret
	if err := store.DeleteSecret("API_KEY"); err != nil {
		t.Fatal(err)
	}

	_, err = store.GetSecret("API_KEY")
	if err == nil {
		t.Error("expected error getting deleted secret")
	}

	// Test deleting non-existent secret
	err = store.DeleteSecret("nonexistent")
	if err == nil {
		t.Error("expected error deleting non-existent secret")
	}

	// Test GetAllSecrets
	allSecrets, err := store.GetAllSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if len(allSecrets) != 1 {
		t.Errorf("expected 1 secret, got %d", len(allSecrets))
	}
	if allSecrets["DB_PASSWORD"] != "db-pass" {
		t.Errorf("expected DB_PASSWORD='db-pass', got %q", allSecrets["DB_PASSWORD"])
	}
}

func TestStoreMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db, "test-secret")

	// Test GetMetadata for non-existent function (returns defaults)
	meta, err := store.GetMetadata("new-func")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "new-func" {
		t.Errorf("expected name 'new-func', got %q", meta.Name)
	}
	if !meta.VerifyJWT {
		t.Error("expected VerifyJWT to be true by default")
	}

	// Test SetMetadata
	meta = &FunctionMetadata{
		Name:      "my-func",
		VerifyJWT: false,
		MemoryMB:  256,
		TimeoutMS: 30000,
		EnvVars:   map[string]string{"CUSTOM_VAR": "value"},
	}
	if err := store.SetMetadata(meta); err != nil {
		t.Fatal(err)
	}

	// Test GetMetadata
	retrieved, err := store.GetMetadata("my-func")
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.Name != "my-func" {
		t.Errorf("expected name 'my-func', got %q", retrieved.Name)
	}
	if retrieved.VerifyJWT {
		t.Error("expected VerifyJWT to be false")
	}
	if retrieved.MemoryMB != 256 {
		t.Errorf("expected MemoryMB=256, got %d", retrieved.MemoryMB)
	}
	if retrieved.TimeoutMS != 30000 {
		t.Errorf("expected TimeoutMS=30000, got %d", retrieved.TimeoutMS)
	}
	if retrieved.EnvVars["CUSTOM_VAR"] != "value" {
		t.Errorf("expected CUSTOM_VAR='value', got %q", retrieved.EnvVars["CUSTOM_VAR"])
	}

	// Test updating metadata
	meta.VerifyJWT = true
	meta.MemoryMB = 512
	if err := store.SetMetadata(meta); err != nil {
		t.Fatal(err)
	}

	retrieved, err = store.GetMetadata("my-func")
	if err != nil {
		t.Fatal(err)
	}
	if !retrieved.VerifyJWT {
		t.Error("expected VerifyJWT to be true after update")
	}
	if retrieved.MemoryMB != 512 {
		t.Errorf("expected MemoryMB=512, got %d", retrieved.MemoryMB)
	}

	// Test DeleteMetadata
	if err := store.DeleteMetadata("my-func"); err != nil {
		t.Fatal(err)
	}

	// After deletion, should get defaults again
	retrieved, err = store.GetMetadata("my-func")
	if err != nil {
		t.Fatal(err)
	}
	if !retrieved.VerifyJWT {
		t.Error("expected VerifyJWT to be true (default) after deletion")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db, "encryption-test-key")

	testCases := []string{
		"simple text",
		"text with special chars: !@#$%^&*()",
		"unicode: 你好世界 🎉",
		"",
		"a",
		string(make([]byte, 1000)), // long string
	}

	for _, original := range testCases {
		encrypted, err := store.encrypt(original)
		if err != nil {
			t.Fatalf("encrypt failed for %q: %v", original, err)
		}

		decrypted, err := store.decrypt(encrypted)
		if err != nil {
			t.Fatalf("decrypt failed for %q: %v", original, err)
		}

		if decrypted != original {
			t.Errorf("round-trip failed: expected %q, got %q", original, decrypted)
		}
	}
}

func TestDifferentSecretsProduceDifferentCiphertexts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store1 := NewStore(db, "secret-key-1")
	store2 := NewStore(db, "secret-key-2")

	plaintext := "test value"

	encrypted1, _ := store1.encrypt(plaintext)
	encrypted2, _ := store2.encrypt(plaintext)

	// Different keys should produce different ciphertexts
	if encrypted1 == encrypted2 {
		t.Error("different keys produced same ciphertext")
	}

	// Each should only decrypt with its own key
	decrypted1, err := store1.decrypt(encrypted1)
	if err != nil || decrypted1 != plaintext {
		t.Error("store1 failed to decrypt its own ciphertext")
	}

	_, err = store1.decrypt(encrypted2)
	if err == nil {
		t.Error("store1 should not decrypt store2's ciphertext")
	}
}
