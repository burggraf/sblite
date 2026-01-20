# PostgreSQL Functions (RPC) - Phase C Implementation Design

**Status:** Approved
**Created:** 2026-01-21
**Scope:** SQL-language functions only (LANGUAGE sql)

## Overview

Implement PostgreSQL SQL-language functions callable via `/rest/v1/rpc/{name}`, fully compatible with `supabase.rpc()`. Uses the existing `pgtranslate` package to convert PostgreSQL syntax to SQLite.

## Architecture

```
CREATE FUNCTION (PostgreSQL) → Parser → Translator (pg→sqlite) → Store in _functions
                                                                        ↓
supabase.rpc('fn', {args})  →  /rest/v1/rpc/fn  →  Executor → SQLite → Response
```

### Components

| Component | File | Purpose |
|-----------|------|---------|
| Parser | `internal/rpc/parser.go` | Parse CREATE FUNCTION statements |
| Store | `internal/rpc/store.go` | CRUD for _functions, _function_args tables |
| Executor | `internal/rpc/executor.go` | Execute functions with parameter binding |
| Handler | `internal/rpc/handler.go` | HTTP handler for /rest/v1/rpc/{name} |
| Interceptor | `internal/rpc/interceptor.go` | Intercept CREATE FUNCTION in SQL execution |

## Database Schema

```sql
-- Function definitions
CREATE TABLE IF NOT EXISTS _functions (
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
CREATE TABLE IF NOT EXISTS _function_args (
  id TEXT PRIMARY KEY,
  function_id TEXT NOT NULL REFERENCES _functions(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  position INTEGER NOT NULL,
  default_value TEXT,
  UNIQUE(function_id, position)
);

CREATE INDEX idx_function_args_function ON _function_args(function_id);
```

## Supported CREATE FUNCTION Syntax

```sql
CREATE [OR REPLACE] FUNCTION name(arg1 type, arg2 type DEFAULT value, ...)
RETURNS type | RETURNS TABLE(col1 type, col2 type, ...) | RETURNS SETOF type
LANGUAGE sql
[VOLATILE | STABLE | IMMUTABLE]
[SECURITY INVOKER | SECURITY DEFINER]
AS $$ ... $$;
```

### Parser Output

```go
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

type FunctionArg struct {
    Name         string
    Type         string
    Position     int
    DefaultValue *string
}
```

## Executor Flow

1. **Lookup**: Get function from `_functions` by name (404 if not found)
2. **Validate Args**: Check required args present, apply defaults
3. **Parameter Binding**: Replace named params in SQL body with values
4. **Execute**: Run translated SQL (stored in `source_sqlite`)
5. **Format Response**: Based on return type (scalar, object, array)

### RLS Integration

- `SECURITY INVOKER` (default): Runs with caller's permissions, RLS applies
- `SECURITY DEFINER`: Bypasses RLS for privileged operations

## HTTP API

### Endpoint

`POST /rest/v1/rpc/{name}`

### Request

```http
POST /rest/v1/rpc/get_user_posts
Content-Type: application/json
Authorization: Bearer <jwt>
apikey: <anon_key>

{"user_uuid": "550e8400-e29b-41d4-a716-446655440000"}
```

### Response Formats

```json
// RETURNS TABLE or SETOF → array
[{"id": "...", "title": "..."}, {"id": "...", "title": "..."}]

// RETURNS single row type → object
{"id": "...", "email": "..."}

// RETURNS scalar → raw value
42

// RETURNS void → null
null
```

### Error Responses

```json
{"code": "PGRST202", "message": "Function not found", "details": null, "hint": null}
{"code": "42883", "message": "Function get_user_posts(wrong_arg) does not exist"}
```

### Headers

- `Prefer: return=minimal` → Return just success status
- `Accept: application/vnd.pgrst.object+json` → Force single object

## SQL Interception

CREATE FUNCTION statements are intercepted at:
1. Dashboard SQL Browser (postgres_mode)
2. Migration files (sblite db push)
3. Direct SQL via /_/api/sql

```go
func (i *Interceptor) ProcessSQL(sql string, postgresMode bool) (result string, handled bool, err error)
```

### DROP FUNCTION Support

```sql
DROP FUNCTION [IF EXISTS] name;
```

## E2E Test Coverage

### Test Files

```
e2e/tests/rpc/
├── sql-functions.test.ts      # Core RPC functionality
├── function-creation.test.ts  # CREATE/DROP FUNCTION via SQL
├── rls-integration.test.ts    # RLS with functions
└── dashboard-sql.test.ts      # Functions in SQL browser
```

### Test Cases

**Core RPC**:
- Execute function returning scalar, single row, TABLE, SETOF
- Functions with no args, required args, default args, NULL args
- 404 for unknown function
- Error for missing/wrong argument type

**Function Creation**:
- CREATE FUNCTION via SQL browser
- CREATE OR REPLACE overwrites existing
- DROP FUNCTION removes function
- Reject LANGUAGE plpgsql with clear error
- Parse complex RETURNS TABLE definitions
- Handle dollar-quoted bodies

**RLS Integration**:
- SECURITY INVOKER respects RLS
- SECURITY DEFINER bypasses RLS
- Function sees caller's auth.uid()

**Dashboard & Direct SQL**:
- Call function from SQL browser
- Use function in RLS policy
- Function in migration file

## Documentation

**New File**: `docs/rpc-functions.md`

Contents:
1. Overview
2. Quick Start
3. Creating Functions (syntax reference)
4. Calling Functions (client examples)
5. Parameter Types
6. Return Types
7. Security (INVOKER vs DEFINER)
8. PostgreSQL Translation
9. Using in RLS Policies
10. Using in SQL Browser
11. Limitations
12. Migration from Supabase
13. Examples

**Updates**:
- README.md: Add feature link
- CLAUDE.md: Add endpoint, update status

## Implementation Steps

1. Add `_functions` and `_function_args` tables to migrations
2. Implement `internal/rpc/parser.go` - CREATE FUNCTION parser
3. Implement `internal/rpc/store.go` - Function CRUD
4. Implement `internal/rpc/executor.go` - Function execution
5. Implement `internal/rpc/handler.go` - HTTP handler
6. Implement `internal/rpc/interceptor.go` - SQL interception
7. Integrate into server routes (`/rest/v1/rpc/{name}`)
8. Integrate interceptor into dashboard SQL and migrations
9. Write E2E tests
10. Write documentation
11. Update README.md and CLAUDE.md

## Limitations (Phase C)

- SQL-language functions only (no PL/pgSQL)
- No function overloading (same name, different arg types)
- No OUT parameters
- No variadic arguments
- No polymorphic types
