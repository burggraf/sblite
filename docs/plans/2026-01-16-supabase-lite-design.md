# Supabase Lite Design Document

**Date:** 2026-01-16
**Status:** Draft
**Author:** Mark + Claude

## Overview

Supabase Lite is a lightweight, single-binary backend that provides a subset of Supabase functionality. It's designed for:

- **Fast startup** (sub-second)
- **Scale to zero** (no idle resources)
- **Easy deployment** (single executable)
- **Migration path** (one-way migration to full Supabase)

### Inspiration

- **PocketBase**: Single Go binary with embedded SQLite, proves the architecture works
- **Supabase**: API compatibility target for seamless developer experience

### Non-Goals

- Full PostgreSQL compatibility
- Horizontal scaling / clustering
- Postgres wire protocol
- GraphQL support
- Edge Functions

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Supabase Lite Binary                   │
├─────────────────────────────────────────────────────────────┤
│  HTTP Server (chi router)                                   │
│  ├── /rest/v1/*     PostgREST-compatible API               │
│  ├── /auth/v1/*     GoTrue-compatible Auth API             │
│  └── /health        Health check endpoint                   │
├─────────────────────────────────────────────────────────────┤
│  Core Services                                              │
│  ├── QueryEngine    SQL generation + RLS injection          │
│  ├── AuthService    JWT issuance, user management           │
│  └── PolicyEngine   RLS policy parsing and enforcement      │
├─────────────────────────────────────────────────────────────┤
│  SQLite (modernc.org/sqlite - pure Go, no CGO)             │
│  ├── data.db        Application data + auth schema          │
│  └── WAL mode       Concurrent reads, serialized writes     │
└─────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

1. **Pure Go SQLite** (`modernc.org/sqlite`): No CGO required, cross-compiles easily
2. **WAL mode**: Enables concurrent reads with single writer
3. **API-layer RLS**: Query rewriting instead of database-native RLS
4. **Supabase API compatibility**: supabase-js works unchanged

### Go Dependencies

| Package | Purpose |
|---------|---------|
| `modernc.org/sqlite` | Pure Go SQLite driver |
| `github.com/golang-jwt/jwt/v5` | JWT signing/verification |
| `github.com/go-chi/chi/v5` | HTTP routing |
| `golang.org/x/crypto/bcrypt` | Password hashing |

---

## Auth Schema

SQLite tables mirroring Supabase's `auth` schema for migration compatibility.

### auth_users

```sql
CREATE TABLE auth_users (
    id                    TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    email                 TEXT UNIQUE,
    encrypted_password    TEXT,  -- bcrypt hash ($2a$10$...)
    email_confirmed_at    TEXT,  -- ISO timestamp or NULL
    invited_at            TEXT,
    confirmation_token    TEXT,
    confirmation_sent_at  TEXT,
    recovery_token        TEXT,
    recovery_sent_at      TEXT,
    email_change_token    TEXT,
    email_change          TEXT,
    last_sign_in_at       TEXT,
    raw_app_meta_data     TEXT DEFAULT '{}' CHECK (json_valid(raw_app_meta_data)),
    raw_user_meta_data    TEXT DEFAULT '{}' CHECK (json_valid(raw_user_meta_data)),
    is_super_admin        INTEGER DEFAULT 0,
    role                  TEXT DEFAULT 'authenticated',
    created_at            TEXT DEFAULT (datetime('now')),
    updated_at            TEXT DEFAULT (datetime('now')),
    banned_until          TEXT,
    deleted_at            TEXT
);

CREATE INDEX idx_auth_users_email ON auth_users(email);
CREATE INDEX idx_auth_users_confirmation_token ON auth_users(confirmation_token);
CREATE INDEX idx_auth_users_recovery_token ON auth_users(recovery_token);
```

### auth_sessions

```sql
CREATE TABLE auth_sessions (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT,
    factor_id     TEXT,  -- for MFA (future)
    aal           TEXT DEFAULT 'aal1',
    not_after     TEXT
);

CREATE INDEX idx_auth_sessions_user_id ON auth_sessions(user_id);
```

### auth_refresh_tokens

```sql
CREATE TABLE auth_refresh_tokens (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    token       TEXT UNIQUE NOT NULL,
    user_id     TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    session_id  TEXT REFERENCES auth_sessions(id) ON DELETE CASCADE,
    revoked     INTEGER DEFAULT 0,
    created_at  TEXT DEFAULT (datetime('now')),
    updated_at  TEXT
);

CREATE INDEX idx_auth_refresh_tokens_token ON auth_refresh_tokens(token);
CREATE INDEX idx_auth_refresh_tokens_session_id ON auth_refresh_tokens(session_id);
```

### Password Hashing

- **Algorithm**: bcrypt (`$2a$` variant)
- **Cost factor**: 10 (matches Supabase default)
- **Go implementation**: `golang.org/x/crypto/bcrypt` with `bcrypt.DefaultCost`

This ensures 100% compatibility - users migrating to Supabase keep their passwords.

---

## JWT Structure

JWTs match Supabase's structure exactly for supabase-js compatibility.

### Access Token Payload

```json
{
  "aud": "authenticated",
  "exp": 1640995200,
  "iat": 1640991600,
  "iss": "http://localhost:8080/auth/v1",
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "phone": "",
  "role": "authenticated",
  "aal": "aal1",
  "session_id": "abc123-session-id",
  "app_metadata": {
    "provider": "email",
    "providers": ["email"]
  },
  "user_metadata": {
    "name": "John Doe"
  }
}
```

### Token Configuration

| Token | Lifetime | Storage |
|-------|----------|---------|
| Access token | 1 hour (3600s) | Client-side only |
| Refresh token | 1 week | `auth_refresh_tokens` table |

### Signing

- **Algorithm**: HS256 (symmetric, like Supabase default)
- **Secret**: Configurable via `JWT_SECRET` env var
- **Future**: RS256 support for asymmetric keys

---

## Auth API Endpoints

GoTrue-compatible endpoints for supabase-js compatibility.

### Core Endpoints (MVP)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/v1/signup` | POST | Register new user |
| `/auth/v1/token?grant_type=password` | POST | Login with email/password |
| `/auth/v1/token?grant_type=refresh_token` | POST | Refresh access token |
| `/auth/v1/logout` | POST | Revoke session |
| `/auth/v1/user` | GET | Get current user |
| `/auth/v1/user` | PUT | Update user metadata |
| `/auth/v1/recover` | POST | Send password reset email |
| `/auth/v1/verify` | POST | Verify email/recovery token |
| `/auth/v1/verify` | GET | Verify via URL (redirects) |
| `/auth/v1/settings` | GET | Public auth configuration |
| `/auth/v1/resend` | POST | Resend confirmation email |

### Request/Response Examples

#### Signup

```http
POST /auth/v1/signup
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "created_at": "2026-01-16T10:00:00Z",
  "updated_at": "2026-01-16T10:00:00Z",
  "app_metadata": { "provider": "email", "providers": ["email"] },
  "user_metadata": {}
}
```

#### Login

```http
POST /auth/v1/token?grant_type=password
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "bearer",
  "expires_in": 3600,
  "refresh_token": "v1.refresh-token-here",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "role": "authenticated"
  }
}
```

---

## RLS (Row Level Security)

SQLite has no native RLS, so we implement it via query rewriting at the API layer.

### Policy Storage

```sql
CREATE TABLE _rls_policies (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name    TEXT NOT NULL,
    policy_name   TEXT NOT NULL,
    command       TEXT CHECK (command IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE', 'ALL')),
    using_expr    TEXT,  -- for SELECT, UPDATE, DELETE
    check_expr    TEXT,  -- for INSERT, UPDATE
    enabled       INTEGER DEFAULT 1,
    created_at    TEXT DEFAULT (datetime('now')),
    UNIQUE(table_name, policy_name)
);
```

### Example Policy

```sql
-- Users can only access their own todos
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr)
VALUES ('todos', 'user_isolation', 'ALL', 'user_id = auth.uid()');
```

### Query Rewriting Flow

1. Extract JWT claims from `Authorization: Bearer <token>` header
2. Load applicable RLS policies for target table
3. Replace auth functions with actual values:
   - `auth.uid()` → JWT `sub` claim
   - `auth.role()` → JWT `role` claim
   - `auth.email()` → JWT `email` claim
   - `auth.jwt()->>'key'` → Any JWT claim
4. Inject policy predicates as WHERE clauses
5. Execute rewritten query

### Example Transformation

```
Original:  SELECT * FROM todos
Rewritten: SELECT * FROM todos WHERE user_id = '550e8400-...'
```

### Security Model

- Rewriting happens before SQL execution
- Users cannot bypass RLS via the API
- Policies are enforced for all operations (SELECT, INSERT, UPDATE, DELETE)
- Table owners/admins can optionally bypass RLS

---

## REST API (PostgREST-Compatible)

### URL Structure

```
/rest/v1/{table}          # CRUD operations
/rest/v1/rpc/{function}   # RPC calls (future)
```

### CRUD Operations

| Operation | HTTP | URL Example |
|-----------|------|-------------|
| Select | GET | `/rest/v1/todos` |
| Insert | POST | `/rest/v1/todos` |
| Update | PATCH | `/rest/v1/todos?id=eq.1` |
| Upsert | POST | `/rest/v1/todos` + `Prefer: resolution=merge-duplicates` |
| Delete | DELETE | `/rest/v1/todos?id=eq.1` |

### Query Operators (MVP)

| Operator | Example | SQL |
|----------|---------|-----|
| `eq` | `status=eq.active` | `status = 'active'` |
| `neq` | `status=neq.deleted` | `status != 'deleted'` |
| `gt` | `age=gt.21` | `age > 21` |
| `gte` | `age=gte.21` | `age >= 21` |
| `lt` | `age=lt.65` | `age < 65` |
| `lte` | `age=lte.65` | `age <= 65` |
| `like` | `name=like.*john*` | `name LIKE '%john%'` |
| `ilike` | `name=ilike.*john*` | `LOWER(name) LIKE '%john%'` |
| `in` | `id=in.(1,2,3)` | `id IN (1, 2, 3)` |
| `is` | `deleted=is.null` | `deleted IS NULL` |

### Query Parameters

```
# Column selection
?select=id,title,completed

# Filtering
?completed=eq.false&user_id=eq.550e8400-...

# Ordering
?order=created_at.desc

# Pagination
?limit=10&offset=20

# Or via Range header
Range: 0-9
```

### Headers

```
# Request
Authorization: Bearer <jwt>
Content-Type: application/json
Prefer: return=representation    # Return inserted/updated rows

# Response
Content-Type: application/json
Content-Range: 0-9/100          # Pagination info
```

### supabase-js Compatibility

```javascript
import { createClient } from '@supabase/supabase-js'

// Point to Supabase Lite instead of supabase.co
const supabase = createClient('http://localhost:8080', 'your-anon-key')

// Works unchanged
const { data, error } = await supabase
  .from('todos')
  .select('id, title, completed')
  .eq('completed', false)
  .order('created_at', { ascending: false })
  .limit(10)
```

---

## Implementation Phases

### Phase 1: Foundation (MVP Core)

- [ ] Project scaffolding (Go modules, directory structure)
- [ ] SQLite integration with `modernc.org/sqlite` + WAL mode
- [ ] Auth schema tables
- [ ] Basic auth endpoints: `/signup`, `/token`, `/logout`, `/user`
- [ ] JWT issuance and validation (HS256)
- [ ] Password hashing (bcrypt, cost=10)
- [ ] Simple REST API: GET, POST, PATCH, DELETE
- [ ] Basic filtering: eq, neq, gt, lt, is
- [ ] CLI: `sblite serve`, `sblite init`

**Outcome:** Working auth + CRUD via supabase-js

### Phase 2: RLS & Security

- [ ] RLS policy storage table
- [ ] Query rewriting engine
- [ ] Auth function substitution (auth.uid(), auth.role())
- [ ] Policy enforcement on all CRUD
- [ ] Email verification flow
- [ ] Password recovery flow
- [ ] `/settings` endpoint
- [ ] CLI: `sblite policy add`, `sblite user create`

**Outcome:** Multi-tenant ready with row isolation

### Phase 3: API Completeness

- [ ] Remaining operators: like, ilike, in, or, and
- [ ] Column selection with relationships
- [ ] Range header pagination
- [ ] Upsert support
- [ ] Bulk operations
- [ ] Count queries
- [ ] OpenAPI schema generation

**Outcome:** Near-complete supabase-js compatibility

### Phase 4: Production Hardening

- [ ] Configuration (env vars, config file)
- [ ] HTTPS/TLS support
- [ ] Rate limiting
- [ ] Request logging & metrics
- [ ] Graceful shutdown
- [ ] Database migrations
- [ ] CLI: `sblite migrate`, `sblite backup`

**Outcome:** Production-deployable

### Future Phases

- **Realtime**: SSE subscriptions for data changes
- **Storage**: File uploads (local or S3)
- **RPC**: Custom SQL function calls
- **Admin UI**: Web dashboard

---

## Capabilities Summary

### What We CAN Do

| Feature | Approach | Compatibility |
|---------|----------|---------------|
| Single binary | Go + modernc.org/sqlite | Like PocketBase |
| Sub-second startup | No external deps | ✅ |
| Email/password auth | Native implementation | 100% Supabase |
| Password hashes | bcrypt, cost=10 | 100% migratable |
| JWT structure | Same claims | 100% Supabase |
| Auth schema | Mirror tables | Easy export |
| REST API | PostgREST syntax | supabase-js works |
| RLS policies | Query rewriting | Same logic |
| Scale to zero | Single process | ✅ |

### Limitations

| Feature | Limitation | Notes |
|---------|------------|-------|
| Write throughput | SQLite single-writer | Fine for <1000 writes/sec |
| Horizontal scaling | Single node | Migrate to Supabase when needed |
| Realtime | Not in MVP | Future phase |
| Storage | Not in MVP | Future phase |
| Edge Functions | Not planned | Use separate service |

### What We Won't Do

- Postgres wire protocol
- Full PostgreSQL compatibility
- GraphQL support
- Reuse GoTrue directly (requires PostgreSQL)

---

## Migration Path

```
Supabase Lite (dev/small scale)
         │
         │  Export:
         │  • auth_users → auth.users
         │  • app tables → public.*
         │  • RLS policies → CREATE POLICY statements
         │
         ▼
Supabase (production/scale)
```

### Export Process

1. Dump SQLite tables to SQL/CSV
2. Import into Supabase PostgreSQL
3. Run exported RLS policies (auto-generated, valid PostgreSQL)
4. Update client to point to new URL

Password hashes work without changes due to bcrypt compatibility.

### RLS Policy Export

Policies use Supabase-compatible syntax, so they export directly to valid PostgreSQL:

```sql
-- From _rls_policies table:
-- table_name: 'todos', policy_name: 'user_isolation',
-- command: 'ALL', using_expr: 'user_id = auth.uid()'

-- Exported as valid PostgreSQL:
ALTER TABLE todos ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_isolation ON todos
    FOR ALL
    USING (user_id = auth.uid());
```

**Why this works:**

| Expression | Supabase Lite | Supabase (PostgreSQL) |
|------------|---------------|----------------------|
| `auth.uid()` | Replaced at query rewrite time | Actual SQL function |
| `auth.role()` | Replaced at query rewrite time | Actual SQL function |
| `auth.jwt()->>'key'` | Replaced at query rewrite time | Actual SQL function |

The syntax is identical - only the implementation differs.

**Export command:**

```bash
sblite export --policies > policies.sql
sblite export --data > data.sql
sblite export --all > full-export.sql
```

---

## Configuration

### Environment Variables

```bash
# Server
SBLITE_HOST=0.0.0.0
SBLITE_PORT=8080

# Database
SBLITE_DB_PATH=./data.db

# Auth
SBLITE_JWT_SECRET=your-secret-key-min-32-chars
SBLITE_JWT_EXPIRY=3600
SBLITE_SITE_URL=http://localhost:8080

# Email (for verification/recovery)
SBLITE_SMTP_HOST=smtp.example.com
SBLITE_SMTP_PORT=587
SBLITE_SMTP_USER=user
SBLITE_SMTP_PASS=pass
SBLITE_SMTP_FROM=noreply@example.com
```

### CLI Commands

```bash
# Initialize new project
sblite init

# Start server
sblite serve
sblite serve --port 3000

# User management
sblite user create --email user@example.com --password secret
sblite user list

# Policy management
sblite policy add --table todos --name user_isolation --using "user_id = auth.uid()"
sblite policy list

# Database
sblite migrate
sblite backup --output backup.db
```

---

## References

- [Supabase Architecture](https://supabase.com/docs/guides/auth/architecture)
- [GoTrue GitHub](https://github.com/supabase/auth)
- [PostgREST Documentation](https://postgrest.org/en/stable/)
- [PocketBase](https://pocketbase.io/docs/)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
