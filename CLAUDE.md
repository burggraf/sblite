# Claude Code Instructions for sblite

## Project Overview

**sblite** (Supabase Lite) is a lightweight, single-binary backend providing a subset of Supabase functionality. Built for fast startup, scale-to-zero deployments, and seamless migration to full Supabase when needed.

**Key characteristics:**
- Single Go binary with embedded SQLite (no CGO)
- Supabase API-compatible (works with `@supabase/supabase-js`)
- Sub-second startup, scale-to-zero ready

## Tech Stack

- **Go 1.25** - Core runtime
- **Chi** - HTTP router (`github.com/go-chi/chi/v5`)
- **modernc.org/sqlite** - Pure Go SQLite (no CGO required)
- **golang-jwt/jwt/v5** - JWT signing/validation
- **bcrypt** - Password hashing (`golang.org/x/crypto/bcrypt`)
- **Cobra** - CLI framework

## Project Structure

```
sblite/
├── main.go                    # Entry point
├── cmd/                       # CLI commands (Cobra)
│   ├── root.go               # Root command
│   ├── init.go               # `sblite init` - create database
│   ├── serve.go              # `sblite serve` - start server
│   └── migrate.go            # `sblite migrate` - export to PostgreSQL
├── internal/
│   ├── auth/                 # Authentication service
│   │   ├── jwt.go            # JWT generation/validation, sessions
│   │   └── user.go           # User management, password hashing
│   ├── db/                   # Database layer
│   │   ├── db.go             # SQLite connection, WAL mode
│   │   └── migrations.go     # Auth schema tables, _columns metadata
│   ├── types/                # Type system
│   │   ├── types.go          # PostgreSQL type definitions
│   │   └── validate.go       # Type validators (uuid, text, integer, etc.)
│   ├── schema/               # Schema metadata
│   │   └── schema.go         # Column metadata operations (_columns table)
│   ├── admin/                # Admin API
│   │   └── handler.go        # Table management endpoints
│   ├── migrate/              # Migration tools
│   │   └── export.go         # PostgreSQL DDL export
│   ├── rest/                 # REST API (PostgREST-compatible)
│   │   ├── handler.go        # HTTP handlers for CRUD
│   │   ├── query.go          # Query parsing (filters, modifiers)
│   │   └── builder.go        # SQL query building
│   ├── log/                  # Logging system
│   │   ├── logger.go         # Config, initialization
│   │   ├── console.go        # Console handler
│   │   ├── file.go           # File handler with rotation
│   │   ├── database.go       # SQLite handler
│   │   └── middleware.go     # HTTP request logging
│   └── server/               # HTTP server
│       ├── server.go         # Chi router setup, route registration
│       ├── auth_handlers.go  # /auth/v1/* endpoints
│       └── middleware.go     # JWT auth middleware
├── e2e/                      # End-to-end test suite (Node.js/Vitest)
│   ├── tests/                # Test files by category
│   ├── TESTS.md              # Complete test inventory (173 tests)
│   └── COMPATIBILITY.md      # Supabase feature compatibility matrix
└── docs/plans/               # Design documents
```

## Build & Run Commands

```bash
# Build the binary
go build -o sblite .

# Initialize database (creates data.db with auth schema)
./sblite init

# Start server
./sblite serve                      # Default: localhost:8080
./sblite serve --port 3000          # Custom port
./sblite serve --host 0.0.0.0       # Bind to all interfaces
./sblite serve --db /path/to/data.db  # Custom database path

# Run Go tests
go test ./...

# Export schema for Supabase migration
./sblite migrate export --db data.db           # Output to stdout
./sblite migrate export --db data.db -o schema.sql  # Output to file

# Run E2E tests (requires Node.js 18+)
cd e2e
npm install
npm run setup    # Initialize test database
npm test         # Run all tests (server must be running)
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_JWT_SECRET` | (warning if unset) | JWT signing secret |
| `SBLITE_HOST` | `0.0.0.0` | Server bind address |
| `SBLITE_PORT` | `8080` | Server port |
| `SBLITE_DB_PATH` | `./data.db` | SQLite database path |

### Logging Configuration

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--log-mode` | `SBLITE_LOG_MODE` | `console` | Output: console, file, database |
| `--log-level` | `SBLITE_LOG_LEVEL` | `info` | Level: debug, info, warn, error |
| `--log-format` | `SBLITE_LOG_FORMAT` | `text` | Format: text, json |
| `--log-file` | `SBLITE_LOG_FILE` | `sblite.log` | Log file path |
| `--log-db` | `SBLITE_LOG_DB` | `log.db` | Log database path |
| `--log-max-size` | `SBLITE_LOG_MAX_SIZE` | `100` | Max file size (MB) |
| `--log-max-age` | `SBLITE_LOG_MAX_AGE` | `7` | Retention days |
| `--log-max-backups` | `SBLITE_LOG_MAX_BACKUPS` | `3` | Backup files to keep |
| `--log-fields` | `SBLITE_LOG_FIELDS` | `` | DB fields (comma-separated) |

**Example usage:**
```bash
# JSON console logging
./sblite serve --log-format=json

# File logging with rotation
./sblite serve --log-mode=file --log-file=/var/log/sblite.log

# Database logging with full context
./sblite serve --log-mode=database --log-db=/var/log/sblite-log.db \
  --log-fields=source,request_id,user_id,extra
```

## API Endpoints

### Authentication (`/auth/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/v1/signup` | POST | Create new user |
| `/auth/v1/token?grant_type=password` | POST | Sign in with email/password |
| `/auth/v1/token?grant_type=refresh_token` | POST | Refresh access token |
| `/auth/v1/logout` | POST | Sign out (requires auth) |
| `/auth/v1/user` | GET | Get current user (requires auth) |
| `/auth/v1/user` | PUT | Update current user (requires auth) |

### REST API (`/rest/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/rest/v1/{table}` | GET | Select rows with filters |
| `/rest/v1/{table}` | POST | Insert rows (validates against type schema) |
| `/rest/v1/{table}` | PATCH | Update rows (validates against type schema) |
| `/rest/v1/{table}` | DELETE | Delete rows matching filters |

### Admin API (`/admin/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/v1/tables` | POST | Create table with typed schema |
| `/admin/v1/tables` | GET | List all user tables |
| `/admin/v1/tables/{name}` | GET | Get table schema |
| `/admin/v1/tables/{name}` | DELETE | Drop table |

### Query Operators

Filters use PostgREST syntax: `column=operator.value`

| Operator | Example | SQL |
|----------|---------|-----|
| `eq` | `status=eq.active` | `status = 'active'` |
| `neq` | `status=neq.deleted` | `status != 'deleted'` |
| `gt`, `gte`, `lt`, `lte` | `age=gt.21` | `age > 21` |
| `like` | `name=like.*john*` | `name LIKE '%john%'` |
| `ilike` | `name=ilike.*john*` | Case-insensitive LIKE |
| `in` | `id=in.(1,2,3)` | `id IN (1, 2, 3)` |
| `is` | `deleted=is.null` | `deleted IS NULL` |

## Architecture Notes

### Database Schema

Auth tables mirror Supabase's structure for migration compatibility:
- `auth_users` - User accounts (bcrypt passwords, metadata)
- `auth_sessions` - Active sessions
- `auth_refresh_tokens` - Refresh token storage

### JWT Structure

JWTs match Supabase format exactly:
- Claims: `sub` (user ID), `email`, `role`, `session_id`, `app_metadata`, `user_metadata`
- Algorithm: HS256
- Access token: 1 hour, Refresh token: 1 week

### SQLite

- Uses WAL mode for concurrent reads
- Pure Go driver (no CGO) for easy cross-compilation
- All timestamps stored as ISO 8601 strings

### Type System

sblite tracks intended PostgreSQL types for SQLite-stored data, enabling clean migration to Supabase.

**Supported Types:**

| sblite Type | SQLite Storage | PostgreSQL Type | Validation |
|-------------|----------------|-----------------|------------|
| `uuid` | TEXT | uuid | RFC 4122 format |
| `text` | TEXT | text | None |
| `integer` | INTEGER | integer | 32-bit signed |
| `numeric` | TEXT | numeric | Valid decimal string |
| `boolean` | INTEGER | boolean | 0 or 1 |
| `timestamptz` | TEXT | timestamptz | ISO 8601 UTC |
| `jsonb` | TEXT | jsonb | json_valid() |
| `bytea` | BLOB | bytea | Valid base64 |

**Schema Metadata:**
- Column types stored in `_columns` table
- Tables created via Admin API automatically register metadata
- REST API validates writes against registered schemas
- `sblite migrate export` generates PostgreSQL DDL from metadata

## Testing

### Go Unit Tests

Each internal package has `*_test.go` files. Run with `go test ./...`.

### E2E Test Suite

Located in `e2e/`, validates compatibility with `@supabase/supabase-js` client.

**Test categories:**
- REST: SELECT, INSERT, UPDATE, UPSERT, DELETE
- Filters: eq, neq, gt, gte, lt, lte, like, ilike, is, in
- Modifiers: order, limit, range, single, maybeSingle
- Auth: signup, signin, signout, session, user management

**Running E2E tests:**
1. Start the server: `./sblite serve --db test.db`
2. In another terminal: `cd e2e && npm test`

See `e2e/TESTS.md` for the complete test inventory (173 tests, 115 active, 58 skipped for unimplemented features).

## Implementation Status

### Implemented (Phase 1)
- Email/password authentication
- JWT sessions with refresh tokens
- REST API CRUD operations
- Query filters (eq, neq, gt, gte, lt, lte, like, ilike, is, in)
- Query modifiers (select, order, limit, offset)
- single() and maybeSingle() response modifiers
- Configurable logging (console, file, database backends)
- Type system with PostgreSQL type tracking
- Admin API for typed table creation
- PostgreSQL DDL export for Supabase migration

### Planned
- Row Level Security (RLS) via query rewriting
- Realtime subscriptions
- File storage
- or() / not() / match() filters
- Referenced table queries (relationships)

## Code Conventions

- Follow standard Go formatting (`gofmt`)
- Package-level comments in each package
- Error handling: wrap errors with context using `fmt.Errorf("context: %w", err)`
- HTTP handlers return JSON with appropriate status codes
- SQL queries use parameterized statements (no string concatenation)
- **UUIDs**: All ID fields (user IDs, session IDs, etc.) must use proper UUID v4 format to maintain Supabase compatibility. Use `github.com/google/uuid` for generation. Never use hex-encoded random bytes or other non-UUID formats.

## Common Tasks

### Adding a new auth endpoint

1. Add handler method in `internal/server/auth_handlers.go`
2. Register route in `internal/server/server.go` setupRoutes()
3. Add tests in `internal/server/auth_handlers_test.go`
4. Add E2E test in `e2e/tests/auth/`

### Adding a new REST filter

1. Add operator handling in `internal/rest/query.go` ParseFilter()
2. Add SQL generation in `internal/rest/builder.go`
3. Add tests in `internal/rest/query_test.go` and `internal/rest/builder_test.go`
4. Add E2E test in `e2e/tests/filters/`

### Adding a new CLI command

1. Create new file in `cmd/` (e.g., `cmd/backup.go`)
2. Define command with `cobra.Command`
3. Add to root command via `init()` function

### Adding a new type to the type system

1. Add type constant in `internal/types/types.go`
2. Add SQLite storage mapping in `SQLiteType()` method
3. Add validator function in `internal/types/validate.go`
4. Register validator in `validators` map
5. Add tests in `internal/types/validate_test.go`
6. Update `_columns` CHECK constraint in `internal/db/migrations.go`
7. Add PostgreSQL DDL mapping in `internal/migrate/export.go`

### Adding a new admin endpoint

1. Add handler method in `internal/admin/handler.go`
2. Register route in `internal/server/server.go` setupRoutes()
3. Add tests in `internal/admin/handler_test.go`
