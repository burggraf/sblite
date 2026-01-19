# PostgreSQL Function Compatibility Layer

**Status:** Design approved
**Created:** 2025-01-19
**Approach:** Phased implementation (C → D → A)

## Overview

Add PostgreSQL function (RPC) compatibility to sblite in three phases:

1. **Phase C:** SQL-language functions only (2-4 weeks)
2. **Phase D:** PL/pgSQL → TypeScript transpilation (4-6 weeks)
3. **Phase A:** Full PL/pgSQL interpreter (3-6 months, if needed)

Each phase is a complete, shippable feature with a decision point before proceeding.

---

## Phase C: SQL-Language Functions

**Timeline:** 2-4 weeks
**Compatibility:** ~40% of Supabase RPC use cases

### Scope

Support `LANGUAGE sql` functions that wrap single or multiple SQL statements:

```sql
-- Supported
CREATE FUNCTION get_user(user_id uuid)
RETURNS TABLE(id uuid, email text)
LANGUAGE sql
AS $$ SELECT id, email FROM users WHERE id = user_id $$;

-- Not supported (requires Phase D or A)
CREATE FUNCTION complex(x int) RETURNS int LANGUAGE plpgsql AS $$
BEGIN
  IF x > 10 THEN RETURN x * 2; END IF;
  RETURN x;
END $$;
```

### Step C.1: Function Metadata Schema

**Files:** `internal/db/migrations.go`

Add tables to track function definitions:

```sql
CREATE TABLE IF NOT EXISTS _functions (
  id TEXT PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  language TEXT NOT NULL DEFAULT 'sql',
  return_type TEXT NOT NULL,
  returns_set INTEGER NOT NULL DEFAULT 0,  -- 1 if RETURNS SETOF/TABLE
  volatility TEXT DEFAULT 'VOLATILE',
  security TEXT DEFAULT 'INVOKER',
  source TEXT NOT NULL,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS _function_args (
  id TEXT PRIMARY KEY,
  function_id TEXT NOT NULL REFERENCES _functions(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  position INTEGER NOT NULL,
  default_value TEXT,
  UNIQUE(function_id, position)
);
```

**Completion criteria:**
- [ ] Migration adds tables on `sblite init`
- [ ] Tables queryable via SQL browser

### Step C.2: Function Store

**Files:** `internal/rpc/store.go` (new)

CRUD operations for function metadata:

```go
type FunctionDef struct {
    ID         string
    Name       string
    Language   string
    ReturnType string
    ReturnsSet bool
    Volatility string
    Security   string
    Source     string
    Args       []FunctionArg
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

type FunctionArg struct {
    Name         string
    Type         string
    Position     int
    DefaultValue *string
}

type Store struct {
    db *sql.DB
}

func (s *Store) Create(def *FunctionDef) error
func (s *Store) Get(name string) (*FunctionDef, error)
func (s *Store) List() ([]*FunctionDef, error)
func (s *Store) Delete(name string) error
```

**Completion criteria:**
- [ ] Unit tests for all CRUD operations
- [ ] Handles function overloading by argument types (future consideration)

### Step C.3: SQL Function Parser

**Files:** `internal/rpc/parser.go` (new)

Parse `CREATE FUNCTION` statements to extract metadata:

```go
type ParsedFunction struct {
    Name       string
    Args       []FunctionArg
    ReturnType string
    ReturnsSet bool
    Language   string
    Source     string
    Volatility string
    Security   string
}

func ParseCreateFunction(sql string) (*ParsedFunction, error)
```

Must handle:
- `RETURNS type` vs `RETURNS TABLE(...)` vs `RETURNS SETOF type`
- `$$ body $$` and `$tag$ body $tag$` dollar quoting
- `DEFAULT` values for arguments
- `VOLATILE`, `STABLE`, `IMMUTABLE` markers
- `SECURITY INVOKER` vs `SECURITY DEFINER`

**Completion criteria:**
- [ ] Parses standard CREATE FUNCTION syntax
- [ ] Rejects LANGUAGE plpgsql with clear error message
- [ ] Unit tests for various function signatures

### Step C.4: SQL Dialect Translator

**Files:** `internal/rpc/translator.go` (new)

Convert PostgreSQL SQL to SQLite:

```go
func TranslateSQL(pgSQL string) (string, error)
```

Translation rules:
| PostgreSQL | SQLite |
|------------|--------|
| `NOW()` | `datetime('now')` |
| `CURRENT_TIMESTAMP` | `datetime('now')` |
| `value::type` | `CAST(value AS type)` |
| `TRUE` / `FALSE` | `1` / `0` |
| `ILIKE` | `LIKE` (with COLLATE NOCASE) |
| `gen_random_uuid()` | Custom UUID generation |
| `string \|\| string` | Keep as-is (SQLite supports) |
| `COALESCE()` | Keep as-is |
| `NULLIF()` | Keep as-is |

**Completion criteria:**
- [ ] Common PostgreSQL functions translated
- [ ] Unknown functions produce clear errors
- [ ] Unit tests for each translation rule

### Step C.5: Function Executor

**Files:** `internal/rpc/executor.go` (new)

Execute SQL function with parameter binding:

```go
type Executor struct {
    db    *sql.DB
    store *Store
}

type ExecuteResult struct {
    Data   interface{}  // Single value, row, or []row
    IsSet  bool         // True if RETURNS SETOF/TABLE
}

func (e *Executor) Execute(name string, args map[string]interface{}) (*ExecuteResult, error)
```

Execution flow:
1. Look up function in store
2. Validate argument types against schema
3. Translate SQL body (PostgreSQL → SQLite)
4. Replace parameter references (`$1`, named params)
5. Execute query
6. Format result based on return type

**Completion criteria:**
- [ ] Executes simple SELECT functions
- [ ] Executes INSERT/UPDATE/DELETE with RETURNING
- [ ] Handles RETURNS TABLE correctly
- [ ] Parameter type validation works
- [ ] Integration tests with real database

### Step C.6: RPC HTTP Handler

**Files:** `internal/rpc/handler.go` (new)

HTTP endpoints matching Supabase RPC:

```go
type Handler struct {
    executor *Executor
    enforcer *rls.Enforcer  // For function-level RLS
}

// POST /rpc/v1/{name}
func (h *Handler) HandleRPC(w http.ResponseWriter, r *http.Request)
```

Request format:
```json
POST /rpc/v1/get_user
Content-Type: application/json
Authorization: Bearer <jwt>

{"user_id": "550e8400-e29b-41d4-a716-446655440000"}
```

Response format:
```json
// RETURNS single row
{"id": "...", "email": "..."}

// RETURNS SETOF/TABLE
[{"id": "...", "email": "..."}, ...]

// RETURNS scalar
42
```

**Completion criteria:**
- [ ] POST endpoint works
- [ ] JWT auth middleware applied
- [ ] Error responses match Supabase format
- [ ] Content-Type handling (JSON)

### Step C.7: Server Integration

**Files:** `internal/server/server.go`

Register RPC routes:

```go
// In setupRoutes()
s.router.Route("/rpc/v1", func(r chi.Router) {
    r.Use(s.apiKeyMiddleware)
    r.Use(s.optionalAuthMiddleware)
    r.Post("/{name}", s.rpcHandler.HandleRPC)
})
```

**Completion criteria:**
- [ ] Route registered and accessible
- [ ] Middleware chain correct
- [ ] 404 for unknown functions

### Step C.8: Admin API for Functions

**Files:** `internal/admin/handler.go`

CRUD endpoints for function management:

```
POST   /admin/v1/functions     - Create function (accepts CREATE FUNCTION SQL)
GET    /admin/v1/functions     - List all functions
GET    /admin/v1/functions/:name - Get function details
DELETE /admin/v1/functions/:name - Drop function
```

**Completion criteria:**
- [ ] Create accepts raw SQL and parses it
- [ ] List returns function signatures
- [ ] Delete removes function and metadata

### Step C.9: Dashboard Integration

**Files:** `internal/dashboard/handler.go`, `internal/dashboard/static/`

Add Functions tab to dashboard:

- List functions with signatures
- Create function with SQL editor
- View function source
- Delete function
- Test function execution (like API Console)

**Completion criteria:**
- [ ] Functions tab in navigation
- [ ] Function list view
- [ ] Create function modal with SQL editor
- [ ] Delete confirmation
- [ ] Test execution panel

### Step C.10: E2E Tests

**Files:** `e2e/tests/rpc/`

Test suite using `@supabase/supabase-js`:

```typescript
// e2e/tests/rpc/sql-functions.test.ts
describe('SQL Functions', () => {
  it('executes simple SELECT function')
  it('executes function with parameters')
  it('executes function returning TABLE')
  it('executes function with INSERT RETURNING')
  it('validates parameter types')
  it('returns 404 for unknown function')
  it('respects RLS policies')
})
```

**Completion criteria:**
- [ ] All tests passing
- [ ] Client compatibility verified

### Step C.11: Documentation

**Files:** `docs/functions.md` (new), update `CLAUDE.md`

Document:
- Supported function syntax
- Limitations (SQL only, no PL/pgSQL)
- API reference
- Migration guide from Supabase
- Examples

**Completion criteria:**
- [ ] README mentions function support
- [ ] Full documentation in docs/
- [ ] CLAUDE.md updated with new endpoints

---

## Phase C Completion Checkpoint

**Before proceeding to Phase D, verify:**

- [ ] All Step C.1-C.11 completion criteria met
- [ ] E2E tests passing
- [ ] Documentation complete
- [ ] Release tagged (e.g., v0.X.0 with SQL functions)

**Decision point:** Evaluate if Phase D is needed based on:
- User feedback on SQL-only limitations
- Specific PL/pgSQL functions that can't be migrated
- Demand for automatic transpilation

---

## Phase D: PL/pgSQL → TypeScript Transpilation

**Timeline:** 4-6 weeks (after Phase C)
**Compatibility:** ~70-80% of Supabase RPC use cases

### Scope

Automatically convert `LANGUAGE plpgsql` functions to TypeScript Edge Functions:

```sql
-- Input
CREATE FUNCTION calculate_discount(order_id uuid)
RETURNS numeric LANGUAGE plpgsql AS $$
DECLARE
  total numeric;
  discount numeric := 0;
BEGIN
  SELECT sum(price * quantity) INTO total FROM order_items WHERE order_id = order_id;
  IF total > 100 THEN
    discount := total * 0.1;
  ELSIF total > 50 THEN
    discount := total * 0.05;
  END IF;
  RETURN discount;
END $$;
```

```typescript
// Output: functions/calculate_discount/index.ts
import { createClient } from "jsr:@anthropic/supabase-js@2";

Deno.serve(async (req) => {
  const { order_id } = await req.json();
  const supabase = createClient(
    Deno.env.get("SUPABASE_URL")!,
    Deno.env.get("SUPABASE_ANON_KEY")!
  );

  // DECLARE
  let total: number;
  let discount: number = 0;

  // SELECT INTO
  const { data } = await supabase
    .from("order_items")
    .select("price, quantity")
    .eq("order_id", order_id);
  total = data?.reduce((sum, item) => sum + item.price * item.quantity, 0) ?? 0;

  // IF/ELSIF
  if (total > 100) {
    discount = total * 0.1;
  } else if (total > 50) {
    discount = total * 0.05;
  }

  return new Response(JSON.stringify(discount), {
    headers: { "Content-Type": "application/json" },
  });
});
```

### Step D.1: PL/pgSQL Lexer

**Files:** `internal/plpgsql/lexer.go` (new)

Tokenize PL/pgSQL source:

```go
type TokenType int

const (
    TOKEN_DECLARE TokenType = iota
    TOKEN_BEGIN
    TOKEN_END
    TOKEN_IF
    TOKEN_THEN
    TOKEN_ELSIF
    TOKEN_ELSE
    TOKEN_LOOP
    TOKEN_WHILE
    TOKEN_FOR
    TOKEN_IN
    TOKEN_RETURN
    TOKEN_RAISE
    TOKEN_EXCEPTION
    TOKEN_WHEN
    TOKEN_INTO
    TOKEN_IDENTIFIER
    TOKEN_STRING
    TOKEN_NUMBER
    TOKEN_OPERATOR
    TOKEN_SEMICOLON
    TOKEN_ASSIGN  // :=
    // ... etc
)

type Token struct {
    Type    TokenType
    Value   string
    Line    int
    Column  int
}

func Tokenize(source string) ([]Token, error)
```

**Completion criteria:**
- [ ] Tokenizes all PL/pgSQL keywords
- [ ] Handles string literals (single quotes, dollar quotes)
- [ ] Handles comments (-- and /* */)
- [ ] Preserves position for error messages
- [ ] Unit tests for edge cases

### Step D.2: PL/pgSQL Parser

**Files:** `internal/plpgsql/parser.go`, `internal/plpgsql/ast.go` (new)

Parse tokens into AST:

```go
// ast.go
type Node interface {
    nodeType() string
}

type Function struct {
    Name       string
    Args       []Argument
    ReturnType string
    ReturnsSet bool
    Body       *Block
}

type Block struct {
    Declarations []Declaration
    Statements   []Statement
    ExceptionHandlers []ExceptionHandler
}

type Declaration struct {
    Name         string
    Type         string
    DefaultValue Expression
}

type Statement interface {
    Node
    stmtType() string
}

type IfStatement struct {
    Condition  Expression
    ThenBlock  []Statement
    ElsifBlocks []ElsifBlock
    ElseBlock  []Statement
}

type LoopStatement struct {
    Label      string
    Statements []Statement
}

type ForStatement struct {
    Variable   string
    Query      string  // FOR r IN SELECT ...
    Statements []Statement
}

type ReturnStatement struct {
    Expression Expression
    Query      string  // RETURN QUERY SELECT ...
}

type RaiseStatement struct {
    Level   string  // NOTICE, WARNING, EXCEPTION
    Message string
    Params  []Expression
}

type SQLStatement struct {
    SQL string
}

// parser.go
func Parse(tokens []Token) (*Function, error)
```

**Completion criteria:**
- [ ] Parses DECLARE blocks
- [ ] Parses IF/ELSIF/ELSE
- [ ] Parses LOOP/WHILE/FOR
- [ ] Parses RETURN and RETURN QUERY
- [ ] Parses RAISE statements
- [ ] Parses EXCEPTION blocks
- [ ] Parses embedded SQL (SELECT, INSERT, etc.)
- [ ] Good error messages with line numbers
- [ ] Unit tests for each construct

### Step D.3: TypeScript Code Generator

**Files:** `internal/plpgsql/codegen.go` (new)

Generate TypeScript from AST:

```go
type CodeGenerator struct {
    indent int
    buf    strings.Builder
}

func (g *CodeGenerator) Generate(fn *Function) (string, error)

// Translation rules:
// DECLARE x type := value  →  let x: Type = value
// IF cond THEN             →  if (cond) {
// ELSIF cond THEN          →  } else if (cond) {
// ELSE                     →  } else {
// END IF                   →  }
// LOOP                     →  while (true) {
// EXIT WHEN cond           →  if (cond) break
// FOR r IN SELECT ...      →  for (const r of (await supabase.from(...)))
// RETURN value             →  return new Response(JSON.stringify(value))
// RETURN QUERY SELECT      →  return new Response(JSON.stringify(await supabase...))
// RAISE EXCEPTION          →  throw new Error(...)
// SELECT INTO var          →  const { data } = await ...; var = data
```

**Completion criteria:**
- [ ] Generates valid TypeScript
- [ ] Handles all parsed constructs
- [ ] Imports generated correctly
- [ ] Error handling for untranslatable constructs
- [ ] Unit tests comparing input/output

### Step D.4: SQL to Supabase-JS Translator

**Files:** `internal/plpgsql/sqltranslate.go` (new)

Convert SQL statements to supabase-js query builder:

```go
func TranslateSQLToJS(sql string) (string, error)

// Examples:
// SELECT * FROM users WHERE id = $1
//   → supabase.from('users').select('*').eq('id', arg1)
//
// INSERT INTO logs (msg) VALUES ($1) RETURNING id
//   → supabase.from('logs').insert({ msg: arg1 }).select('id').single()
//
// UPDATE users SET name = $1 WHERE id = $2
//   → supabase.from('users').update({ name: arg1 }).eq('id', arg2)
```

For complex SQL that can't be translated, fall back to raw SQL execution:
```typescript
const { data } = await supabase.rpc('_raw_sql', { query: '...', params: [...] })
```

**Completion criteria:**
- [ ] Translates simple SELECT/INSERT/UPDATE/DELETE
- [ ] Handles WHERE clauses with common operators
- [ ] Handles JOIN (may require raw SQL fallback)
- [ ] Falls back gracefully for complex queries
- [ ] Unit tests for each translation pattern

### Step D.5: Transpiler Integration

**Files:** `internal/rpc/transpiler.go` (new)

Orchestrate the transpilation pipeline:

```go
type Transpiler struct {
    functionsDir string  // Path to functions/ directory
}

type TranspileResult struct {
    FunctionName string
    TypeScript   string
    Warnings     []string
    Errors       []error
}

func (t *Transpiler) Transpile(createSQL string) (*TranspileResult, error)
func (t *Transpiler) Deploy(result *TranspileResult) error  // Write to functions/
```

**Completion criteria:**
- [ ] Full pipeline: SQL → tokens → AST → TypeScript
- [ ] Warnings for potentially incorrect translations
- [ ] Errors for definitely unsupported constructs
- [ ] Integration with Edge Functions directory structure

### Step D.6: Auto-Deploy on CREATE FUNCTION

**Files:** `internal/rpc/handler.go`, `internal/rpc/store.go`

When a PL/pgSQL function is created:
1. Parse and validate
2. Transpile to TypeScript
3. Write to `functions/{name}/index.ts`
4. Store metadata with `language = 'plpgsql_ts'`
5. Edge Functions runtime auto-detects new function

```go
func (s *Store) Create(def *FunctionDef) error {
    if def.Language == "plpgsql" {
        result, err := s.transpiler.Transpile(def.Source)
        if err != nil {
            return err
        }
        if err := s.transpiler.Deploy(result); err != nil {
            return err
        }
        def.Language = "plpgsql_ts"  // Mark as transpiled
    }
    // Store metadata...
}
```

**Completion criteria:**
- [ ] CREATE FUNCTION with plpgsql triggers transpilation
- [ ] Function immediately callable via RPC
- [ ] Metadata indicates transpiled status
- [ ] Dashboard shows original source AND generated TypeScript

### Step D.7: Dashboard Transpilation UI

**Files:** `internal/dashboard/static/`

Enhance Functions tab:
- Show "Transpiled" badge for plpgsql_ts functions
- "View Generated Code" button to see TypeScript
- Warnings displayed if transpilation had issues
- Manual edit option for generated TypeScript

**Completion criteria:**
- [ ] Visual distinction for transpiled functions
- [ ] Side-by-side view: PL/pgSQL ↔ TypeScript
- [ ] Warning display
- [ ] Edit generated code option

### Step D.8: CLI Transpilation Command

**Files:** `cmd/functions.go`

Add CLI command for batch transpilation:

```bash
# Transpile a single function
sblite functions transpile --sql "CREATE FUNCTION ..."

# Transpile from file
sblite functions transpile --file my_function.sql

# Transpile all functions from a Supabase dump
sblite functions transpile --from-dump supabase_schema.sql

# Preview without deploying
sblite functions transpile --sql "..." --dry-run
```

**Completion criteria:**
- [ ] Single function transpilation
- [ ] File input support
- [ ] Batch from SQL dump
- [ ] Dry-run mode
- [ ] Clear output showing result/warnings

### Step D.9: E2E Tests for Transpilation

**Files:** `e2e/tests/rpc/`

Test transpiled functions:

```typescript
describe('PL/pgSQL Transpilation', () => {
  it('transpiles simple IF/ELSE function')
  it('transpiles LOOP with EXIT WHEN')
  it('transpiles FOR IN SELECT')
  it('transpiles RETURN QUERY')
  it('transpiles RAISE EXCEPTION to throw')
  it('handles DECLARE with defaults')
  it('handles nested blocks')
  it('produces warning for cursor usage')
})
```

**Completion criteria:**
- [ ] Test each supported construct
- [ ] Verify semantic equivalence (same inputs → same outputs)
- [ ] Warning/error cases tested

### Step D.10: Documentation Update

**Files:** `docs/functions.md`, `docs/plpgsql-transpilation.md` (new)

Document:
- Supported PL/pgSQL constructs
- Translation rules and semantics
- Known limitations
- How to handle unsupported constructs manually
- Migration workflow from Supabase

**Completion criteria:**
- [ ] Construct compatibility table
- [ ] Examples for each pattern
- [ ] Troubleshooting guide

---

## Phase D Completion Checkpoint

**Before proceeding to Phase A, verify:**

- [ ] All Step D.1-D.10 completion criteria met
- [ ] E2E tests passing
- [ ] Real-world function migration tested
- [ ] Documentation complete
- [ ] Release tagged

**Decision point:** Evaluate if Phase A is needed based on:
- Transpilation coverage for real-world functions
- Performance requirements (HTTP overhead acceptable?)
- Demand for exact PostgreSQL semantics
- Transaction isolation requirements

---

## Phase A: Full PL/pgSQL Interpreter

**Timeline:** 3-6 months (if needed after Phase D)
**Compatibility:** ~95% of Supabase RPC use cases

### Scope

Native execution of PL/pgSQL without transpilation:

- Full procedural control flow
- Transaction support within functions
- Cursor operations
- Exception handling with SQLSTATE
- SECURITY DEFINER execution context
- Exact PostgreSQL semantics

### Step A.1: Interpreter Core

**Files:** `internal/plpgsql/interpreter.go` (new)

Execute AST directly against SQLite:

```go
type Interpreter struct {
    db      *sql.DB
    tx      *sql.Tx        // Current transaction
    vars    *VariableScope // Variable storage
    cursors map[string]*Cursor
}

type VariableScope struct {
    parent *VariableScope
    vars   map[string]interface{}
}

func (i *Interpreter) Execute(fn *Function, args map[string]interface{}) (interface{}, error)
func (i *Interpreter) executeBlock(block *Block) error
func (i *Interpreter) executeStatement(stmt Statement) error
func (i *Interpreter) evaluateExpression(expr Expression) (interface{}, error)
```

**Completion criteria:**
- [ ] Variable declaration and assignment
- [ ] Expression evaluation
- [ ] Block execution with proper scoping

### Step A.2: Control Flow Implementation

**Files:** `internal/plpgsql/interpreter.go`

Implement control structures:

```go
func (i *Interpreter) executeIf(stmt *IfStatement) error
func (i *Interpreter) executeLoop(stmt *LoopStatement) error
func (i *Interpreter) executeWhile(stmt *WhileStatement) error
func (i *Interpreter) executeFor(stmt *ForStatement) error
func (i *Interpreter) executeForeach(stmt *ForeachStatement) error
```

Support:
- `IF/ELSIF/ELSE/END IF`
- `LOOP/END LOOP` with `EXIT` and `CONTINUE`
- `WHILE condition LOOP`
- `FOR var IN min..max LOOP`
- `FOR record IN SELECT LOOP`
- `FOREACH element IN ARRAY arr LOOP`

**Completion criteria:**
- [ ] All loop types working
- [ ] EXIT and CONTINUE with labels
- [ ] Nested loops
- [ ] Unit tests for each pattern

### Step A.3: SQL Execution Within Functions

**Files:** `internal/plpgsql/interpreter.go`

Execute SQL statements in function context:

```go
func (i *Interpreter) executeSQL(sql string) (*sql.Rows, error)
func (i *Interpreter) executeSQLInto(sql string, targets []string) error
func (i *Interpreter) executeReturnQuery(sql string) ([]interface{}, error)
```

Handle:
- Variable substitution in SQL
- SELECT INTO variable
- INSERT/UPDATE/DELETE with RETURNING INTO
- RETURN QUERY
- PERFORM (execute without result)

**Completion criteria:**
- [ ] Variable substitution works
- [ ] SELECT INTO populates variables
- [ ] RETURNING INTO works
- [ ] PERFORM executes without error

### Step A.4: Cursor Support

**Files:** `internal/plpgsql/cursor.go` (new)

Implement cursor operations:

```go
type Cursor struct {
    name    string
    query   string
    rows    *sql.Rows
    columns []string
}

func (i *Interpreter) declareCursor(name string, query string) error
func (i *Interpreter) openCursor(name string) error
func (i *Interpreter) fetchCursor(name string, target string) (bool, error)
func (i *Interpreter) closeCursor(name string) error
```

Support:
- DECLARE cursor FOR query
- OPEN cursor
- FETCH cursor INTO variables
- CLOSE cursor
- FOR record IN cursor LOOP
- Cursor parameters

**Completion criteria:**
- [ ] Basic cursor operations work
- [ ] Cursor iteration in loops
- [ ] Cursor with parameters
- [ ] Proper cleanup on function exit

### Step A.5: Exception Handling

**Files:** `internal/plpgsql/exception.go` (new)

Implement PostgreSQL exception model:

```go
type PLpgSQLError struct {
    SQLState string
    Message  string
    Detail   string
    Hint     string
}

func (i *Interpreter) executeWithExceptionHandler(block *Block) error
func (i *Interpreter) raise(level string, message string, params ...interface{}) error
func (i *Interpreter) getSQLState(err error) string
```

Support:
- BEGIN/EXCEPTION/END blocks
- WHEN condition THEN handler
- RAISE NOTICE/WARNING/EXCEPTION
- SQLSTATE codes
- GET STACKED DIAGNOSTICS

**Completion criteria:**
- [ ] EXCEPTION blocks catch errors
- [ ] WHEN conditions filter by SQLSTATE
- [ ] RAISE produces appropriate errors
- [ ] GET DIAGNOSTICS retrieves error info
- [ ] Nested exception blocks

### Step A.6: SECURITY DEFINER Support

**Files:** `internal/plpgsql/security.go` (new)

Execute functions as the definer (not invoker):

```go
func (i *Interpreter) executeAsDefiner(fn *Function, args map[string]interface{}) (interface{}, error)
```

When `SECURITY DEFINER`:
- Function runs with definer's permissions
- RLS policies see definer's identity
- Useful for privileged operations

**Completion criteria:**
- [ ] SECURITY INVOKER works (default)
- [ ] SECURITY DEFINER elevates privileges
- [ ] RLS respects security context

### Step A.7: Transaction Integration

**Files:** `internal/plpgsql/interpreter.go`

Proper transaction handling:

```go
func (i *Interpreter) beginTransaction() error
func (i *Interpreter) commitTransaction() error
func (i *Interpreter) rollbackTransaction() error
func (i *Interpreter) savepoint(name string) error
func (i *Interpreter) rollbackToSavepoint(name string) error
```

Functions should:
- See their own uncommitted changes
- Rollback on unhandled exception
- Support savepoints within exception blocks

**Completion criteria:**
- [ ] Function changes visible within function
- [ ] Rollback on error
- [ ] Savepoints work with exception handling

### Step A.8: Expression Evaluator

**Files:** `internal/plpgsql/eval.go` (new)

Evaluate PL/pgSQL expressions:

```go
func (i *Interpreter) evaluate(expr Expression) (interface{}, error)
func (i *Interpreter) evaluateBinaryOp(left, right interface{}, op string) (interface{}, error)
func (i *Interpreter) evaluateFunction(name string, args []interface{}) (interface{}, error)
func (i *Interpreter) coerce(value interface{}, targetType string) (interface{}, error)
```

Support:
- Arithmetic operators
- Comparison operators
- Logical operators (AND, OR, NOT)
- String concatenation
- Array subscripting
- Field access (record.field)
- Type coercion
- Built-in functions (COALESCE, NULLIF, etc.)

**Completion criteria:**
- [ ] All operators work
- [ ] Type coercion matches PostgreSQL
- [ ] Built-in functions implemented
- [ ] Array operations work

### Step A.9: Performance Optimization

**Files:** `internal/plpgsql/`

Optimize for production use:

- Cache parsed ASTs
- Prepare frequently-used SQL statements
- Connection pooling awareness
- Memory management for large result sets

**Completion criteria:**
- [ ] Benchmark suite created
- [ ] AST caching implemented
- [ ] No memory leaks in long-running functions
- [ ] Performance acceptable vs PostgreSQL

### Step A.10: Comprehensive Testing

**Files:** `internal/plpgsql/*_test.go`, `e2e/tests/rpc/`

Full test coverage:

```go
// Unit tests for each component
func TestLexer_*
func TestParser_*
func TestInterpreter_*
func TestCursor_*
func TestException_*
```

```typescript
// E2E tests for semantic equivalence
describe('PL/pgSQL Interpreter', () => {
  // Run same function on PostgreSQL and sblite
  // Compare results
})
```

**Completion criteria:**
- [ ] >90% code coverage
- [ ] Semantic equivalence tests pass
- [ ] Edge cases documented and tested
- [ ] Fuzz testing for parser

### Step A.11: Documentation and Migration Guide

**Files:** `docs/plpgsql-interpreter.md` (new)

Document:
- Full compatibility matrix
- Known differences from PostgreSQL
- Performance characteristics
- Migration from transpiled functions
- Debugging PL/pgSQL in sblite

**Completion criteria:**
- [ ] Compatibility matrix complete
- [ ] All differences documented
- [ ] Migration guide tested

---

## Phase A Completion Checkpoint

**Final verification:**

- [ ] All Step A.1-A.11 completion criteria met
- [ ] Semantic equivalence with PostgreSQL verified
- [ ] Performance benchmarks acceptable
- [ ] Documentation complete
- [ ] Release tagged as major version

---

## Summary

| Phase | Timeline | Compatibility | Key Deliverable |
|-------|----------|---------------|-----------------|
| **C** | 2-4 weeks | ~40% | SQL functions via `/rpc/v1/` |
| **D** | 4-6 weeks | ~70-80% | PL/pgSQL → TypeScript transpiler |
| **A** | 3-6 months | ~95% | Native PL/pgSQL interpreter |

Each phase is independently valuable and shippable. Evaluate actual user needs before proceeding to the next phase.
