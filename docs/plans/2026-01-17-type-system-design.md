# SQLite-to-PostgreSQL Type System Design

**Date:** 2026-01-17
**Status:** Approved
**Goal:** Enable sblite apps to migrate cleanly to Supabase by tracking PostgreSQL type information for SQLite-stored data.

## Overview

sblite stores all data in SQLite but needs to generate proper PostgreSQL schemas for migration to Supabase. This design introduces:

1. A defined set of supported types
2. A metadata table tracking intended PostgreSQL types
3. Runtime validation on writes
4. A migration export tool

## Supported Types

| sblite Type | SQLite Storage | PostgreSQL Type | Validation | REST API Format |
|-------------|----------------|-----------------|------------|-----------------|
| `uuid` | TEXT | uuid | RFC 4122 format | string |
| `text` | TEXT | text | None | string |
| `integer` | INTEGER | integer | 32-bit signed | number |
| `numeric` | TEXT | numeric | Valid decimal string | string (preserves precision) |
| `boolean` | INTEGER | boolean | 0 or 1 | true/false |
| `timestamptz` | TEXT | timestamptz | ISO 8601 UTC | string |
| `jsonb` | TEXT | jsonb | json_valid() | object/array |
| `bytea` | BLOB | bytea | Valid base64 | base64 string |

### Type Rationale

- **8 types** covers 95%+ of real-world use cases
- **numeric stored as TEXT** preserves arbitrary precision (no floating-point errors)
- **timestamptz** is Supabase's standard for all datetime needs
- **Arrays** handled via jsonb (more flexible, works natively with JS client)
- **Enums** handled via text + application validation

## Schema Metadata Table

```sql
CREATE TABLE IF NOT EXISTS _columns (
    table_name    TEXT NOT NULL,
    column_name   TEXT NOT NULL,
    pg_type       TEXT NOT NULL CHECK (pg_type IN (
                    'uuid', 'text', 'integer', 'numeric',
                    'boolean', 'timestamptz', 'jsonb', 'bytea'
                  )),
    is_nullable   INTEGER DEFAULT 1,
    default_value TEXT,
    is_primary    INTEGER DEFAULT 0,
    created_at    TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (table_name, column_name)
);
```

### Usage

1. **Table creation** - Admin API creates SQLite table AND populates `_columns`
2. **Validation** - REST handlers look up types before INSERT/UPDATE
3. **Migration** - Export tool reads `_columns` to generate PostgreSQL DDL

## Type Validation

### Validator Interface

```go
// internal/types/validate.go

type Validator func(value any) error

var validators = map[string]Validator{
    "uuid":        validateUUID,
    "text":        validateText,
    "integer":     validateInteger,
    "numeric":     validateNumeric,
    "boolean":     validateBoolean,
    "timestamptz": validateTimestamptz,
    "jsonb":       validateJSONB,
    "bytea":       validateBytea,
}

func Validate(pgType string, value any) error {
    if v, ok := validators[pgType]; ok {
        return v(value)
    }
    return fmt.Errorf("unknown type: %s", pgType)
}
```

### Validation Rules

| Type | Rule | Valid Examples | Invalid Examples |
|------|------|----------------|------------------|
| uuid | RFC 4122 format | `"550e8400-e29b-41d4-a716-446655440000"` | `"not-a-uuid"` |
| text | Any string | Any string | — |
| integer | -2³¹ to 2³¹-1 | `42`, `-1000` | `"abc"`, `9999999999999` |
| numeric | Valid decimal | `"123.45"`, `"-99.00"` | `"12.34.56"` |
| boolean | bool or 0/1 | `true`, `false`, `0`, `1` | `"yes"`, `2` |
| timestamptz | ISO 8601 | `"2024-01-15T10:30:00Z"` | `"yesterday"` |
| jsonb | Valid JSON | `{"key": "value"}` | `{invalid` |
| bytea | Valid base64 | `"SGVsbG8="` | `"not@base64!"` |

### Integration Point

```go
func (h *Handler) validateRow(table string, data map[string]any) error {
    columns, _ := h.schema.GetColumns(table)
    for col, val := range data {
        if colDef, ok := columns[col]; ok {
            if err := types.Validate(colDef.PgType, val); err != nil {
                return fmt.Errorf("column %q: %w", col, err)
            }
        }
    }
    return nil
}
```

## Table Creation API

### Endpoint

`POST /admin/v1/tables`

```json
{
  "name": "products",
  "columns": [
    {"name": "id", "type": "uuid", "primary": true, "default": "gen_uuid()"},
    {"name": "name", "type": "text", "nullable": false},
    {"name": "price", "type": "numeric"},
    {"name": "in_stock", "type": "boolean", "default": "true"},
    {"name": "metadata", "type": "jsonb"},
    {"name": "created_at", "type": "timestamptz", "default": "now()"}
  ]
}
```

### Internal Flow

1. Validate request (types valid, column names safe)
2. Create SQLite table with appropriate storage types
3. Populate `_columns` with intended PostgreSQL types
4. Return success with generated schema

### Default Value Mapping

| API Default | SQLite Default |
|-------------|----------------|
| `"gen_uuid()"` | `(lower(hex(randomblob(16))))` or proper UUID v4 |
| `"now()"` | `(datetime('now'))` |
| `"true"` / `"false"` | `1` / `0` |
| Literal value | Literal value |

### Additional Admin Endpoints

- `GET /admin/v1/tables` - List tables with schemas
- `GET /admin/v1/tables/{name}` - Get single table schema
- `DELETE /admin/v1/tables/{name}` - Drop table
- `PATCH /admin/v1/tables/{name}` - Alter table (add/drop columns)

## Migration Export

### CLI Command

```bash
./sblite migrate export --output schema.sql
./sblite migrate export --output schema.sql --include-data
```

### Output Format

```sql
-- Generated by sblite migrate export

CREATE TABLE "products" (
    "id" uuid PRIMARY KEY,
    "name" text NOT NULL,
    "price" numeric,
    "metadata" jsonb,
    "created_at" timestamptz DEFAULT now()
);

-- With --include-data
INSERT INTO "products" ("id", "name", "price", "metadata", "created_at") VALUES
    ('550e8400-e29b-41d4-a716-446655440000', 'Widget', '29.99', '{"color":"blue"}', '2024-01-15T10:30:00Z');
```

### Type Transformations

| sblite Storage | Export Transformation |
|----------------|----------------------|
| TEXT (uuid) | Output as-is |
| TEXT (numeric) | Output as-is (preserves precision) |
| INTEGER (boolean) | Convert 0/1 → false/true |
| TEXT (timestamptz) | Output as-is (ISO 8601) |
| TEXT (jsonb) | Output as-is |
| BLOB (bytea) | Encode as `'\x...'` hex format |

## Implementation Scope

### New Packages

- `internal/types/` - Type validation
- `internal/schema/` - Schema metadata operations
- `internal/admin/` - Admin API handlers
- `cmd/migrate.go` - Migration CLI command

### Modified Packages

- `internal/rest/handler.go` - Add validation before writes
- `internal/db/migrations.go` - Add `_columns` table

## Future Considerations

Types explicitly NOT included (can add if needed):

- **Arrays** - Use jsonb instead
- **Enums** - Use text + validation
- **date/time** - Use timestamptz
- **varchar(n)** - Use text + app validation
- **float/double** - Use numeric for precision
