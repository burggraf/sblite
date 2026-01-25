package pgwire

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/markb/sblite/internal/pgtranslate"
)

func TestNewServer(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "basic config",
			config: Config{
				Address: ":5432",
			},
			wantErr: false,
		},
		{
			name: "with password auth",
			config: Config{
				Address:  ":5433",
				Password: "secret",
			},
			wantErr: false,
		},
		{
			name: "with no auth",
			config: Config{
				Address: ":5434",
				NoAuth:  true,
			},
			wantErr: false,
		},
		{
			name: "with logger",
			config: Config{
				Address: ":5435",
				Logger:  slog.New(slog.NewTextHandler(os.Stderr, nil)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := NewServer(db, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && srv == nil {
				t.Error("NewServer() returned nil server without error")
			}
		})
	}
}

func TestServer_PasswordAuth(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	cfg := Config{
		Address:  ":5436",
		Password: "correctpassword",
	}

	srv, err := NewServer(db, cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name     string
		password string
		wantOK   bool
	}{
		{"correct password", "correctpassword", true},
		{"wrong password", "wrongpassword", false},
		{"empty password", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := srv.passwordAuth(nil, "sblite", "user", tt.password)
			if err != nil {
				t.Errorf("passwordAuth() error = %v", err)
				return
			}
			if ok != tt.wantOK {
				t.Errorf("passwordAuth() = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestServer_PasswordAuth_NoPasswordConfigured(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create server with no password configured
	cfg := Config{
		Address:  ":5437",
		Password: "", // No password configured
	}

	srv, err := NewServer(db, cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// All authentication attempts should fail when no password is configured
	tests := []struct {
		name     string
		password string
	}{
		{"empty password attempt", ""},
		{"any password attempt", "somepassword"},
		{"complex password attempt", "s3cr3t!@#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := srv.passwordAuth(nil, "sblite", "user", tt.password)
			if err != nil {
				t.Errorf("passwordAuth() error = %v", err)
				return
			}
			if ok {
				t.Errorf("passwordAuth() should reject all attempts when no password configured, got ok=true")
			}
		})
	}
}

func TestServer_PasswordAuth_WithVerifier(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a custom verifier that accepts "secret123"
	verifier := func(password string) bool {
		return password == "secret123"
	}

	cfg := Config{
		Address:          ":5438",
		PasswordVerifier: verifier,
	}

	srv, err := NewServer(db, cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name     string
		password string
		wantOK   bool
	}{
		{"correct password via verifier", "secret123", true},
		{"wrong password via verifier", "wrongpassword", false},
		{"empty password via verifier", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := srv.passwordAuth(nil, "sblite", "user", tt.password)
			if err != nil {
				t.Errorf("passwordAuth() error = %v", err)
				return
			}
			if ok != tt.wantOK {
				t.Errorf("passwordAuth() = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestRegisterTableMetadata_WithUUIDColumns(t *testing.T) {
	// Create in-memory database with shared cache for same connection
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Ensure single connection to avoid issues with in-memory DB
	db.SetMaxOpenConns(1)

	// Create _columns table (mimicking sblite schema)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS _columns (
		table_name TEXT,
		column_name TEXT,
		pg_type TEXT,
		is_nullable INTEGER,
		default_value TEXT,
		is_primary INTEGER,
		description TEXT,
		PRIMARY KEY (table_name, column_name)
	)`)
	if err != nil {
		t.Fatalf("failed to create _columns table: %v", err)
	}

	// Verify _columns table exists
	var tblName string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='_columns'").Scan(&tblName); err != nil {
		t.Fatalf("_columns table not found: %v", err)
	}

	// Create server with the test DB
	server := &Server{db: db}

	// Simulate the exact query the user is running
	query := "create table mynewtable (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name text, age integer)"

	// Extract table name and UUID columns (what handleCreateTable does)
	tableName := pgtranslate.GetTableName(query)
	uuidColumns := pgtranslate.GetUUIDColumns(query)

	t.Logf("tableName: %q", tableName)
	t.Logf("uuidColumns: %v", uuidColumns)

	if tableName != "mynewtable" {
		t.Errorf("GetTableName() = %q, want %q", tableName, "mynewtable")
	}

	if len(uuidColumns) != 1 || uuidColumns[0] != "id" {
		t.Errorf("GetUUIDColumns() = %v, want [id]", uuidColumns)
	}

	// Translate and execute CREATE TABLE
	translated := pgtranslate.TranslateToSQLite(query)
	t.Logf("translated: %s", translated)

	_, err = db.Exec(translated)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Call registerTableMetadata (what happens after CREATE TABLE)
	ctx := context.Background()
	err = server.registerTableMetadata(ctx, tableName, uuidColumns)
	if err != nil {
		t.Fatalf("registerTableMetadata failed: %v", err)
	}

	// Check _columns table
	rows, err := db.Query(`SELECT column_name, pg_type, default_value FROM _columns WHERE table_name = 'mynewtable' ORDER BY column_name`)
	if err != nil {
		t.Fatalf("failed to query _columns: %v", err)
	}
	defer rows.Close()

	type colInfo struct {
		name         string
		pgType       string
		defaultValue string
	}
	var cols []colInfo

	for rows.Next() {
		var c colInfo
		var defVal sql.NullString
		if err := rows.Scan(&c.name, &c.pgType, &defVal); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		if defVal.Valid {
			c.defaultValue = defVal.String
		}
		cols = append(cols, c)
	}

	t.Logf("columns in _columns: %+v", cols)

	// Verify results
	expected := map[string]colInfo{
		"id":   {name: "id", pgType: "uuid", defaultValue: "gen_random_uuid()"},
		"name": {name: "name", pgType: "text", defaultValue: ""},
		"age":  {name: "age", pgType: "integer", defaultValue: ""},
	}

	if len(cols) != 3 {
		t.Errorf("expected 3 columns, got %d", len(cols))
	}

	for _, col := range cols {
		exp, ok := expected[col.name]
		if !ok {
			t.Errorf("unexpected column: %q", col.name)
			continue
		}
		if col.pgType != exp.pgType {
			t.Errorf("column %q: pg_type = %q, want %q", col.name, col.pgType, exp.pgType)
		}
		if col.defaultValue != exp.defaultValue {
			t.Errorf("column %q: default_value = %q, want %q", col.name, col.defaultValue, exp.defaultValue)
		}
	}
}
