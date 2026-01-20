# RPC Functions (Phase C) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement SQL-language PostgreSQL functions callable via `/rest/v1/rpc/{name}`, fully compatible with `supabase.rpc()`.

**Architecture:** Parse CREATE FUNCTION statements, translate PostgreSQL body to SQLite using pgtranslate, store in metadata tables, execute via HTTP endpoint with parameter binding and RLS integration.

**Tech Stack:** Go, Chi router, SQLite, pgtranslate package, supabase-js for E2E tests

---

## Task 1: Add RPC Function Schema Migration

**Files:**
- Modify: `internal/db/migrations.go`

**Step 1: Add the RPC schema constant**

Add after `functionsSchema` (around line 228):

```go
// RPC functions schema for PostgreSQL-compatible stored functions
const rpcFunctionsSchema = `
-- Function definitions
CREATE TABLE IF NOT EXISTS _rpc_functions (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    language TEXT NOT NULL DEFAULT 'sql' CHECK (language IN ('sql')),
    return_type TEXT NOT NULL,
    returns_set INTEGER NOT NULL DEFAULT 0,
    volatility TEXT DEFAULT 'VOLATILE' CHECK (volatility IN ('VOLATILE', 'STABLE', 'IMMUTABLE')),
    security TEXT DEFAULT 'INVOKER' CHECK (security IN ('INVOKER', 'DEFINER')),
    source_pg TEXT NOT NULL,
    source_sqlite TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Function arguments
CREATE TABLE IF NOT EXISTS _rpc_function_args (
    id TEXT PRIMARY KEY,
    function_id TEXT NOT NULL REFERENCES _rpc_functions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    position INTEGER NOT NULL,
    default_value TEXT,
    UNIQUE(function_id, position)
);

CREATE INDEX IF NOT EXISTS idx_rpc_function_args_function ON _rpc_function_args(function_id);
`
```

**Step 2: Add migration call in RunMigrations()**

Add after `functionsSchema` execution (around line 405):

```go
	_, err = db.Exec(rpcFunctionsSchema)
	if err != nil {
		return fmt.Errorf("failed to run RPC functions schema migration: %w", err)
	}
```

**Step 3: Verify migration compiles**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go build ./...`
Expected: Build succeeds

**Step 4: Test migration runs**

Run: `cd /Users/markb/dev/sblite-rpc-functions && rm -f test_rpc.db && ./sblite init --db test_rpc.db && sqlite3 test_rpc.db ".tables" | grep -q "_rpc_functions" && echo "SUCCESS" || echo "FAIL"`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(rpc): add _rpc_functions schema migration"
```

---

## Task 2: Create RPC Package Structure

**Files:**
- Create: `internal/rpc/types.go`

**Step 1: Create types file**

```go
// internal/rpc/types.go
package rpc

import "time"

// FunctionDef represents a stored function definition.
type FunctionDef struct {
	ID           string
	Name         string
	Language     string
	ReturnType   string
	ReturnsSet   bool
	Volatility   string
	Security     string
	SourcePG     string // Original PostgreSQL body
	SourceSQLite string // Translated SQLite body
	Args         []FunctionArg
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FunctionArg represents a function parameter.
type FunctionArg struct {
	ID           string
	FunctionID   string
	Name         string
	Type         string
	Position     int
	DefaultValue *string
}

// ParsedFunction is the result of parsing a CREATE FUNCTION statement.
type ParsedFunction struct {
	Name        string
	Args        []FunctionArg
	ReturnType  string
	ReturnsSet  bool
	Language    string
	Volatility  string
	Security    string
	Body        string
	OrReplace   bool
}

// ExecuteResult holds the result of function execution.
type ExecuteResult struct {
	Data      interface{} // Single value, row, or []row
	IsSet     bool        // True if RETURNS SETOF/TABLE
	IsScalar  bool        // True if single scalar value (not row)
}
```

**Step 2: Verify compiles**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/rpc/types.go
git commit -m "feat(rpc): add RPC types definition"
```

---

## Task 3: Implement Function Store

**Files:**
- Create: `internal/rpc/store.go`
- Create: `internal/rpc/store_test.go`

**Step 1: Write the store test**

```go
// internal/rpc/store_test.go
package rpc

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE _rpc_functions (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			language TEXT NOT NULL DEFAULT 'sql',
			return_type TEXT NOT NULL,
			returns_set INTEGER NOT NULL DEFAULT 0,
			volatility TEXT DEFAULT 'VOLATILE',
			security TEXT DEFAULT 'INVOKER',
			source_pg TEXT NOT NULL,
			source_sqlite TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		);
		CREATE TABLE _rpc_function_args (
			id TEXT PRIMARY KEY,
			function_id TEXT NOT NULL REFERENCES _rpc_functions(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			position INTEGER NOT NULL,
			default_value TEXT,
			UNIQUE(function_id, position)
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return db
}

func TestStore_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "get_user",
		Language:     "sql",
		ReturnType:   "TABLE(id TEXT, email TEXT)",
		ReturnsSet:   true,
		Volatility:   "STABLE",
		Security:     "INVOKER",
		SourcePG:     "SELECT id, email FROM users WHERE id = user_id",
		SourceSQLite: "SELECT id, email FROM users WHERE id = ?",
		Args: []FunctionArg{
			{Name: "user_id", Type: "uuid", Position: 0},
		},
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it was created
	got, err := store.Get("get_user")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Name != "get_user" {
		t.Errorf("Name = %q, want %q", got.Name, "get_user")
	}
	if len(got.Args) != 1 {
		t.Errorf("Args len = %d, want 1", len(got.Args))
	}
}

func TestStore_CreateOrReplace(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "my_func",
		Language:     "sql",
		ReturnType:   "integer",
		SourcePG:     "SELECT 1",
		SourceSQLite: "SELECT 1",
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update with CreateOrReplace
	def.SourcePG = "SELECT 2"
	def.SourceSQLite = "SELECT 2"
	err = store.CreateOrReplace(def)
	if err != nil {
		t.Fatalf("CreateOrReplace failed: %v", err)
	}

	got, _ := store.Get("my_func")
	if got.SourcePG != "SELECT 2" {
		t.Errorf("SourcePG = %q, want %q", got.SourcePG, "SELECT 2")
	}
}

func TestStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "to_delete",
		Language:     "sql",
		ReturnType:   "void",
		SourcePG:     "SELECT 1",
		SourceSQLite: "SELECT 1",
	}

	store.Create(def)

	err := store.Delete("to_delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get("to_delete")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestStore_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	store.Create(&FunctionDef{Name: "func1", Language: "sql", ReturnType: "int", SourcePG: "1", SourceSQLite: "1"})
	store.Create(&FunctionDef{Name: "func2", Language: "sql", ReturnType: "int", SourcePG: "2", SourceSQLite: "2"})

	funcs, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Errorf("List returned %d funcs, want 2", len(funcs))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go test ./internal/rpc/... -v 2>&1 | head -20`
Expected: FAIL (NewStore not defined)

**Step 3: Implement the store**

```go
// internal/rpc/store.go
package rpc

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store provides CRUD operations for function definitions.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create stores a new function definition.
func (s *Store) Create(def *FunctionDef) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO _rpc_functions (id, name, language, return_type, returns_set, volatility, security, source_pg, source_sqlite, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, def.ID, def.Name, def.Language, def.ReturnType, boolToInt(def.ReturnsSet), def.Volatility, def.Security, def.SourcePG, def.SourceSQLite, now, now)
	if err != nil {
		return fmt.Errorf("insert function: %w", err)
	}

	for _, arg := range def.Args {
		argID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO _rpc_function_args (id, function_id, name, type, position, default_value)
			VALUES (?, ?, ?, ?, ?, ?)
		`, argID, def.ID, arg.Name, arg.Type, arg.Position, arg.DefaultValue)
		if err != nil {
			return fmt.Errorf("insert arg %s: %w", arg.Name, err)
		}
	}

	return tx.Commit()
}

// CreateOrReplace creates or updates a function definition.
func (s *Store) CreateOrReplace(def *FunctionDef) error {
	existing, err := s.Get(def.Name)
	if err == nil {
		// Delete existing first
		if err := s.Delete(existing.Name); err != nil {
			return fmt.Errorf("delete existing: %w", err)
		}
	}
	return s.Create(def)
}

// Get retrieves a function by name.
func (s *Store) Get(name string) (*FunctionDef, error) {
	var def FunctionDef
	var returnsSet int

	err := s.db.QueryRow(`
		SELECT id, name, language, return_type, returns_set, volatility, security, source_pg, source_sqlite, created_at, updated_at
		FROM _rpc_functions WHERE name = ?
	`, name).Scan(&def.ID, &def.Name, &def.Language, &def.ReturnType, &returnsSet, &def.Volatility, &def.Security, &def.SourcePG, &def.SourceSQLite, &def.CreatedAt, &def.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("function %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("query function: %w", err)
	}
	def.ReturnsSet = returnsSet == 1

	// Load arguments
	rows, err := s.db.Query(`
		SELECT id, function_id, name, type, position, default_value
		FROM _rpc_function_args WHERE function_id = ? ORDER BY position
	`, def.ID)
	if err != nil {
		return nil, fmt.Errorf("query args: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var arg FunctionArg
		if err := rows.Scan(&arg.ID, &arg.FunctionID, &arg.Name, &arg.Type, &arg.Position, &arg.DefaultValue); err != nil {
			return nil, fmt.Errorf("scan arg: %w", err)
		}
		def.Args = append(def.Args, arg)
	}

	return &def, nil
}

// List returns all function definitions.
func (s *Store) List() ([]*FunctionDef, error) {
	rows, err := s.db.Query(`SELECT name FROM _rpc_functions ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query functions: %w", err)
	}
	defer rows.Close()

	var funcs []*FunctionDef
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan name: %w", err)
		}
		def, err := s.Get(name)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, def)
	}
	return funcs, nil
}

// Delete removes a function by name.
func (s *Store) Delete(name string) error {
	result, err := s.db.Exec(`DELETE FROM _rpc_functions WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete function: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("function %q not found", name)
	}
	return nil
}

// Exists checks if a function exists.
func (s *Store) Exists(name string) bool {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM _rpc_functions WHERE name = ?`, name).Scan(&count)
	return count > 0
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go test ./internal/rpc/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/rpc/store.go internal/rpc/store_test.go
git commit -m "feat(rpc): implement function store with CRUD operations"
```

---

## Task 4: Implement CREATE FUNCTION Parser

**Files:**
- Create: `internal/rpc/parser.go`
- Create: `internal/rpc/parser_test.go`

**Step 1: Write parser tests**

```go
// internal/rpc/parser_test.go
package rpc

import (
	"testing"
)

func TestParseCreateFunction_Simple(t *testing.T) {
	sql := `CREATE FUNCTION get_one() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Name != "get_one" {
		t.Errorf("Name = %q, want %q", fn.Name, "get_one")
	}
	if fn.ReturnType != "integer" {
		t.Errorf("ReturnType = %q, want %q", fn.ReturnType, "integer")
	}
	if fn.Body != "SELECT 1" {
		t.Errorf("Body = %q, want %q", fn.Body, "SELECT 1")
	}
	if fn.OrReplace {
		t.Error("OrReplace = true, want false")
	}
}

func TestParseCreateFunction_OrReplace(t *testing.T) {
	sql := `CREATE OR REPLACE FUNCTION my_func() RETURNS void LANGUAGE sql AS $$ SELECT 1 $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !fn.OrReplace {
		t.Error("OrReplace = false, want true")
	}
}

func TestParseCreateFunction_WithArgs(t *testing.T) {
	sql := `CREATE FUNCTION get_user(user_id uuid, include_deleted boolean DEFAULT false)
	        RETURNS TABLE(id uuid, email text)
	        LANGUAGE sql AS $$
	          SELECT id, email FROM users WHERE id = user_id
	        $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Name != "get_user" {
		t.Errorf("Name = %q, want %q", fn.Name, "get_user")
	}
	if len(fn.Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(fn.Args))
	}
	if fn.Args[0].Name != "user_id" {
		t.Errorf("Args[0].Name = %q, want %q", fn.Args[0].Name, "user_id")
	}
	if fn.Args[0].Type != "uuid" {
		t.Errorf("Args[0].Type = %q, want %q", fn.Args[0].Type, "uuid")
	}
	if fn.Args[1].DefaultValue == nil || *fn.Args[1].DefaultValue != "false" {
		t.Errorf("Args[1].DefaultValue unexpected")
	}
	if !fn.ReturnsSet {
		t.Error("ReturnsSet = false, want true (for TABLE)")
	}
}

func TestParseCreateFunction_ReturnsSetof(t *testing.T) {
	sql := `CREATE FUNCTION get_all_users() RETURNS SETOF users LANGUAGE sql AS $$ SELECT * FROM users $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !fn.ReturnsSet {
		t.Error("ReturnsSet = false, want true")
	}
	if fn.ReturnType != "users" {
		t.Errorf("ReturnType = %q, want %q", fn.ReturnType, "users")
	}
}

func TestParseCreateFunction_Volatility(t *testing.T) {
	sql := `CREATE FUNCTION cached_count() RETURNS integer LANGUAGE sql STABLE AS $$ SELECT count(*) FROM users $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Volatility != "STABLE" {
		t.Errorf("Volatility = %q, want %q", fn.Volatility, "STABLE")
	}
}

func TestParseCreateFunction_SecurityDefiner(t *testing.T) {
	sql := `CREATE FUNCTION admin_only() RETURNS void LANGUAGE sql SECURITY DEFINER AS $$ SELECT 1 $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Security != "DEFINER" {
		t.Errorf("Security = %q, want %q", fn.Security, "DEFINER")
	}
}

func TestParseCreateFunction_DollarTag(t *testing.T) {
	sql := `CREATE FUNCTION with_tag() RETURNS text LANGUAGE sql AS $body$ SELECT 'hello' $body$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Body != "SELECT 'hello'" {
		t.Errorf("Body = %q, want %q", fn.Body, "SELECT 'hello'")
	}
}

func TestParseCreateFunction_RejectPlpgsql(t *testing.T) {
	sql := `CREATE FUNCTION plpg() RETURNS void LANGUAGE plpgsql AS $$ BEGIN END; $$;`

	_, err := ParseCreateFunction(sql)
	if err == nil {
		t.Error("expected error for plpgsql, got nil")
	}
}

func TestIsCreateFunction(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"CREATE FUNCTION foo() RETURNS int LANGUAGE sql AS $$ SELECT 1 $$", true},
		{"CREATE OR REPLACE FUNCTION bar() RETURNS void LANGUAGE sql AS $$$$", true},
		{"  create function baz() returns text language sql as $$x$$", true},
		{"SELECT * FROM functions", false},
		{"CREATE TABLE functions (id int)", false},
		{"DROP FUNCTION foo", false},
	}

	for _, tt := range tests {
		got := IsCreateFunction(tt.sql)
		if got != tt.want {
			t.Errorf("IsCreateFunction(%q) = %v, want %v", tt.sql, got, tt.want)
		}
	}
}

func TestIsDropFunction(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"DROP FUNCTION foo", true},
		{"DROP FUNCTION IF EXISTS foo", true},
		{"drop function bar()", true},
		{"SELECT * FROM foo", false},
	}

	for _, tt := range tests {
		got := IsDropFunction(tt.sql)
		if got != tt.want {
			t.Errorf("IsDropFunction(%q) = %v, want %v", tt.sql, got, tt.want)
		}
	}
}

func TestParseDropFunction(t *testing.T) {
	tests := []struct {
		sql      string
		name     string
		ifExists bool
	}{
		{"DROP FUNCTION foo", "foo", false},
		{"DROP FUNCTION IF EXISTS bar", "bar", true},
		{"DROP FUNCTION baz()", "baz", false},
		{"DROP FUNCTION IF EXISTS qux(uuid, text)", "qux", true},
	}

	for _, tt := range tests {
		name, ifExists, err := ParseDropFunction(tt.sql)
		if err != nil {
			t.Errorf("ParseDropFunction(%q) error: %v", tt.sql, err)
			continue
		}
		if name != tt.name {
			t.Errorf("ParseDropFunction(%q) name = %q, want %q", tt.sql, name, tt.name)
		}
		if ifExists != tt.ifExists {
			t.Errorf("ParseDropFunction(%q) ifExists = %v, want %v", tt.sql, ifExists, tt.ifExists)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go test ./internal/rpc/... -run TestParse -v 2>&1 | head -20`
Expected: FAIL (ParseCreateFunction not defined)

**Step 3: Implement the parser**

```go
// internal/rpc/parser.go
package rpc

import (
	"fmt"
	"regexp"
	"strings"
)

// IsCreateFunction checks if SQL is a CREATE FUNCTION statement.
func IsCreateFunction(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "CREATE FUNCTION") ||
		strings.HasPrefix(normalized, "CREATE OR REPLACE FUNCTION")
}

// IsDropFunction checks if SQL is a DROP FUNCTION statement.
func IsDropFunction(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "DROP FUNCTION")
}

// ParseCreateFunction parses a CREATE FUNCTION statement.
func ParseCreateFunction(sql string) (*ParsedFunction, error) {
	// Normalize whitespace
	sql = strings.TrimSpace(sql)
	sql = strings.TrimSuffix(sql, ";")

	fn := &ParsedFunction{
		Language:   "sql",
		Volatility: "VOLATILE",
		Security:   "INVOKER",
	}

	// Check for OR REPLACE
	upperSQL := strings.ToUpper(sql)
	if strings.HasPrefix(upperSQL, "CREATE OR REPLACE FUNCTION") {
		fn.OrReplace = true
	}

	// Extract function name and arguments
	// Pattern: CREATE [OR REPLACE] FUNCTION name(args) RETURNS ...
	nameArgsPattern := regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(\w+)\s*\(([^)]*)\)`)
	matches := nameArgsPattern.FindStringSubmatch(sql)
	if matches == nil {
		return nil, fmt.Errorf("invalid CREATE FUNCTION syntax")
	}
	fn.Name = matches[1]
	argsStr := strings.TrimSpace(matches[2])

	// Parse arguments
	if argsStr != "" {
		args, err := parseArguments(argsStr)
		if err != nil {
			return nil, fmt.Errorf("parse arguments: %w", err)
		}
		fn.Args = args
	}

	// Extract RETURNS clause
	returnsPattern := regexp.MustCompile(`(?i)RETURNS\s+(TABLE\s*\([^)]+\)|SETOF\s+\w+|\w+)`)
	returnsMatch := returnsPattern.FindStringSubmatch(sql)
	if returnsMatch == nil {
		return nil, fmt.Errorf("missing RETURNS clause")
	}
	returnType := strings.TrimSpace(returnsMatch[1])

	// Check for SETOF or TABLE
	upperReturn := strings.ToUpper(returnType)
	if strings.HasPrefix(upperReturn, "SETOF ") {
		fn.ReturnsSet = true
		fn.ReturnType = strings.TrimSpace(returnType[6:]) // Remove "SETOF "
	} else if strings.HasPrefix(upperReturn, "TABLE") {
		fn.ReturnsSet = true
		fn.ReturnType = returnType
	} else {
		fn.ReturnType = returnType
	}

	// Extract LANGUAGE
	langPattern := regexp.MustCompile(`(?i)LANGUAGE\s+(\w+)`)
	langMatch := langPattern.FindStringSubmatch(sql)
	if langMatch != nil {
		fn.Language = strings.ToLower(langMatch[1])
	}

	// Reject non-SQL languages
	if fn.Language != "sql" {
		return nil, fmt.Errorf("only LANGUAGE sql is supported, got %q", fn.Language)
	}

	// Extract volatility
	if strings.Contains(upperSQL, "IMMUTABLE") {
		fn.Volatility = "IMMUTABLE"
	} else if strings.Contains(upperSQL, "STABLE") {
		fn.Volatility = "STABLE"
	}

	// Extract security
	if strings.Contains(upperSQL, "SECURITY DEFINER") {
		fn.Security = "DEFINER"
	}

	// Extract body from dollar-quoted string
	body, err := extractDollarQuotedBody(sql)
	if err != nil {
		return nil, fmt.Errorf("extract body: %w", err)
	}
	fn.Body = strings.TrimSpace(body)

	return fn, nil
}

// parseArguments parses function arguments from "arg1 type1, arg2 type2 DEFAULT val" format.
func parseArguments(argsStr string) ([]FunctionArg, error) {
	if strings.TrimSpace(argsStr) == "" {
		return nil, nil
	}

	var args []FunctionArg
	parts := splitArgs(argsStr)

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		arg := FunctionArg{Position: i}

		// Check for DEFAULT
		upperPart := strings.ToUpper(part)
		defaultIdx := strings.Index(upperPart, " DEFAULT ")
		if defaultIdx != -1 {
			defaultVal := strings.TrimSpace(part[defaultIdx+9:])
			arg.DefaultValue = &defaultVal
			part = strings.TrimSpace(part[:defaultIdx])
		}

		// Split into name and type
		tokens := strings.Fields(part)
		if len(tokens) < 2 {
			return nil, fmt.Errorf("invalid argument: %q", part)
		}

		arg.Name = tokens[0]
		arg.Type = strings.Join(tokens[1:], " ")

		args = append(args, arg)
	}

	return args, nil
}

// splitArgs splits arguments respecting parentheses (for types like TABLE(...)).
func splitArgs(s string) []string {
	var result []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// extractDollarQuotedBody extracts the body from $$ ... $$ or $tag$ ... $tag$.
func extractDollarQuotedBody(sql string) (string, error) {
	// Find opening dollar quote
	dollarPattern := regexp.MustCompile(`\$(\w*)\$`)
	matches := dollarPattern.FindAllStringIndex(sql, -1)
	if len(matches) < 2 {
		return "", fmt.Errorf("missing dollar-quoted body")
	}

	// Get the tag
	openMatch := dollarPattern.FindStringSubmatch(sql[matches[0][0]:matches[0][1]])
	tag := openMatch[1]
	openDelim := "$" + tag + "$"

	// Find matching close
	openIdx := matches[0][1]
	closePattern := regexp.MustCompile(regexp.QuoteMeta(openDelim))
	closeMatches := closePattern.FindStringIndex(sql[openIdx:])
	if closeMatches == nil {
		return "", fmt.Errorf("unclosed dollar quote")
	}

	body := sql[openIdx : openIdx+closeMatches[0]]
	return body, nil
}

// ParseDropFunction parses DROP FUNCTION [IF EXISTS] name[(args)].
func ParseDropFunction(sql string) (name string, ifExists bool, err error) {
	sql = strings.TrimSpace(sql)
	sql = strings.TrimSuffix(sql, ";")

	upperSQL := strings.ToUpper(sql)
	if strings.Contains(upperSQL, "IF EXISTS") {
		ifExists = true
	}

	// Extract function name
	pattern := regexp.MustCompile(`(?i)DROP\s+FUNCTION\s+(?:IF\s+EXISTS\s+)?(\w+)`)
	matches := pattern.FindStringSubmatch(sql)
	if matches == nil {
		return "", false, fmt.Errorf("invalid DROP FUNCTION syntax")
	}

	return matches[1], ifExists, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go test ./internal/rpc/... -run TestParse -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/rpc/parser.go internal/rpc/parser_test.go
git commit -m "feat(rpc): implement CREATE FUNCTION parser"
```

---

## Task 5: Implement Function Executor

**Files:**
- Create: `internal/rpc/executor.go`
- Create: `internal/rpc/executor_test.go`

**Step 1: Write executor tests**

```go
// internal/rpc/executor_test.go
package rpc

import (
	"database/sql"
	"testing"
)

func setupExecutorTestDB(t *testing.T) *sql.DB {
	db := setupTestDB(t)

	// Create test table
	_, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, active INTEGER DEFAULT 1)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, email) VALUES ('u1', 'a@test.com'), ('u2', 'b@test.com')`)
	if err != nil {
		t.Fatalf("insert data: %v", err)
	}

	return db
}

func TestExecutor_ScalarReturn(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "count_users",
		Language:     "sql",
		ReturnType:   "integer",
		SourcePG:     "SELECT count(*) FROM users",
		SourceSQLite: "SELECT count(*) FROM users",
	})

	result, err := exec.Execute("count_users", nil, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsScalar {
		t.Error("expected scalar result")
	}
	// Result should be 2
	if result.Data != int64(2) {
		t.Errorf("Data = %v, want 2", result.Data)
	}
}

func TestExecutor_TableReturn(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "get_all_users",
		Language:     "sql",
		ReturnType:   "TABLE(id TEXT, email TEXT)",
		ReturnsSet:   true,
		SourcePG:     "SELECT id, email FROM users",
		SourceSQLite: "SELECT id, email FROM users",
	})

	result, err := exec.Execute("get_all_users", nil, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsSet {
		t.Error("expected set result")
	}
	rows, ok := result.Data.([]map[string]interface{})
	if !ok {
		t.Fatalf("Data type = %T, want []map[string]interface{}", result.Data)
	}
	if len(rows) != 2 {
		t.Errorf("len(rows) = %d, want 2", len(rows))
	}
}

func TestExecutor_WithParameter(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "get_user_by_id",
		Language:     "sql",
		ReturnType:   "TABLE(id TEXT, email TEXT)",
		ReturnsSet:   true,
		SourcePG:     "SELECT id, email FROM users WHERE id = user_id",
		SourceSQLite: "SELECT id, email FROM users WHERE id = :user_id",
		Args:         []FunctionArg{{Name: "user_id", Type: "text", Position: 0}},
	})

	result, err := exec.Execute("get_user_by_id", map[string]interface{}{"user_id": "u1"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	rows := result.Data.([]map[string]interface{})
	if len(rows) != 1 {
		t.Errorf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0]["email"] != "a@test.com" {
		t.Errorf("email = %v, want a@test.com", rows[0]["email"])
	}
}

func TestExecutor_MissingRequiredArg(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "needs_arg",
		Language:     "sql",
		ReturnType:   "integer",
		SourceSQLite: "SELECT 1",
		Args:         []FunctionArg{{Name: "required_arg", Type: "text", Position: 0}},
	})

	_, err := exec.Execute("needs_arg", nil, nil)
	if err == nil {
		t.Error("expected error for missing arg")
	}
}

func TestExecutor_DefaultArg(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	defaultVal := "10"
	store.Create(&FunctionDef{
		Name:         "with_default",
		Language:     "sql",
		ReturnType:   "integer",
		SourceSQLite: "SELECT :limit_val",
		Args:         []FunctionArg{{Name: "limit_val", Type: "integer", Position: 0, DefaultValue: &defaultVal}},
	})

	result, err := exec.Execute("with_default", nil, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Data != "10" {
		t.Errorf("Data = %v, want 10", result.Data)
	}
}

func TestExecutor_FunctionNotFound(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	_, err := exec.Execute("nonexistent", nil, nil)
	if err == nil {
		t.Error("expected error for nonexistent function")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go test ./internal/rpc/... -run TestExecutor -v 2>&1 | head -20`
Expected: FAIL (NewExecutor not defined)

**Step 3: Implement the executor**

```go
// internal/rpc/executor.go
package rpc

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/markb/sblite/internal/rls"
)

// Executor executes stored functions.
type Executor struct {
	db    *sql.DB
	store *Store
}

// NewExecutor creates a new Executor.
func NewExecutor(db *sql.DB, store *Store) *Executor {
	return &Executor{db: db, store: store}
}

// Execute runs a function with the given arguments.
func (e *Executor) Execute(name string, args map[string]interface{}, authCtx *rls.AuthContext) (*ExecuteResult, error) {
	// Get function definition
	fn, err := e.store.Get(name)
	if err != nil {
		return nil, fmt.Errorf("function %q not found", name)
	}

	// Validate and bind arguments
	boundArgs, err := e.bindArguments(fn, args)
	if err != nil {
		return nil, err
	}

	// Prepare SQL with parameter substitution
	sqlStr := fn.SourceSQLite
	var sqlArgs []interface{}

	// Replace named parameters with positional ones
	for _, arg := range fn.Args {
		placeholder := ":" + arg.Name
		if strings.Contains(sqlStr, placeholder) {
			sqlStr = strings.ReplaceAll(sqlStr, placeholder, "?")
			sqlArgs = append(sqlArgs, boundArgs[arg.Name])
		}
	}

	// Execute the query
	rows, err := e.db.Query(sqlStr, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	// Get column info
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	// Collect results
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	// Format result based on return type
	result := &ExecuteResult{}

	if fn.ReturnsSet {
		result.IsSet = true
		result.Data = results
	} else if len(results) == 0 {
		result.Data = nil
	} else if len(columns) == 1 && !isTableReturn(fn.ReturnType) {
		// Single scalar value
		result.IsScalar = true
		result.Data = results[0][columns[0]]
	} else {
		// Single row
		result.Data = results[0]
	}

	return result, nil
}

// bindArguments validates and binds function arguments, applying defaults.
func (e *Executor) bindArguments(fn *FunctionDef, provided map[string]interface{}) (map[string]interface{}, error) {
	bound := make(map[string]interface{})

	for _, arg := range fn.Args {
		if val, ok := provided[arg.Name]; ok {
			bound[arg.Name] = val
		} else if arg.DefaultValue != nil {
			bound[arg.Name] = *arg.DefaultValue
		} else {
			return nil, fmt.Errorf("missing required argument: %s", arg.Name)
		}
	}

	return bound, nil
}

// isTableReturn checks if return type is TABLE(...).
func isTableReturn(returnType string) bool {
	return strings.HasPrefix(strings.ToUpper(returnType), "TABLE")
}

// PrepareSource converts PostgreSQL function body to SQLite-ready form.
// It replaces parameter references with named placeholders.
func PrepareSource(body string, args []FunctionArg) string {
	result := body

	// Replace bare parameter names with named placeholders
	for _, arg := range args {
		// Match word boundary to avoid partial replacements
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(arg.Name) + `\b`)
		result = pattern.ReplaceAllString(result, ":"+arg.Name)
	}

	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go test ./internal/rpc/... -run TestExecutor -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/rpc/executor.go internal/rpc/executor_test.go
git commit -m "feat(rpc): implement function executor with parameter binding"
```

---

## Task 6: Implement HTTP Handler

**Files:**
- Create: `internal/rpc/handler.go`

**Step 1: Implement the handler**

```go
// internal/rpc/handler.go
package rpc

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/rls"
)

// Handler handles RPC HTTP requests.
type Handler struct {
	executor *Executor
	store    *Store
}

// NewHandler creates a new RPC handler.
func NewHandler(executor *Executor, store *Store) *Handler {
	return &Handler{
		executor: executor,
		store:    store,
	}
}

// HandleRPC handles POST /rest/v1/rpc/{name}.
func (h *Handler) HandleRPC(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "PGRST000", "Function name required")
		return
	}

	// Parse request body for arguments
	var args map[string]interface{}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			h.writeError(w, http.StatusBadRequest, "PGRST000", "Invalid JSON body")
			return
		}
	}

	// Get auth context from request
	authCtx := getAuthContextFromRequest(r)

	// Execute the function
	result, err := h.executor.Execute(name, args, authCtx)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, "PGRST202", "Function not found: "+name)
			return
		}
		if strings.Contains(err.Error(), "missing required argument") {
			h.writeError(w, http.StatusBadRequest, "42883", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "PGRST500", err.Error())
		return
	}

	// Check Accept header for response format
	accept := r.Header.Get("Accept")
	wantSingle := strings.Contains(accept, "application/vnd.pgrst.object+json")

	// Check Prefer header for minimal return
	prefer := r.Header.Get("Prefer")
	if strings.Contains(prefer, "return=minimal") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Handle single object request
	if wantSingle && result.IsSet {
		rows, ok := result.Data.([]map[string]interface{})
		if !ok || len(rows) != 1 {
			h.writeError(w, http.StatusNotAcceptable, "PGRST116", "JSON object requested, multiple (or no) rows returned")
			return
		}
		json.NewEncoder(w).Encode(rows[0])
		return
	}

	json.NewEncoder(w).Encode(result.Data)
}

// writeError writes a PostgREST-compatible error response.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": message,
		"details": nil,
		"hint":    nil,
	})
}

// getAuthContextFromRequest extracts auth context from request.
func getAuthContextFromRequest(r *http.Request) *rls.AuthContext {
	ctx := &rls.AuthContext{}

	// Check for service_role API key - bypasses RLS
	apiKeyRole, _ := r.Context().Value("apikey_role").(string)
	if apiKeyRole == "service_role" {
		ctx.BypassRLS = true
		ctx.Role = "service_role"
		return ctx
	}

	// Extract user claims from Bearer token (set by auth middleware)
	if claims, ok := r.Context().Value("claims").(map[string]interface{}); ok {
		if sub, ok := claims["sub"].(string); ok {
			ctx.UserID = sub
		}
		if email, ok := claims["email"].(string); ok {
			ctx.Email = email
		}
		if role, ok := claims["role"].(string); ok {
			ctx.Role = role
		}
	}

	if apiKeyRole != "" {
		ctx.Role = apiKeyRole
	}

	return ctx
}
```

**Step 2: Verify compiles**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/rpc/handler.go
git commit -m "feat(rpc): implement HTTP handler for RPC endpoint"
```

---

## Task 7: Implement SQL Interceptor

**Files:**
- Create: `internal/rpc/interceptor.go`

**Step 1: Implement the interceptor**

```go
// internal/rpc/interceptor.go
package rpc

import (
	"fmt"

	"github.com/markb/sblite/internal/pgtranslate"
)

// Interceptor intercepts CREATE/DROP FUNCTION statements.
type Interceptor struct {
	store *Store
}

// NewInterceptor creates a new Interceptor.
func NewInterceptor(store *Store) *Interceptor {
	return &Interceptor{store: store}
}

// ProcessSQL processes a SQL statement, handling CREATE/DROP FUNCTION.
// Returns (result message, handled, error).
// If handled is true, the caller should not execute the SQL normally.
func (i *Interceptor) ProcessSQL(sql string, postgresMode bool) (string, bool, error) {
	// Check for CREATE FUNCTION
	if IsCreateFunction(sql) {
		return i.handleCreateFunction(sql, postgresMode)
	}

	// Check for DROP FUNCTION
	if IsDropFunction(sql) {
		return i.handleDropFunction(sql)
	}

	return "", false, nil
}

func (i *Interceptor) handleCreateFunction(sql string, postgresMode bool) (string, bool, error) {
	// Parse the CREATE FUNCTION statement
	parsed, err := ParseCreateFunction(sql)
	if err != nil {
		return "", true, fmt.Errorf("parse CREATE FUNCTION: %w", err)
	}

	// Translate the body from PostgreSQL to SQLite
	var translatedBody string
	if postgresMode {
		translatedBody = pgtranslate.Translate(parsed.Body)
	} else {
		translatedBody = parsed.Body
	}

	// Prepare the SQLite source with parameter placeholders
	sqliteSource := PrepareSource(translatedBody, parsed.Args)

	// Create the function definition
	def := &FunctionDef{
		Name:         parsed.Name,
		Language:     parsed.Language,
		ReturnType:   parsed.ReturnType,
		ReturnsSet:   parsed.ReturnsSet,
		Volatility:   parsed.Volatility,
		Security:     parsed.Security,
		SourcePG:     parsed.Body,
		SourceSQLite: sqliteSource,
		Args:         parsed.Args,
	}

	// Store the function
	var storeErr error
	if parsed.OrReplace {
		storeErr = i.store.CreateOrReplace(def)
	} else {
		if i.store.Exists(parsed.Name) {
			return "", true, fmt.Errorf("function %q already exists", parsed.Name)
		}
		storeErr = i.store.Create(def)
	}

	if storeErr != nil {
		return "", true, fmt.Errorf("store function: %w", storeErr)
	}

	return fmt.Sprintf("CREATE FUNCTION %s", parsed.Name), true, nil
}

func (i *Interceptor) handleDropFunction(sql string) (string, bool, error) {
	name, ifExists, err := ParseDropFunction(sql)
	if err != nil {
		return "", true, err
	}

	err = i.store.Delete(name)
	if err != nil {
		if ifExists {
			// IF EXISTS means don't error if not found
			return fmt.Sprintf("DROP FUNCTION %s (not found)", name), true, nil
		}
		return "", true, fmt.Errorf("drop function: %w", err)
	}

	return fmt.Sprintf("DROP FUNCTION %s", name), true, nil
}
```

**Step 2: Verify compiles**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/rpc/interceptor.go
git commit -m "feat(rpc): implement SQL interceptor for CREATE/DROP FUNCTION"
```

---

## Task 8: Integrate RPC into Server

**Files:**
- Modify: `internal/server/server.go`

**Step 1: Add RPC imports and fields**

Add import:
```go
"github.com/markb/sblite/internal/rpc"
```

Add to Server struct (around line 35):
```go
	// RPC fields
	rpcStore       *rpc.Store
	rpcExecutor    *rpc.Executor
	rpcHandler     *rpc.Handler
	rpcInterceptor *rpc.Interceptor
```

**Step 2: Initialize RPC in NewWithConfig**

Add after schema initialization (around line 110):
```go
	// Initialize RPC components
	s.rpcStore = rpc.NewStore(database.DB)
	s.rpcExecutor = rpc.NewExecutor(database.DB, s.rpcStore)
	s.rpcHandler = rpc.NewHandler(s.rpcExecutor, s.rpcStore)
	s.rpcInterceptor = rpc.NewInterceptor(s.rpcStore)
```

**Step 3: Add RPC route in setupRoutes**

Add inside the `/rest/v1` route group (around line 257, after DELETE route):
```go
		// RPC endpoint
		r.Post("/rpc/{name}", s.rpcHandler.HandleRPC)
```

**Step 4: Export interceptor for dashboard**

Add method after existing exports:
```go
// GetRPCInterceptor returns the RPC interceptor for SQL interception.
func (s *Server) GetRPCInterceptor() *rpc.Interceptor {
	return s.rpcInterceptor
}
```

**Step 5: Verify compiles**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/server/server.go
git commit -m "feat(rpc): integrate RPC handler into server routes"
```

---

## Task 9: Integrate Interceptor into Dashboard SQL

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add RPC interceptor field and setter**

Add import:
```go
"github.com/markb/sblite/internal/rpc"
```

Add field to Handler struct (around line 45):
```go
	rpcInterceptor *rpc.Interceptor
```

Add setter method:
```go
// SetRPCInterceptor sets the RPC interceptor for SQL statement handling.
func (h *Handler) SetRPCInterceptor(i *rpc.Interceptor) {
	h.rpcInterceptor = i
}
```

**Step 2: Modify handleExecuteSQL to intercept**

In `handleExecuteSQL` function (around line 2993), add interception before query execution:

After the PostgresMode translation block (around line 2999), add:
```go
	// Intercept CREATE/DROP FUNCTION statements
	if h.rpcInterceptor != nil {
		result, handled, err := h.rpcInterceptor.ProcessSQL(queryToExecute, req.PostgresMode)
		if handled {
			response.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			if err != nil {
				response.Error = err.Error()
			} else {
				response.Type = "CREATE"
				response.AffectedRows = 1
				response.RowCount = 1
				// Store result message in a way the UI can display
				response.Rows = [][]interface{}{{result}}
				response.Columns = []string{"result"}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}
```

**Step 3: Wire interceptor in server**

In `internal/server/server.go`, after dashboard handler is created (around line 119), add:
```go
	// Set RPC interceptor on dashboard handler
	s.dashboardHandler.SetRPCInterceptor(s.rpcInterceptor)
```

**Step 4: Verify compiles**

Run: `cd /Users/markb/dev/sblite-rpc-functions && go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/server/server.go
git commit -m "feat(rpc): integrate SQL interceptor into dashboard"
```

---

## Task 10: Write E2E Tests - Core RPC

**Files:**
- Create: `e2e/tests/rpc/sql-functions.test.ts`

**Step 1: Create test file**

```typescript
/**
 * RPC - SQL Functions Tests
 *
 * Tests for PostgreSQL-compatible SQL functions called via supabase.rpc()
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('RPC - SQL Functions', () => {
  let supabase: SupabaseClient
  let adminDb: any // For direct SQL execution

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Create test table and functions via SQL browser
    const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: `
          CREATE TABLE IF NOT EXISTS rpc_test_users (
            id TEXT PRIMARY KEY,
            email TEXT,
            score INTEGER DEFAULT 0
          );
          DELETE FROM rpc_test_users;
          INSERT INTO rpc_test_users (id, email, score) VALUES
            ('u1', 'alice@test.com', 100),
            ('u2', 'bob@test.com', 200),
            ('u3', 'charlie@test.com', 150);
        `,
        postgres_mode: false
      })
    })
  })

  afterAll(async () => {
    // Cleanup
    await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: `
          DROP FUNCTION IF EXISTS get_one;
          DROP FUNCTION IF EXISTS get_user_by_id;
          DROP FUNCTION IF EXISTS get_all_users;
          DROP FUNCTION IF EXISTS get_top_scorers;
          DROP FUNCTION IF EXISTS get_total_score;
          DROP TABLE IF EXISTS rpc_test_users;
        `,
        postgres_mode: true
      })
    })
  })

  describe('Scalar return functions', () => {
    it('should execute function returning single integer', async () => {
      // Create function
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_one() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_one')

      expect(error).toBeNull()
      expect(data).toBe(1)
    })

    it('should execute function with aggregation', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_total_score() RETURNS integer LANGUAGE sql AS $$ SELECT COALESCE(SUM(score), 0) FROM rpc_test_users $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_total_score')

      expect(error).toBeNull()
      expect(data).toBe(450) // 100 + 200 + 150
    })
  })

  describe('Table return functions', () => {
    it('should execute function returning TABLE', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_all_users() RETURNS TABLE(id TEXT, email TEXT, score INTEGER) LANGUAGE sql AS $$ SELECT id, email, score FROM rpc_test_users $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_all_users')

      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBe(3)
      expect(data[0]).toHaveProperty('id')
      expect(data[0]).toHaveProperty('email')
    })
  })

  describe('Functions with parameters', () => {
    it('should execute function with required parameter', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_user_by_id(user_id TEXT) RETURNS TABLE(id TEXT, email TEXT) LANGUAGE sql AS $$ SELECT id, email FROM rpc_test_users WHERE id = user_id $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_user_by_id', { user_id: 'u1' })

      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBe(1)
      expect(data[0].email).toBe('alice@test.com')
    })

    it('should execute function with default parameter', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_top_scorers(limit_count INTEGER DEFAULT 2) RETURNS TABLE(id TEXT, score INTEGER) LANGUAGE sql AS $$ SELECT id, score FROM rpc_test_users ORDER BY score DESC LIMIT limit_count $$;`,
          postgres_mode: true
        })
      })

      // Call without parameter (use default)
      const { data: data1, error: error1 } = await supabase.rpc('get_top_scorers')
      expect(error1).toBeNull()
      expect(data1.length).toBe(2)

      // Call with parameter
      const { data: data2, error: error2 } = await supabase.rpc('get_top_scorers', { limit_count: 1 })
      expect(error2).toBeNull()
      expect(data2.length).toBe(1)
    })

    it('should return error for missing required parameter', async () => {
      const { data, error } = await supabase.rpc('get_user_by_id', {})

      expect(error).not.toBeNull()
      expect(error?.message).toContain('missing required argument')
    })
  })

  describe('Error handling', () => {
    it('should return 404 for unknown function', async () => {
      const { data, error } = await supabase.rpc('nonexistent_function')

      expect(error).not.toBeNull()
      expect(error?.code).toBe('PGRST202')
    })
  })
})
```

**Step 2: Run tests**

Run: `cd /Users/markb/dev/sblite-rpc-functions/e2e && npm test -- --grep "RPC - SQL Functions"`
Expected: Tests run (may need server running)

**Step 3: Commit**

```bash
git add e2e/tests/rpc/sql-functions.test.ts
git commit -m "test(rpc): add E2E tests for core RPC functionality"
```

---

## Task 11: Write E2E Tests - Function Creation

**Files:**
- Create: `e2e/tests/rpc/function-creation.test.ts`

**Step 1: Create test file**

```typescript
/**
 * RPC - Function Creation Tests
 *
 * Tests for CREATE/DROP FUNCTION via SQL browser
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { TEST_CONFIG } from '../../setup/global-setup'

async function executeSql(query: string, postgresMode = true) {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, postgres_mode: postgresMode })
  })
  return response.json()
}

describe('RPC - Function Creation', () => {
  afterAll(async () => {
    // Cleanup all test functions
    await executeSql(`
      DROP FUNCTION IF EXISTS test_create_func;
      DROP FUNCTION IF EXISTS test_replace_func;
      DROP FUNCTION IF EXISTS test_drop_func;
    `)
  })

  describe('CREATE FUNCTION', () => {
    it('should create function via SQL browser', async () => {
      const result = await executeSql(`
        CREATE FUNCTION test_create_func() RETURNS integer LANGUAGE sql AS $$ SELECT 42 $$;
      `)

      expect(result.error).toBeUndefined()
      expect(result.rows[0][0]).toContain('CREATE FUNCTION')
    })

    it('should fail if function already exists', async () => {
      // First create
      await executeSql(`CREATE FUNCTION test_create_func() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`)

      // Second create without OR REPLACE should fail
      const result = await executeSql(`CREATE FUNCTION test_create_func() RETURNS integer LANGUAGE sql AS $$ SELECT 2 $$;`)

      expect(result.error).toBeDefined()
      expect(result.error).toContain('already exists')
    })
  })

  describe('CREATE OR REPLACE FUNCTION', () => {
    it('should update existing function', async () => {
      // Create initial
      await executeSql(`CREATE OR REPLACE FUNCTION test_replace_func() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`)

      // Replace
      const result = await executeSql(`CREATE OR REPLACE FUNCTION test_replace_func() RETURNS integer LANGUAGE sql AS $$ SELECT 2 $$;`)

      expect(result.error).toBeUndefined()
    })
  })

  describe('DROP FUNCTION', () => {
    it('should drop existing function', async () => {
      // Create first
      await executeSql(`CREATE FUNCTION test_drop_func() RETURNS void LANGUAGE sql AS $$ SELECT 1 $$;`)

      // Drop
      const result = await executeSql(`DROP FUNCTION test_drop_func;`)

      expect(result.error).toBeUndefined()
      expect(result.rows[0][0]).toContain('DROP FUNCTION')
    })

    it('should fail if function does not exist', async () => {
      const result = await executeSql(`DROP FUNCTION nonexistent_func;`)

      expect(result.error).toBeDefined()
    })

    it('should succeed with IF EXISTS when function does not exist', async () => {
      const result = await executeSql(`DROP FUNCTION IF EXISTS nonexistent_func;`)

      expect(result.error).toBeUndefined()
    })
  })

  describe('Language validation', () => {
    it('should reject LANGUAGE plpgsql', async () => {
      const result = await executeSql(`
        CREATE FUNCTION plpgsql_func() RETURNS void LANGUAGE plpgsql AS $$ BEGIN END; $$;
      `)

      expect(result.error).toBeDefined()
      expect(result.error).toContain('only LANGUAGE sql is supported')
    })
  })

  describe('PostgreSQL translation', () => {
    it('should translate NOW() in function body', async () => {
      const result = await executeSql(`
        CREATE OR REPLACE FUNCTION get_current_time() RETURNS text LANGUAGE sql AS $$ SELECT NOW() $$;
      `)

      expect(result.error).toBeUndefined()

      // Verify the function works
      const callResult = await executeSql(`SELECT * FROM get_current_time()`)
      expect(callResult.error).toBeUndefined()
    })
  })
})
```

**Step 2: Commit**

```bash
git add e2e/tests/rpc/function-creation.test.ts
git commit -m "test(rpc): add E2E tests for function creation"
```

---

## Task 12: Write Documentation

**Files:**
- Create: `docs/rpc-functions.md`
- Modify: `README.md`
- Modify: `CLAUDE.md`

**Step 1: Create documentation file**

```markdown
# PostgreSQL Functions (RPC)

**Status:** Implemented (Phase C - SQL functions only)
**Version:** 0.2.0

## Overview

sblite supports PostgreSQL-compatible SQL functions that can be called via the Supabase client's `rpc()` method. This enables server-side logic execution with parameter binding and automatic PostgreSQL-to-SQLite translation.

## Quick Start

### 1. Create a Function

In the SQL Browser (with PostgreSQL mode enabled):

```sql
CREATE FUNCTION get_active_users(min_score integer DEFAULT 0)
RETURNS TABLE(id text, email text, score integer)
LANGUAGE sql AS $$
  SELECT id, email, score
  FROM users
  WHERE active = TRUE AND score >= min_score
  ORDER BY score DESC
$$;
```

### 2. Call via Supabase Client

```typescript
const { data, error } = await supabase.rpc('get_active_users', {
  min_score: 100
})

// data = [{ id: "...", email: "...", score: 150 }, ...]
```

## Creating Functions

### Syntax

```sql
CREATE [OR REPLACE] FUNCTION name(arg1 type [DEFAULT value], ...)
RETURNS return_type
LANGUAGE sql
[VOLATILE | STABLE | IMMUTABLE]
[SECURITY INVOKER | SECURITY DEFINER]
AS $$ function_body $$;
```

### Return Types

| Type | Description | Response Format |
|------|-------------|-----------------|
| `integer`, `text`, etc. | Single scalar value | Raw value: `42` |
| `record` | Single row | Object: `{"id": "...", "name": "..."}` |
| `TABLE(col1 type, ...)` | Multiple rows | Array: `[{...}, {...}]` |
| `SETOF type` | Multiple rows of type | Array: `[{...}, {...}]` |
| `void` | No return value | `null` |

### Examples

**Scalar function:**
```sql
CREATE FUNCTION count_users() RETURNS integer LANGUAGE sql AS $$
  SELECT COUNT(*) FROM users
$$;
```

**Table function with parameters:**
```sql
CREATE FUNCTION search_users(search_term text)
RETURNS TABLE(id text, email text)
LANGUAGE sql AS $$
  SELECT id, email FROM users
  WHERE email LIKE '%' || search_term || '%'
$$;
```

**Function with default parameter:**
```sql
CREATE FUNCTION get_recent_posts(limit_count integer DEFAULT 10)
RETURNS TABLE(id text, title text, created_at text)
LANGUAGE sql AS $$
  SELECT id, title, created_at FROM posts
  ORDER BY created_at DESC
  LIMIT limit_count
$$;
```

## Calling Functions

### Via Supabase Client

```typescript
// No parameters
const { data } = await supabase.rpc('count_users')

// With parameters
const { data } = await supabase.rpc('search_users', { search_term: 'john' })

// With default parameters (omit to use default)
const { data } = await supabase.rpc('get_recent_posts')
const { data } = await supabase.rpc('get_recent_posts', { limit_count: 5 })
```

### Via HTTP

```bash
curl -X POST http://localhost:8080/rest/v1/rpc/search_users \
  -H "Content-Type: application/json" \
  -H "apikey: your-anon-key" \
  -d '{"search_term": "john"}'
```

### Via SQL Browser

Functions can be called directly in SQL:

```sql
SELECT * FROM get_active_users(100);
```

## Security

### SECURITY INVOKER (Default)

Function runs with the caller's permissions. RLS policies apply based on the authenticated user.

```sql
CREATE FUNCTION get_my_posts()
RETURNS TABLE(id text, title text)
LANGUAGE sql
SECURITY INVOKER AS $$
  SELECT id, title FROM posts WHERE user_id = auth.uid()
$$;
```

### SECURITY DEFINER

Function runs with elevated privileges, bypassing RLS. Use carefully for administrative operations.

```sql
CREATE FUNCTION admin_delete_user(target_id text)
RETURNS void
LANGUAGE sql
SECURITY DEFINER AS $$
  DELETE FROM users WHERE id = target_id
$$;
```

## PostgreSQL Translation

Function bodies support PostgreSQL syntax that's automatically translated to SQLite:

| PostgreSQL | SQLite |
|------------|--------|
| `NOW()` | `datetime('now')` |
| `CURRENT_TIMESTAMP` | `datetime('now')` |
| `TRUE` / `FALSE` | `1` / `0` |
| `gen_random_uuid()` | UUID generation expression |
| `COALESCE()`, `NULLIF()` | Same (native support) |

See [PostgreSQL Translation](./postgres-translation.md) for the complete list.

## Using in RLS Policies

Functions can be called within RLS policy expressions:

```sql
-- Create helper function
CREATE FUNCTION is_team_member(team_id text)
RETURNS boolean
LANGUAGE sql
SECURITY INVOKER AS $$
  SELECT EXISTS(
    SELECT 1 FROM team_members
    WHERE team_id = team_id AND user_id = auth.uid()
  )
$$;

-- Use in policy
CREATE POLICY team_access ON documents
FOR SELECT USING (is_team_member(team_id));
```

## Managing Functions

### List Functions

```sql
SELECT name, return_type, volatility FROM _rpc_functions;
```

### View Function Details

```sql
SELECT * FROM _rpc_functions WHERE name = 'my_function';
SELECT * FROM _rpc_function_args WHERE function_id = (
  SELECT id FROM _rpc_functions WHERE name = 'my_function'
);
```

### Drop Function

```sql
DROP FUNCTION function_name;
DROP FUNCTION IF EXISTS function_name;
```

## Limitations

### Phase C (Current)

- **SQL functions only** - `LANGUAGE plpgsql` is not supported
- **No function overloading** - Each function name must be unique
- **No OUT parameters** - Use `RETURNS TABLE` instead
- **No variadic arguments**
- **No polymorphic types**

### Planned (Phase D)

- PL/pgSQL to TypeScript transpilation
- Automatic Edge Function generation

## Error Codes

| Code | Description |
|------|-------------|
| `PGRST202` | Function not found |
| `42883` | Wrong number/type of arguments |
| `PGRST116` | Single row expected but multiple returned |

## Migration from Supabase

1. Export your functions from Supabase SQL Editor
2. Run in sblite SQL Browser with PostgreSQL mode enabled
3. Test each function via `supabase.rpc()`

Note: PL/pgSQL functions must be rewritten as SQL functions or converted to Edge Functions.

## Related Documentation

- [PostgreSQL Syntax Translation](./postgres-translation.md)
- [Row Level Security](./rls.md)
- [Edge Functions](./edge-functions.md)
```

**Step 2: Update README.md**

Add to Features section:
```markdown
- **PostgreSQL Functions (RPC)** - SQL functions callable via `supabase.rpc()` ([docs](docs/rpc-functions.md))
```

**Step 3: Update CLAUDE.md**

Add to API Endpoints table under REST API section:
```markdown
| `/rest/v1/rpc/{name}` | POST | Call RPC function |
```

Add to Implementation Status:
```markdown
- PostgreSQL Functions (RPC) - Phase C (SQL functions)
```

**Step 4: Commit**

```bash
git add docs/rpc-functions.md README.md CLAUDE.md
git commit -m "docs: add RPC functions documentation"
```

---

## Task 13: Final Integration Test

**Step 1: Build and run full test**

```bash
cd /Users/markb/dev/sblite-rpc-functions
go build -o sblite .
rm -f test.db
./sblite init --db test.db
./sblite serve --db test.db &
SERVER_PID=$!
sleep 2
cd e2e && npm test -- --grep "RPC"
kill $SERVER_PID
```

**Step 2: Verify all tests pass**

Expected: All RPC tests pass

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat(rpc): complete Phase C implementation"
```

---

## Summary

This plan implements PostgreSQL Functions (RPC) Phase C with:

1. **Schema**: `_rpc_functions` and `_rpc_function_args` tables
2. **Parser**: Handles CREATE/DROP FUNCTION SQL statements
3. **Store**: CRUD operations for function metadata
4. **Executor**: Runs functions with parameter binding
5. **Handler**: HTTP endpoint at `/rest/v1/rpc/{name}`
6. **Interceptor**: Captures CREATE/DROP FUNCTION in SQL browser
7. **E2E Tests**: Core RPC and function creation tests
8. **Documentation**: Complete usage guide

Total: 13 tasks, ~50 implementation steps
