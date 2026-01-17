# sblite Dashboard Design

## Overview

A web-based dashboard for sblite that provides a GUI for managing tables, RLS policies, data, and settings. Inspired by Supabase's dashboard (simpler) and Pocketbase's approach. All schema changes generate migration files for complete version control history.

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Frontend tech | Vanilla JS + CSS | No build step, single-binary philosophy, easy to modify |
| Authentication | Setup password (Pocketbase-style) | Simple, familiar, like database passwords |
| URL path | `/_/` | Clean, won't conflict with user tables |
| Migration storage | Database + filesystem | Track state in DB, generate .sql files for version control |
| Migration format | Timestamp prefix | Avoids branch conflicts (e.g., `20260117143022_create_posts.sql`) |
| Rollback support | Forward-only | Simpler; create new migration to fix issues |
| Theme | Light + dark, default dark | Match Supabase aesthetic |

## Architecture

```
sblite serve
    │
    ├── /_/                    Dashboard UI (embedded static files)
    ├── /_/api/                Dashboard API (internal)
    │   ├── /auth              Login/setup
    │   ├── /schema            Tables, columns, migrations
    │   ├── /data              Table data CRUD
    │   ├── /policies          RLS policies
    │   └── /settings          Configuration
    │
    ├── /auth/v1/*             Existing auth API (unchanged)
    ├── /rest/v1/*             Existing REST API (unchanged)
    └── /admin/v1/*            Existing admin API (unchanged)
```

**Principles:**
- Dashboard API is separate from public APIs (uses different auth)
- All schema changes route through the migration system
- Frontend is vanilla JS embedded via `//go:embed`
- Setup password stored hashed in database

---

## Migration System

### Database Tracking

```sql
CREATE TABLE _schema_migrations (
    version TEXT PRIMARY KEY,      -- '20260117143022'
    name TEXT,                     -- 'create_posts'
    statements TEXT,               -- JSON array of SQL statements
    applied_at TEXT NOT NULL       -- ISO 8601 timestamp
);
```

### File Format

Location: `./migrations/`

Example: `./migrations/20260117143022_create_posts.sql`

```sql
-- Migration: create_posts
-- Created: 2026-01-17T14:30:22Z

CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary)
VALUES
    ('posts', 'id', 'uuid', 0, 1),
    ('posts', 'title', 'text', 0, 0),
    ('posts', 'body', 'text', 1, 0),
    ('posts', 'created_at', 'timestamptz', 1, 0);
```

### CLI Commands (Supabase-aligned)

```bash
sblite migration new <name>     # Create timestamped migration file
sblite migration list           # Show applied/pending migrations
sblite db push                  # Apply pending migrations
sblite db reset                 # Drop all, rerun migrations from scratch
sblite db diff                  # Show unapplied schema changes as SQL
```

### Execution Flow

1. Dashboard generates SQL for the change
2. SQL written to `./migrations/` with timestamp
3. Migration executed in transaction
4. Record inserted into `_schema_migrations`
5. On failure: transaction rolls back, file deleted

---

## Dashboard Authentication

### Setup Password Flow

1. First visit to `/_/` → setup screen if no password exists
2. User sets password → hashed and stored
3. Subsequent visits → login screen
4. Session stored as HTTP-only cookie

### Database Storage

```sql
CREATE TABLE _dashboard (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Stores:
-- ('password_hash', '$2a$10$...')      -- bcrypt hash
-- ('session_secret', '...')            -- for signing session cookies
-- ('theme', 'dark')                    -- user preference
```

### Session Management

- Cookie-based sessions (not JWT)
- Cookie name: `_sblite_session` (HTTP-only, SameSite=Strict)
- Session expires after 24 hours of inactivity
- Single concurrent session (new login invalidates old)

### API Endpoints

```
POST /_/api/auth/setup     # Set initial password (first time only)
POST /_/api/auth/login     # Login with password
POST /_/api/auth/logout    # Clear session
GET  /_/api/auth/status    # Check if logged in / needs setup
```

### Security

- Rate limiting on login attempts (5 per minute)
- Password minimum 8 characters
- HTTPS recommended in production (warning shown if HTTP)

### CLI Commands

```bash
sblite dashboard setup              # Interactive: prompt for password
sblite dashboard setup --password   # Set password (prompts securely)
sblite dashboard reset-password     # Reset password (prompts securely)
sblite dashboard disable            # Disable dashboard entirely
sblite dashboard enable             # Re-enable dashboard
```

**Behavior:**
- `setup` only works if no password exists (first time)
- `reset-password` works anytime, invalidates all existing sessions
- Passwords never echoed or logged
- `sblite init` optionally prompts "Set dashboard password now? [y/N]"

---

## Dashboard UI Structure

### Layout

```
┌─────────────────────────────────────────────────────┐
│  sblite                              [◐] [logout]   │
├──────────┬──────────────────────────────────────────┤
│          │                                          │
│  Tables  │   [Main content area]                    │
│  ────────│                                          │
│  • posts │                                          │
│  • users │                                          │
│          │                                          │
│  Auth    │                                          │
│  ────────│                                          │
│  Users   │                                          │
│          │                                          │
│  Policies│                                          │
│          │                                          │
│  Settings│                                          │
│          │                                          │
│  Logs    │                                          │
│          │                                          │
└──────────┴──────────────────────────────────────────┘
```

### Sections

| Section | Purpose |
|---------|---------|
| **Tables** | List user tables, click to browse/edit data and schema |
| **Auth > Users** | View/manage auth_users (email, metadata, etc.) |
| **Policies** | List/create/edit RLS policies with SQL templates |
| **Settings** | JWT config, API keys, logging, email settings |
| **Logs** | View recent request logs (if logging enabled) |

- Theme toggle `[◐]` switches light/dark, persists to `_dashboard` table
- Sidebar collapses to hamburger menu on narrow screens

---

## Table Editor & Schema Management

### Table List View

- Shows all user tables from `_columns` metadata
- Displays row count, column count
- Quick actions: Browse data, Edit schema, Delete

### Schema Editor

```
┌─────────────────────────────────────────────────────┐
│  Create Table                            [Cancel]   │
├─────────────────────────────────────────────────────┤
│  Table name: [posts____________]                    │
│                                                     │
│  Columns:                                           │
│  ┌──────────┬──────────┬──────┬─────────┬────────┐  │
│  │ Name     │ Type     │ Null │ Default │ PK     │  │
│  ├──────────┼──────────┼──────┼─────────┼────────┤  │
│  │ id       │ uuid [▼] │ [ ]  │ gen_uuid│ [✓]    │  │
│  │ title    │ text [▼] │ [ ]  │         │ [ ]    │  │
│  │ body     │ text [▼] │ [✓]  │         │ [ ]    │  │
│  │ created  │ tstz [▼] │ [ ]  │ now()   │ [ ]    │  │
│  └──────────┴──────────┴──────┴─────────┴────────┘  │
│  [+ Add Column]                                     │
│                                                     │
│  ─── Migration Preview ───────────────────────────  │
│  CREATE TABLE posts (                               │
│      id TEXT PRIMARY KEY DEFAULT (uuid()),          │
│      ...                                            │
│  );                                                 │
│                                                     │
│  [Save & Generate Migration]                        │
└─────────────────────────────────────────────────────┘
```

### Behavior

- Type dropdown: uuid, text, integer, numeric, boolean, timestamptz, jsonb, bytea
- Default values: `now()`, `gen_uuid()`, or custom literal
- Live migration preview updates as you edit
- "Save" always generates a migration file first, then applies it

### Editing Existing Tables

- Add column → generates `ALTER TABLE ADD COLUMN` migration
- Drop column → generates `ALTER TABLE DROP COLUMN` migration
- Rename column → generates appropriate migration
- Cannot change type (SQLite limitation) → must drop/recreate

---

## Data Browser

### Table Data View

```
┌─────────────────────────────────────────────────────┐
│  posts                      [+ Add Row] [Schema]    │
├─────────────────────────────────────────────────────┤
│  Filter: [column▼] [eq▼] [value____] [+] [Apply]    │
├─────────────────────────────────────────────────────┤
│  [ ] │ id         │ title        │ created_at      │
│  ────┼────────────┼──────────────┼─────────────────│
│  [ ] │ 7a3f...    │ Hello World  │ 2026-01-17T...  │
│  [ ] │ 9c2b...    │ Second Post  │ 2026-01-17T...  │
├─────────────────────────────────────────────────────┤
│  ◀ 1 2 3 ... ▶        Showing 1-25 of 142 rows     │
└─────────────────────────────────────────────────────┘
│  [Delete Selected]                                  │
```

### Features

| Feature | Behavior |
|---------|----------|
| **Pagination** | 25/50/100 rows per page |
| **Filtering** | Column + operator + value, multiple filters with AND |
| **Sorting** | Click column header to sort asc/desc |
| **Inline edit** | Click cell to edit, blur or Enter to save |
| **Add row** | Modal form with fields for each column |
| **Delete** | Checkbox selection + bulk delete button |
| **Copy cell** | Click to copy UUID/long values |

### Cell Rendering by Type

- `uuid` → truncated with copy button (7a3f...bc12)
- `jsonb` → expandable preview, click to edit in modal
- `boolean` → toggle switch
- `timestamptz` → formatted datetime, edit via picker
- `text` → inline editable, multiline for long content

**Note:** Data changes do NOT generate migrations (only schema changes do).

---

## RLS Policy Editor

### Policy List View

```
┌─────────────────────────────────────────────────────┐
│  RLS Policies                        [+ New Policy] │
├─────────────────────────────────────────────────────┤
│  posts                                              │
│  ├─ select_own_posts     SELECT  ✓ enabled         │
│  ├─ insert_own_posts     INSERT  ✓ enabled         │
│  └─ update_own_posts     UPDATE  ✓ enabled         │
│                                                     │
│  comments                                           │
│  ├─ public_read          SELECT  ✓ enabled         │
│  └─ author_write         ALL     ○ disabled        │
└─────────────────────────────────────────────────────┘
```

### Policy Editor

```
┌─────────────────────────────────────────────────────┐
│  New Policy                              [Cancel]   │
├─────────────────────────────────────────────────────┤
│  Table: [posts ▼]                                   │
│  Name:  [select_own_posts____]                      │
│                                                     │
│  Operations: [✓] SELECT [ ] INSERT [ ] UPDATE [ ] DELETE │
│                                                     │
│  ─── Start from template ───────────────────────── │
│  [Owner-based] [Public read] [Authenticated] [Custom] │
│                                                     │
│  ─── Policy expression (SQL) ──────────────────── │
│  ┌─────────────────────────────────────────────┐   │
│  │ (select auth.uid()) = user_id               │   │
│  │                                             │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  ─── Migration Preview ───────────────────────────  │
│  INSERT INTO _rls_policies (table_name, name, ...)  │
│  VALUES ('posts', 'select_own_posts', ...);         │
│                                                     │
│  [Save & Generate Migration]                        │
└─────────────────────────────────────────────────────┘
```

### Templates

| Template | Expression | Use case |
|----------|------------|----------|
| **Owner-based** | `(select auth.uid()) = user_id` | User owns the row |
| **Public read** | `true` | Anyone can read |
| **Authenticated only** | `(select auth.uid()) IS NOT NULL` | Logged-in users only |
| **Custom** | (empty) | Write your own |

- Owner-based auto-detects `user_id`, `author_id`, `owner_id` columns
- Shows warning if selected column doesn't exist
- Uses `(select auth.uid())` for performance (Supabase best practice)

---

## Settings Panel

### Settings Tabs

```
┌─────────────────────────────────────────────────────┐
│  Settings                                           │
├─────────────────────────────────────────────────────┤
│  [API Keys] [Auth] [Email] [Logging] [Dashboard]    │
├─────────────────────────────────────────────────────┤
│                                                     │
│  API Keys                                           │
│  ─────────────────────────────────────────────────  │
│  Anon Key:                                          │
│  [eyJhbG...truncated...] [Copy] [Regenerate]        │
│                                                     │
│  Service Role Key:                                  │
│  [eyJhbG...truncated...] [Copy] [Regenerate]        │
│                                                     │
│  ⚠ Regenerating keys invalidates existing clients   │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### Settings by Tab

| Tab | Settings |
|-----|----------|
| **API Keys** | View/copy/regenerate anon & service role keys |
| **Auth** | JWT expiry (access/refresh), password min length, require email confirmation |
| **Email** | Mail mode (log/catch/smtp), SMTP settings if applicable |
| **Logging** | Log level, log mode (console/file/db), retention |
| **Dashboard** | Change password, theme preference |

### Behavior

- Settings requiring restart show: `⟲ Requires restart`
- After changing: "Restart sblite to apply changes"
- Most settings stored in `_dashboard` table as key-value pairs
- Env-only settings (like JWT secret) shown as read-only: "Set via SBLITE_JWT_SECRET"

---

## Logs Viewer

### Log View

```
┌─────────────────────────────────────────────────────┐
│  Logs                              [Auto-refresh ◉] │
├─────────────────────────────────────────────────────┤
│  Level: [All ▼]  Source: [All ▼]  [Search______] 🔍 │
├─────────────────────────────────────────────────────┤
│  TIME        LEVEL  SOURCE   MESSAGE                │
│  ─────────────────────────────────────────────────  │
│  14:32:01    INFO   rest     GET /rest/v1/posts 200 │
│  14:31:58    DEBUG  auth     JWT validated user_123 │
│  14:31:45    WARN   rls      Policy denied: sel...  │
│  14:31:30    ERROR  rest     Invalid filter: fo...  │
│  ─────────────────────────────────────────────────  │
│  [Load more...]                                     │
└─────────────────────────────────────────────────────┘
```

### Features

| Feature | Behavior |
|---------|----------|
| **Filter by level** | DEBUG, INFO, WARN, ERROR, or All |
| **Filter by source** | auth, rest, rls, admin, or All |
| **Search** | Full-text search in message |
| **Auto-refresh** | Poll for new logs every 2s (toggle on/off) |
| **Click to expand** | Show full message, request details, stack trace |
| **Time range** | Last hour / 24h / 7d / custom range |

### Data Source

- If `--log-mode=database`: Query `log.db` directly
- If `--log-mode=file`: Read and parse log file (limited)
- If `--log-mode=console`: Show "Enable database logging to view logs here"

---

## File Structure

### New Packages

```
internal/
├── dashboard/
│   ├── handler.go          # HTTP handlers, static file serving
│   ├── auth.go             # Setup password, sessions, cookies
│   ├── api.go              # /_/api/* endpoint handlers
│   └── static/
│       ├── index.html      # SPA shell
│       ├── app.js          # Vanilla JS application
│       └── style.css       # Light/dark theme styles
│
├── migration/
│   ├── migration.go        # Core migration logic
│   ├── generator.go        # SQL generation from schema changes
│   ├── runner.go           # Apply migrations, track state
│   └── diff.go             # Compare schema states
```

### New CLI Commands

```
cmd/
├── migration_new.go        # sblite migration new <name>
├── migration_list.go       # sblite migration list
├── db_push.go              # sblite db push
├── db_reset.go             # sblite db reset
├── db_diff.go              # sblite db diff
├── dashboard.go            # sblite dashboard setup/reset-password/enable/disable
```

### New Database Tables

| Table | Purpose |
|-------|---------|
| `_schema_migrations` | Track applied migrations |
| `_dashboard` | Dashboard settings, password hash |

### Embedded Assets

All static files compiled into binary via `//go:embed`, following the existing mail viewer pattern.

---

## Implementation Phases

### Phase 1: Migration System Foundation
- Create `internal/migration/` package
- Implement `_schema_migrations` table
- CLI commands: `migration new`, `migration list`, `db push`
- File generation with timestamps

### Phase 2: Dashboard Shell
- Create `internal/dashboard/` package
- Setup password authentication
- Basic SPA shell with routing
- Theme toggle (light/dark)

### Phase 3: Table Management
- Table list view
- Schema editor with migration preview
- CREATE TABLE via dashboard
- ALTER TABLE (add/drop column) via dashboard

### Phase 4: Data Browser
- Paginated table view
- Filtering and sorting
- Inline editing
- Add/delete rows

### Phase 5: RLS Policy Editor
- Policy list view
- Policy editor with templates
- Migration generation for policies

### Phase 6: Settings & Logs
- Settings panel with all tabs
- API key management
- Logs viewer

### Phase 7: Polish
- `db reset` and `db diff` commands
- Error handling and edge cases
- Responsive design refinement
- Documentation

---

## Open Questions

None at this time - design is complete and ready for implementation planning.
