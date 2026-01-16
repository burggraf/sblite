# Phase 2: RLS & Security Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Row Level Security (RLS) via query rewriting, email verification, password recovery, and CLI tools for multi-tenant isolation.

**Architecture:** RLS is implemented at the API layer by storing policies in `_rls_policies` table, then injecting WHERE clauses into queries at runtime. Auth functions like `auth.uid()` are replaced with actual JWT claim values before query execution.

**Tech Stack:** Go, SQLite, JWT, bcrypt, Chi router

---

## Task 1: RLS Policy Storage Table

**Files:**
- Modify: `internal/db/migrations.go`
- Test: `internal/db/migrations_test.go`

**Step 1: Add _rls_policies table to migrations**

Add to `internal/db/migrations.go` in the `migrations` slice:

```go
// RLS policies table
`CREATE TABLE IF NOT EXISTS _rls_policies (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name    TEXT NOT NULL,
    policy_name   TEXT NOT NULL,
    command       TEXT CHECK (command IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE', 'ALL')),
    using_expr    TEXT,
    check_expr    TEXT,
    enabled       INTEGER DEFAULT 1,
    created_at    TEXT DEFAULT (datetime('now')),
    UNIQUE(table_name, policy_name)
);`,
```

**Step 2: Run migrations test**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 3: Verify table creation manually**

Run: `rm -f test.db && ./sblite init --db test.db && sqlite3 test.db ".schema _rls_policies"`
Expected: Shows CREATE TABLE statement

**Step 4: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(rls): add _rls_policies table migration"
```

---

## Task 2: Policy Service - CRUD Operations

**Files:**
- Create: `internal/rls/policy.go`
- Create: `internal/rls/policy_test.go`

**Step 1: Create policy types and service**

Create `internal/rls/policy.go`:

```go
// internal/rls/policy.go
package rls

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/markb/sblite/internal/db"
)

type Policy struct {
	ID         int64     `json:"id"`
	TableName  string    `json:"table_name"`
	PolicyName string    `json:"policy_name"`
	Command    string    `json:"command"`
	UsingExpr  string    `json:"using_expr,omitempty"`
	CheckExpr  string    `json:"check_expr,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

type Service struct {
	db *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

func (s *Service) CreatePolicy(tableName, policyName, command, usingExpr, checkExpr string) (*Policy, error) {
	result, err := s.db.Exec(`
		INSERT INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr)
		VALUES (?, ?, ?, ?, ?)
	`, tableName, policyName, command, usingExpr, checkExpr)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	id, _ := result.LastInsertId()
	return s.GetPolicyByID(id)
}

func (s *Service) GetPolicyByID(id int64) (*Policy, error) {
	var p Policy
	var createdAt string
	var usingExpr, checkExpr sql.NullString

	err := s.db.QueryRow(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE id = ?
	`, id).Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &p.Enabled, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	p.UsingExpr = usingExpr.String
	p.CheckExpr = checkExpr.String
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &p, nil
}

func (s *Service) GetPoliciesForTable(tableName string) ([]*Policy, error) {
	rows, err := s.db.Query(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE table_name = ? AND enabled = 1
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*Policy
	for rows.Next() {
		var p Policy
		var createdAt string
		var usingExpr, checkExpr sql.NullString

		if err := rows.Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &p.Enabled, &createdAt); err != nil {
			return nil, err
		}
		p.UsingExpr = usingExpr.String
		p.CheckExpr = checkExpr.String
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		policies = append(policies, &p)
	}
	return policies, nil
}

func (s *Service) ListAllPolicies() ([]*Policy, error) {
	rows, err := s.db.Query(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies ORDER BY table_name, policy_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*Policy
	for rows.Next() {
		var p Policy
		var createdAt string
		var usingExpr, checkExpr sql.NullString

		if err := rows.Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &p.Enabled, &createdAt); err != nil {
			return nil, err
		}
		p.UsingExpr = usingExpr.String
		p.CheckExpr = checkExpr.String
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		policies = append(policies, &p)
	}
	return policies, nil
}

func (s *Service) DeletePolicy(id int64) error {
	_, err := s.db.Exec("DELETE FROM _rls_policies WHERE id = ?", id)
	return err
}
```

**Step 2: Write tests**

Create `internal/rls/policy_test.go`:

```go
// internal/rls/policy_test.go
package rls

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database
}

func TestCreatePolicy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	policy, err := service.CreatePolicy("todos", "user_isolation", "ALL", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	if policy.TableName != "todos" {
		t.Errorf("expected table_name 'todos', got %s", policy.TableName)
	}
	if policy.PolicyName != "user_isolation" {
		t.Errorf("expected policy_name 'user_isolation', got %s", policy.PolicyName)
	}
	if policy.UsingExpr != "user_id = auth.uid()" {
		t.Errorf("expected using_expr 'user_id = auth.uid()', got %s", policy.UsingExpr)
	}
}

func TestGetPoliciesForTable(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	// Create two policies for same table
	service.CreatePolicy("todos", "policy1", "SELECT", "user_id = auth.uid()", "")
	service.CreatePolicy("todos", "policy2", "INSERT", "", "user_id = auth.uid()")
	service.CreatePolicy("other_table", "policy3", "ALL", "true", "")

	policies, err := service.GetPoliciesForTable("todos")
	if err != nil {
		t.Fatalf("failed to get policies: %v", err)
	}

	if len(policies) != 2 {
		t.Errorf("expected 2 policies for 'todos', got %d", len(policies))
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/rls/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/rls/
git commit -m "feat(rls): add policy service with CRUD operations"
```

---

## Task 3: Auth Function Substitution

**Files:**
- Create: `internal/rls/rewriter.go`
- Create: `internal/rls/rewriter_test.go`

**Step 1: Create rewriter with auth function substitution**

Create `internal/rls/rewriter.go`:

```go
// internal/rls/rewriter.go
package rls

import (
	"regexp"
	"strings"
)

// AuthContext holds JWT claims for auth function substitution
type AuthContext struct {
	UserID   string
	Email    string
	Role     string
	Claims   map[string]any
}

// SubstituteAuthFunctions replaces auth.uid(), auth.role(), etc. with actual values
func SubstituteAuthFunctions(expr string, ctx *AuthContext) string {
	if ctx == nil {
		return expr
	}

	// Replace auth.uid()
	expr = strings.ReplaceAll(expr, "auth.uid()", "'"+escapeSQLString(ctx.UserID)+"'")

	// Replace auth.role()
	expr = strings.ReplaceAll(expr, "auth.role()", "'"+escapeSQLString(ctx.Role)+"'")

	// Replace auth.email()
	expr = strings.ReplaceAll(expr, "auth.email()", "'"+escapeSQLString(ctx.Email)+"'")

	// Replace auth.jwt()->>'key' patterns
	jwtPattern := regexp.MustCompile(`auth\.jwt\(\)->>'\s*(\w+)\s*'`)
	expr = jwtPattern.ReplaceAllStringFunc(expr, func(match string) string {
		submatches := jwtPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]
		if val, ok := ctx.Claims[key]; ok {
			switch v := val.(type) {
			case string:
				return "'" + escapeSQLString(v) + "'"
			default:
				return "'" + escapeSQLString(toString(v)) + "'"
			}
		}
		return "NULL"
	})

	return expr
}

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(fmt.Sprintf("%f", val), ".", ",", 1), "0"), ",")
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}
```

Note: Add `"fmt"` to imports.

**Step 2: Write tests**

Create `internal/rls/rewriter_test.go`:

```go
// internal/rls/rewriter_test.go
package rls

import (
	"testing"
)

func TestSubstituteAuthFunctions(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "authenticated",
		Claims: map[string]any{
			"custom_claim": "custom_value",
		},
	}

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "auth.uid()",
			expr:     "user_id = auth.uid()",
			expected: "user_id = 'user-123'",
		},
		{
			name:     "auth.role()",
			expr:     "role = auth.role()",
			expected: "role = 'authenticated'",
		},
		{
			name:     "auth.email()",
			expr:     "email = auth.email()",
			expected: "email = 'test@example.com'",
		},
		{
			name:     "auth.jwt()->>'key'",
			expr:     "custom = auth.jwt()->>'custom_claim'",
			expected: "custom = 'custom_value'",
		},
		{
			name:     "multiple substitutions",
			expr:     "user_id = auth.uid() AND role = auth.role()",
			expected: "user_id = 'user-123' AND role = 'authenticated'",
		},
		{
			name:     "SQL injection prevention",
			expr:     "user_id = auth.uid()",
			expected: "user_id = 'user-123'", // With UserID = "user'; DROP TABLE--" it would escape
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteAuthFunctions(tt.expr, ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSubstituteAuthFunctionsWithSQLInjection(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user'; DROP TABLE users; --",
		Email:  "test@example.com",
		Role:   "authenticated",
	}

	result := SubstituteAuthFunctions("user_id = auth.uid()", ctx)
	expected := "user_id = 'user''; DROP TABLE users; --'"

	if result != expected {
		t.Errorf("SQL injection not properly escaped: got %q", result)
	}
}

func TestSubstituteAuthFunctionsNilContext(t *testing.T) {
	result := SubstituteAuthFunctions("user_id = auth.uid()", nil)
	if result != "user_id = auth.uid()" {
		t.Errorf("expected unchanged expression with nil context, got %q", result)
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/rls/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/rls/rewriter.go internal/rls/rewriter_test.go
git commit -m "feat(rls): add auth function substitution in expressions"
```

---

## Task 4: Query Rewriting for SELECT

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/server/server.go`
- Create: `internal/rls/enforcer.go`
- Test: `internal/rls/enforcer_test.go`

**Step 1: Create RLS enforcer**

Create `internal/rls/enforcer.go`:

```go
// internal/rls/enforcer.go
package rls

import (
	"strings"
)

// Enforcer applies RLS policies to queries
type Enforcer struct {
	policyService *Service
}

func NewEnforcer(policyService *Service) *Enforcer {
	return &Enforcer{policyService: policyService}
}

// GetSelectConditions returns WHERE conditions for SELECT queries
func (e *Enforcer) GetSelectConditions(tableName string, ctx *AuthContext) (string, error) {
	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", err
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "SELECT" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := SubstituteAuthFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil // No RLS policies, allow all
	}

	// AND all conditions together (all policies must pass)
	return strings.Join(conditions, " AND "), nil
}

// GetInsertConditions returns CHECK conditions for INSERT queries
func (e *Enforcer) GetInsertConditions(tableName string, ctx *AuthContext) (string, error) {
	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", err
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "INSERT" || p.Command == "ALL" {
			if p.CheckExpr != "" {
				substituted := SubstituteAuthFunctions(p.CheckExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), nil
}

// GetUpdateConditions returns WHERE conditions for UPDATE queries
func (e *Enforcer) GetUpdateConditions(tableName string, ctx *AuthContext) (string, error) {
	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", err
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "UPDATE" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := SubstituteAuthFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), nil
}

// GetDeleteConditions returns WHERE conditions for DELETE queries
func (e *Enforcer) GetDeleteConditions(tableName string, ctx *AuthContext) (string, error) {
	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", err
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "DELETE" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := SubstituteAuthFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), nil
}
```

**Step 2: Write enforcer tests**

Create `internal/rls/enforcer_test.go`:

```go
// internal/rls/enforcer_test.go
package rls

import (
	"testing"
)

func TestEnforcerSelectConditions(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create a SELECT policy
	policyService.CreatePolicy("todos", "user_isolation", "SELECT", "user_id = auth.uid()", "")

	ctx := &AuthContext{
		UserID: "user-123",
		Role:   "authenticated",
	}

	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	expected := "(user_id = 'user-123')"
	if conditions != expected {
		t.Errorf("expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerNoPolicy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	ctx := &AuthContext{UserID: "user-123"}

	conditions, err := enforcer.GetSelectConditions("no_policy_table", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	if conditions != "" {
		t.Errorf("expected empty conditions for table without policy, got %q", conditions)
	}
}

func TestEnforcerMultiplePolicies(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create multiple policies - both must pass
	policyService.CreatePolicy("todos", "user_isolation", "SELECT", "user_id = auth.uid()", "")
	policyService.CreatePolicy("todos", "not_deleted", "SELECT", "deleted = 0", "")

	ctx := &AuthContext{UserID: "user-123"}

	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	// Both conditions should be ANDed
	if !strings.Contains(conditions, "user_id = 'user-123'") {
		t.Errorf("expected user condition in %q", conditions)
	}
	if !strings.Contains(conditions, "deleted = 0") {
		t.Errorf("expected deleted condition in %q", conditions)
	}
}
```

Add `"strings"` to imports.

**Step 3: Run tests**

Run: `go test ./internal/rls/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/rls/enforcer.go internal/rls/enforcer_test.go
git commit -m "feat(rls): add policy enforcer for query conditions"
```

---

## Task 5: Integrate RLS into REST Handler

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/server/server.go`

**Step 1: Add RLS enforcer to REST handler**

Modify `internal/rest/handler.go` to accept an RLS enforcer and apply conditions:

```go
// Add to Handler struct
type Handler struct {
	db       *db.DB
	enforcer *rls.Enforcer // Add this field
}

// Update NewHandler
func NewHandler(database *db.DB, enforcer *rls.Enforcer) *Handler {
	return &Handler{db: database, enforcer: enforcer}
}
```

Modify `HandleSelect` to apply RLS:

```go
func (h *Handler) HandleSelect(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	// Get auth context from request (set by middleware)
	authCtx := GetAuthContextFromRequest(r)

	// Apply RLS conditions if enforcer is configured
	if h.enforcer != nil {
		rlsCondition, err := h.enforcer.GetSelectConditions(q.Table, authCtx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "rls_error", "Failed to apply RLS")
			return
		}
		if rlsCondition != "" {
			// Add RLS condition as a filter
			q.RLSCondition = rlsCondition
		}
	}

	// ... rest of existing code
}
```

**Step 2: Add RLSCondition to Query struct**

In `internal/rest/query.go`, add:

```go
type Query struct {
	Table        string
	Select       []string
	Filters      []Filter
	Order        []OrderBy
	Limit        int
	Offset       int
	RLSCondition string // Add this field
}
```

**Step 3: Modify BuildSelectQuery to include RLS**

In `internal/rest/builder.go`, update `BuildSelectQuery`:

```go
// After building WHERE from filters, add RLS condition
if q.RLSCondition != "" {
	if len(q.Filters) > 0 {
		sb.WriteString(" AND ")
	} else {
		sb.WriteString(" WHERE ")
	}
	sb.WriteString(q.RLSCondition)
}
```

**Step 4: Add GetAuthContextFromRequest helper**

In `internal/rest/handler.go`:

```go
import "github.com/markb/sblite/internal/rls"

// GetAuthContextFromRequest extracts auth context from request
func GetAuthContextFromRequest(r *http.Request) *rls.AuthContext {
	claims := r.Context().Value("claims")
	if claims == nil {
		return nil
	}

	claimsMap, ok := claims.(*map[string]any)
	if !ok {
		return nil
	}

	ctx := &rls.AuthContext{
		Claims: *claimsMap,
	}

	if sub, ok := (*claimsMap)["sub"].(string); ok {
		ctx.UserID = sub
	}
	if email, ok := (*claimsMap)["email"].(string); ok {
		ctx.Email = email
	}
	if role, ok := (*claimsMap)["role"].(string); ok {
		ctx.Role = role
	}

	return ctx
}
```

**Step 5: Update server.go to wire RLS**

In `internal/server/server.go`, update Server struct and initialization:

```go
type Server struct {
	router      *chi.Mux
	db          *db.DB
	authService *auth.Service
	rlsService  *rls.Service  // Add
	rlsEnforcer *rls.Enforcer // Add
}

func New(database *db.DB, jwtSecret string) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		db:          database,
		authService: auth.NewService(database, jwtSecret),
		rlsService:  rls.NewService(database),  // Add
	}
	s.rlsEnforcer = rls.NewEnforcer(s.rlsService) // Add
	s.setupMiddleware()
	s.setupRoutes()
	return s
}
```

Update route setup to pass enforcer to handler:

```go
restHandler := rest.NewHandler(s.db, s.rlsEnforcer)
```

**Step 6: Run tests**

Run: `go test ./... -v`
Expected: PASS (may need to fix import paths)

**Step 7: Commit**

```bash
git add internal/rest/ internal/server/server.go
git commit -m "feat(rls): integrate RLS enforcement into REST handler"
```

---

## Task 6: RLS for INSERT/UPDATE/DELETE

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/rest/builder.go`

**Step 1: Apply RLS to HandleUpdate**

Add RLS condition injection to `HandleUpdate` in `handler.go`:

```go
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	// Apply RLS
	if h.enforcer != nil {
		authCtx := GetAuthContextFromRequest(r)
		rlsCondition, err := h.enforcer.GetUpdateConditions(q.Table, authCtx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "rls_error", "Failed to apply RLS")
			return
		}
		if rlsCondition != "" {
			q.RLSCondition = rlsCondition
		}
	}
	// ... rest of existing code
}
```

**Step 2: Apply RLS to HandleDelete**

Similar modification to `HandleDelete`.

**Step 3: Apply RLS check to HandleInsert**

For INSERT, we need to validate the data matches the CHECK expression:

```go
func (h *Handler) HandleInsert(w http.ResponseWriter, r *http.Request) {
	// ... existing parsing code ...

	// Apply RLS CHECK for insert
	if h.enforcer != nil {
		authCtx := GetAuthContextFromRequest(r)
		checkCondition, err := h.enforcer.GetInsertConditions(table, authCtx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "rls_error", "Failed to apply RLS")
			return
		}
		// For INSERT, we validate the data after insert or reject if check fails
		// This is a simplification - full implementation would validate before insert
		q.RLSCheckCondition = checkCondition
	}
	// ... rest
}
```

**Step 4: Update BuildUpdateQuery and BuildDeleteQuery**

In `builder.go`, add RLS condition handling similar to SELECT.

**Step 5: Run tests**

Run: `go test ./... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/rest/
git commit -m "feat(rls): apply RLS to UPDATE and DELETE operations"
```

---

## Task 7: Email Verification Flow

**Files:**
- Modify: `internal/auth/user.go`
- Modify: `internal/server/auth_handlers.go`
- Create: `internal/email/sender.go` (stub for now)

**Step 1: Add verification token generation**

In `internal/auth/user.go`, add:

```go
func (s *Service) GenerateConfirmationToken(userID string) (string, error) {
	token := generateToken() // 32 random bytes, base64 encoded
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		UPDATE auth_users SET confirmation_token = ?, confirmation_sent_at = ?
		WHERE id = ?
	`, token, now, userID)

	return token, err
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *Service) VerifyEmail(token string) (*User, error) {
	var userID string
	err := s.db.QueryRow(`
		SELECT id FROM auth_users WHERE confirmation_token = ? AND deleted_at IS NULL
	`, token).Scan(&userID)

	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		UPDATE auth_users SET email_confirmed_at = ?, confirmation_token = NULL WHERE id = ?
	`, now, userID)

	if err != nil {
		return nil, err
	}

	return s.GetUserByID(userID)
}
```

**Step 2: Add verify endpoint**

In `internal/server/auth_handlers.go`:

```go
type VerifyRequest struct {
	Type  string `json:"type"`  // "signup" or "recovery"
	Token string `json:"token"`
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	// Handle both GET (URL click) and POST (API call)
	var token, verifyType string

	if r.Method == "GET" {
		token = r.URL.Query().Get("token")
		verifyType = r.URL.Query().Get("type")
	} else {
		var req VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
		token = req.Token
		verifyType = req.Type
	}

	if token == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Token is required")
		return
	}

	switch verifyType {
	case "signup", "email":
		user, err := s.authService.VerifyEmail(token)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired token")
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Email verified successfully",
			"user":    user,
		})
	default:
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid verification type")
	}
}
```

**Step 3: Register route**

In `server.go` setupRoutes():

```go
r.Post("/auth/v1/verify", s.handleVerify)
r.Get("/auth/v1/verify", s.handleVerify)
```

**Step 4: Run tests**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/ internal/server/
git commit -m "feat(auth): add email verification flow"
```

---

## Task 8: Password Recovery Flow

**Files:**
- Modify: `internal/auth/user.go`
- Modify: `internal/server/auth_handlers.go`

**Step 1: Add recovery token generation**

In `internal/auth/user.go`:

```go
func (s *Service) GenerateRecoveryToken(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var userID string
	err := s.db.QueryRow("SELECT id FROM auth_users WHERE email = ? AND deleted_at IS NULL", email).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}

	token := generateToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		UPDATE auth_users SET recovery_token = ?, recovery_sent_at = ?
		WHERE id = ?
	`, token, now, userID)

	return token, err
}

func (s *Service) ResetPassword(token, newPassword string) (*User, error) {
	var userID string
	err := s.db.QueryRow(`
		SELECT id FROM auth_users WHERE recovery_token = ? AND deleted_at IS NULL
	`, token).Scan(&userID)

	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		UPDATE auth_users SET encrypted_password = ?, recovery_token = NULL, updated_at = ?
		WHERE id = ?
	`, string(hash), now, userID)

	if err != nil {
		return nil, err
	}

	return s.GetUserByID(userID)
}
```

**Step 2: Add recover endpoint**

In `auth_handlers.go`:

```go
type RecoverRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	var req RecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate token (don't reveal if user exists)
	_, _ = s.authService.GenerateRecoveryToken(req.Email)

	// Always return success to prevent email enumeration
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If the email exists, a recovery link has been sent",
	})
}
```

**Step 3: Add password reset to verify handler**

Update `handleVerify` to handle recovery type:

```go
case "recovery":
	// Get new password from request
	newPassword := r.URL.Query().Get("password")
	if r.Method == "POST" {
		// Parse from body for POST
		// ... handle POST body for password
	}

	user, err := s.authService.ResetPassword(token, newPassword)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired token")
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"message": "Password reset successfully",
		"user":    user,
	})
```

**Step 4: Register route**

```go
r.Post("/auth/v1/recover", s.handleRecover)
```

**Step 5: Commit**

```bash
git add internal/auth/ internal/server/
git commit -m "feat(auth): add password recovery flow"
```

---

## Task 9: Settings Endpoint

**Files:**
- Modify: `internal/server/auth_handlers.go`
- Modify: `internal/server/server.go`

**Step 1: Add settings handler**

In `auth_handlers.go`:

```go
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]any{
		"external": map[string]any{
			"email":    true,
			"phone":    false,
			"google":   false,
			"github":   false,
			"facebook": false,
		},
		"disable_signup":       false,
		"mailer_autoconfirm":   true, // sblite doesn't require email confirmation by default
		"phone_autoconfirm":    false,
		"sms_provider":         "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
```

**Step 2: Register route (public, no auth required)**

In `server.go`:

```go
// Public auth routes (no auth required)
r.Get("/auth/v1/settings", s.handleSettings)
```

**Step 3: Run tests**

Run: `go test ./... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/server/
git commit -m "feat(auth): add /settings endpoint"
```

---

## Task 10: CLI - Policy Commands

**Files:**
- Create: `cmd/policy.go`

**Step 1: Create policy command**

Create `cmd/policy.go`:

```go
// cmd/policy.go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/rls"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage RLS policies",
}

var policyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new RLS policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName, _ := cmd.Flags().GetString("table")
		policyName, _ := cmd.Flags().GetString("name")
		command, _ := cmd.Flags().GetString("command")
		usingExpr, _ := cmd.Flags().GetString("using")
		checkExpr, _ := cmd.Flags().GetString("check")

		if tableName == "" || policyName == "" {
			return fmt.Errorf("--table and --name are required")
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		service := rls.NewService(database)
		policy, err := service.CreatePolicy(tableName, policyName, command, usingExpr, checkExpr)
		if err != nil {
			return fmt.Errorf("failed to create policy: %w", err)
		}

		fmt.Printf("Created policy '%s' on table '%s' (ID: %d)\n", policy.PolicyName, policy.TableName, policy.ID)
		return nil
	},
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all RLS policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		service := rls.NewService(database)
		policies, err := service.ListAllPolicies()
		if err != nil {
			return fmt.Errorf("failed to list policies: %w", err)
		}

		if len(policies) == 0 {
			fmt.Println("No policies found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTABLE\tNAME\tCOMMAND\tUSING\tCHECK")
		for _, p := range policies {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
				p.ID, p.TableName, p.PolicyName, p.Command, p.UsingExpr, p.CheckExpr)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyListCmd)

	policyAddCmd.Flags().String("table", "", "Table name")
	policyAddCmd.Flags().String("name", "", "Policy name")
	policyAddCmd.Flags().String("command", "ALL", "Command (SELECT, INSERT, UPDATE, DELETE, ALL)")
	policyAddCmd.Flags().String("using", "", "USING expression for SELECT/UPDATE/DELETE")
	policyAddCmd.Flags().String("check", "", "CHECK expression for INSERT/UPDATE")
}
```

**Step 2: Test CLI**

Run: `go build -o sblite . && ./sblite policy --help`
Expected: Shows policy subcommands

Run: `./sblite policy add --table todos --name user_isolation --using "user_id = auth.uid()"`
Expected: "Created policy 'user_isolation' on table 'todos'"

Run: `./sblite policy list`
Expected: Shows the created policy

**Step 3: Commit**

```bash
git add cmd/policy.go
git commit -m "feat(cli): add policy add/list commands"
```

---

## Task 11: CLI - User Commands

**Files:**
- Create: `cmd/user.go`

**Step 1: Create user command**

Create `cmd/user.go`:

```go
// cmd/user.go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		if email == "" || password == "" {
			return fmt.Errorf("--email and --password are required")
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		service := auth.NewService(database, "not-needed-for-create")
		user, err := service.CreateUser(email, password, nil)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		fmt.Printf("Created user: %s (ID: %s)\n", user.Email, user.ID)
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		rows, err := database.Query(`
			SELECT id, email, role, created_at FROM auth_users WHERE deleted_at IS NULL
		`)
		if err != nil {
			return fmt.Errorf("failed to query users: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tEMAIL\tROLE\tCREATED")
		for rows.Next() {
			var id, email, role, createdAt string
			if err := rows.Scan(&id, &email, &role, &createdAt); err != nil {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, email, role, createdAt)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userListCmd)

	userCreateCmd.Flags().String("email", "", "User email")
	userCreateCmd.Flags().String("password", "", "User password")
}
```

**Step 2: Test CLI**

Run: `go build -o sblite . && ./sblite user --help`
Expected: Shows user subcommands

Run: `./sblite user create --email test@example.com --password secret123`
Expected: "Created user: test@example.com"

Run: `./sblite user list`
Expected: Shows the created user

**Step 3: Commit**

```bash
git add cmd/user.go
git commit -m "feat(cli): add user create/list commands"
```

---

## Task 12: E2E Tests for RLS

**Files:**
- Create: `e2e/tests/rls/rls.test.ts`

**Step 1: Create RLS E2E test**

Create `e2e/tests/rls/rls.test.ts`:

```typescript
import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient } from '@supabase/supabase-js'
import { execSync } from 'child_process'

describe('RLS - Row Level Security', () => {
  const supabaseUrl = process.env.SBLITE_URL || 'http://localhost:8080'
  const supabaseKey = 'test-key'

  let user1Client: any
  let user2Client: any
  let user1Id: string
  let user2Id: string

  beforeAll(async () => {
    // Create test table
    execSync(`sqlite3 test.db "CREATE TABLE IF NOT EXISTS rls_test (id INTEGER PRIMARY KEY, user_id TEXT, data TEXT)"`)

    // Add RLS policy
    execSync(`./sblite policy add --table rls_test --name user_isolation --using "user_id = auth.uid()"`)

    // Create two users
    const client = createClient(supabaseUrl, supabaseKey)

    const { data: user1 } = await client.auth.signUp({
      email: 'rls-user1@test.com',
      password: 'password123'
    })
    user1Id = user1.user!.id

    const { data: user2 } = await client.auth.signUp({
      email: 'rls-user2@test.com',
      password: 'password123'
    })
    user2Id = user2.user!.id

    // Create authenticated clients
    user1Client = createClient(supabaseUrl, supabaseKey)
    await user1Client.auth.signInWithPassword({
      email: 'rls-user1@test.com',
      password: 'password123'
    })

    user2Client = createClient(supabaseUrl, supabaseKey)
    await user2Client.auth.signInWithPassword({
      email: 'rls-user2@test.com',
      password: 'password123'
    })

    // Insert test data
    await user1Client.from('rls_test').insert({ user_id: user1Id, data: 'user1 data' })
    await user2Client.from('rls_test').insert({ user_id: user2Id, data: 'user2 data' })
  })

  it('should only return rows belonging to authenticated user', async () => {
    const { data: user1Data } = await user1Client.from('rls_test').select('*')
    const { data: user2Data } = await user2Client.from('rls_test').select('*')

    expect(user1Data).toHaveLength(1)
    expect(user1Data[0].data).toBe('user1 data')

    expect(user2Data).toHaveLength(1)
    expect(user2Data[0].data).toBe('user2 data')
  })

  it('should not allow user to see other users data', async () => {
    const { data } = await user1Client
      .from('rls_test')
      .select('*')
      .eq('user_id', user2Id)

    expect(data).toHaveLength(0)
  })
})
```

**Step 2: Run RLS tests**

Run: `cd e2e && npm test -- tests/rls/`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/tests/rls/
git commit -m "test(rls): add E2E tests for row level security"
```

---

## Task 13: Final Integration & Testing

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All Go tests pass

Run: `cd e2e && npm test`
Expected: RLS tests pass, existing tests still pass

**Step 2: Manual verification**

```bash
# Start fresh
rm -f test.db && ./sblite init --db test.db

# Create a test table
sqlite3 test.db "CREATE TABLE todos (id INTEGER PRIMARY KEY, user_id TEXT, title TEXT)"

# Add RLS policy
./sblite policy add --table todos --name user_isolation --using "user_id = auth.uid()"
./sblite policy list

# Start server
./sblite serve --db test.db

# In another terminal, test with curl
# 1. Create user and get token
TOKEN=$(curl -s -X POST http://localhost:8080/auth/v1/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}' | jq -r '.access_token')

# 2. Get user ID from token
USER_ID=$(echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq -r '.sub')

# 3. Insert data with user_id
curl -X POST http://localhost:8080/rest/v1/todos \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\": \"$USER_ID\", \"title\": \"My Todo\"}"

# 4. Query - should only see own todos
curl http://localhost:8080/rest/v1/todos \
  -H "Authorization: Bearer $TOKEN"
```

**Step 3: Update CLAUDE.md**

Add Phase 2 features to Implementation Status section.

**Step 4: Final commit**

```bash
git add .
git commit -m "feat: complete Phase 2 - RLS & Security"
```

---

## Summary

Phase 2 delivers:

1. **RLS Policy Storage** - `_rls_policies` table for storing security rules
2. **Query Rewriting** - Automatic WHERE clause injection based on policies
3. **Auth Functions** - `auth.uid()`, `auth.role()`, `auth.email()`, `auth.jwt()` substitution
4. **Email Verification** - Token-based email confirmation flow
5. **Password Recovery** - Token-based password reset flow
6. **Settings Endpoint** - `/auth/v1/settings` for client configuration
7. **CLI Tools** - `sblite policy add/list` and `sblite user create/list`

**Outcome:** Multi-tenant ready with row-level isolation
