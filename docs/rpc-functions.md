# PostgreSQL Functions (RPC)

**Status:** Implemented (Phase C - SQL functions only)

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

## Managing Functions

### List Functions

```sql
SELECT name, return_type, volatility FROM _rpc_functions;
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
