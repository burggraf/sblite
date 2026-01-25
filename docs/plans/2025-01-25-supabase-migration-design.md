# Supabase Migration Feature Design

**Date:** 2025-01-25
**Status:** Approved
**Feature:** Export & Migration Center

## Overview

A comprehensive migration feature that enables sblite users to migrate their entire project to Supabase through either an automated one-click process or manual export packages.

### Goals

- Enable seamless migration path from sblite to full Supabase
- Support both automated migration (Management API) and manual exports
- Provide verification tools to ensure migration success
- Allow rollback if issues occur
- Track migration state for resume capability

### Non-Goals

- Reverse migration (Supabase → sblite)
- Incremental/delta sync after initial migration
- Multi-project migration in single session

## Architecture

### Component Structure

```
internal/dashboard/
├── migration/
│   ├── service.go         # Orchestrates migration workflow
│   ├── supabase_client.go # Management API client
│   ├── state.go           # Persistence (migration state in DB)
│   ├── rollback.go        # Undo/cleanup operations
│   ├── exporters/         # Individual export generators
│   │   ├── schema.go      # DDL export (enhance existing)
│   │   ├── data.go        # Table data export
│   │   ├── functions.go   # Edge function export
│   │   ├── storage.go     # Bucket config + files
│   │   ├── auth.go        # Users, config, templates
│   │   └── rls.go         # RLS policies
│   └── verification/      # Post-migration checks
│       ├── basic.go       # Existence checks
│       ├── integrity.go   # Data comparison
│       └── functional.go  # Live tests
```

### Database Schema

```sql
-- Migration sessions
CREATE TABLE _migrations (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,          -- pending, in_progress, completed, failed, rolled_back
    supabase_project_ref TEXT,
    supabase_project_name TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT,
    error_message TEXT,
    credentials_encrypted TEXT     -- Encrypted API token, connection string
);

-- Individual items within a migration
CREATE TABLE _migration_items (
    id TEXT PRIMARY KEY,
    migration_id TEXT NOT NULL REFERENCES _migrations(id),
    item_type TEXT NOT NULL,       -- schema, data, users, rls, storage_buckets,
                                   -- storage_files, functions, secrets, auth_config,
                                   -- oauth_config, email_templates
    item_name TEXT,                -- e.g., table name, function name, bucket id
    status TEXT NOT NULL,          -- pending, in_progress, completed, failed, skipped
    started_at TEXT,
    completed_at TEXT,
    error_message TEXT,
    rollback_info TEXT,            -- JSON: info needed to undo this item
    metadata TEXT                  -- JSON: additional item-specific data
);

-- Verification results
CREATE TABLE _migration_verifications (
    id TEXT PRIMARY KEY,
    migration_id TEXT NOT NULL REFERENCES _migrations(id),
    layer TEXT NOT NULL,           -- basic, integrity, functional
    status TEXT NOT NULL,          -- pending, running, passed, failed
    started_at TEXT,
    completed_at TEXT,
    results TEXT                   -- JSON array of individual check results
);
```

## User Interface

### Tab Structure

```
┌─────────────────────────────────────────────────────────────┐
│  Export & Migration                                          │
├──────────┬──────────────────┬─────────────┬─────────────────┤
│  Export  │  Migrate (Auto)  │   Verify    │     History     │
└──────────┴──────────────────┴─────────────┴─────────────────┘
```

### Tab 1: Export (Manual Path)

Individual download buttons for each component:

| Export | Format | Contents |
|--------|--------|----------|
| Schema | `.sql` | PostgreSQL DDL |
| Data | `.json` / `.csv` | Per-table download |
| RLS Policies | `.sql` | CREATE POLICY statements |
| Auth Users | `.json` | User records (optional password exclusion) |
| Auth Config | `.json` | JWT settings, signup config, SMTP |
| Email Templates | `.json` | Subject, HTML, text per template |
| OAuth Config | `.json` | Provider settings (secrets redacted) |
| Storage Buckets | `.json` | Bucket configurations |
| Storage Files | `.zip` | Per-bucket download with folder structure |
| Edge Functions | `.zip` | Per-function or all, includes config |
| Secrets | `.txt` | Names only (values never exported) |

Each download includes README with import instructions.

### Tab 2: Migrate (Automated Path)

**Step 1 - Connect:** Input Management API token, select/create project

**Step 2 - Select:** Checklist of all items with status indicators:
- Database schema
- Database data (per-table selection)
- Auth users
- RLS policies
- Storage buckets
- Storage files
- Edge functions
- Secrets
- Auth configuration
- OAuth provider config
- Email templates

**Step 3 - Review:** Summary of what will be migrated, warnings

**Step 4 - Migrate:** Progress view with real-time status per item

### Tab 3: Verify

Three expandable verification layers:

1. **Basic Checks** (automatic) - Tables exist, functions deployed, buckets created
2. **Data Integrity** (user-triggered) - Row counts, sample comparison, FK validation
3. **Functional Tests** (user-triggered) - Test queries, file upload/download, function invocation

### Tab 4: History

List of past migrations with:
- Timestamps
- Status
- Items migrated
- Click to view details or resume incomplete migration

## Supabase Management API Integration

### Authentication

Users provide a Management API personal access token generated from supabase.com/dashboard/account/tokens.

### Endpoints Used

| Operation | Endpoint | Method |
|-----------|----------|--------|
| List projects | `/v1/projects` | GET |
| Get project details | `/v1/projects/{ref}` | GET |
| Get API keys | `/v1/projects/{ref}/api-keys` | GET |
| Deploy function | `/v1/projects/{ref}/functions/deploy?slug={name}` | POST |
| Create secrets | `/v1/projects/{ref}/secrets` | POST |
| Delete secrets | `/v1/projects/{ref}/secrets` | DELETE |
| Update auth config | `/v1/projects/{ref}/config/auth` | PATCH |
| Update storage config | `/v1/projects/{ref}/config/storage` | PATCH |

### Database Operations

Direct Postgres connection using credentials from Management API:
- Schema: Execute DDL statements
- Data: Bulk INSERT with transactions
- RLS policies: CREATE POLICY statements
- Auth users: INSERT into auth.users
- Storage buckets: INSERT into storage.buckets

### Storage File Upload

Use Supabase Storage API with service_role key:
- `POST /storage/v1/object/{bucket}/{path}`

## Dashboard API Endpoints

### Migration Management

```
POST   /_/api/migration/start           # Create new migration session
GET    /_/api/migration/{id}            # Get migration status
POST   /_/api/migration/{id}/connect    # Store Supabase credentials
GET    /_/api/migration/{id}/projects   # List user's Supabase projects
POST   /_/api/migration/{id}/select     # Select items to migrate
POST   /_/api/migration/{id}/run        # Start migration
POST   /_/api/migration/{id}/retry      # Retry failed items
POST   /_/api/migration/{id}/rollback   # Undo migration
DELETE /_/api/migration/{id}            # Cancel/delete migration
GET    /_/api/migrations                # List migration history
```

### Verification

```
POST   /_/api/migration/{id}/verify/basic       # Run basic checks
POST   /_/api/migration/{id}/verify/integrity   # Run data integrity
POST   /_/api/migration/{id}/verify/functional  # Run functional tests
GET    /_/api/migration/{id}/verify/results     # Get verification results
```

### Manual Setup

```
POST   /_/api/migration/{id}/manual/{item}/complete  # Mark manual step done
POST   /_/api/migration/{id}/manual/oauth/test       # Test OAuth config
```

### Export Downloads

```
GET    /_/api/export/schema             # Existing (enhanced)
GET    /_/api/export/data               # Existing (enhanced)
GET    /_/api/export/rls                # RLS policies
GET    /_/api/export/auth/users         # Auth users
GET    /_/api/export/auth/config        # Auth configuration
GET    /_/api/export/auth/templates     # Email templates
GET    /_/api/export/oauth              # OAuth config
GET    /_/api/export/storage/buckets    # Bucket configs
GET    /_/api/export/storage/files/{bucket}  # Bucket files
GET    /_/api/export/functions          # All functions
GET    /_/api/export/functions/{name}   # Single function
GET    /_/api/export/secrets            # Secret names
```

### WebSocket

```
WS /_/api/migration/{id}/progress    # Real-time migration status updates
```

## Error Handling & Rollback

### Error Handling Strategy

```go
type MigrationError struct {
    ItemType    string
    ItemName    string
    Operation   string
    Message     string
    Retryable   bool
    Suggestion  string
}
```

**Behavior:**
1. Mark item as `failed` with error details
2. Continue with remaining items
3. Show summary at end: X succeeded, Y failed, Z skipped
4. Failed items can be retried individually or in bulk

### Rollback Implementation

Rollback processes completed items in reverse order:

| Item Type | Rollback Action |
|-----------|-----------------|
| schema | DROP TABLE, DROP INDEX |
| function | DELETE function via API |
| storage_bucket | DELETE bucket (empty first) |
| storage_file | DELETE object |
| secret | DELETE via API |
| auth_user | DELETE from auth.users |
| rls_policy | DROP POLICY |

**Rollback Limitations:**
- Data deleted from Supabase cannot be recovered
- Auth users who logged in after migration may have new sessions
- Files uploaded after migration will remain
- Best-effort, not transactional

## Manual Setup Instructions

Items requiring manual setup are presented as an interactive checklist:

### OAuth Provider Apps

Expandable sections with:
1. Step-by-step instructions with screenshots
2. Pre-filled redirect URLs for user's Supabase project
3. Input fields for Client ID and Secret
4. "Test" button to validate OAuth flow works
5. Checkbox to mark complete

### Providers Covered

- Google OAuth
- GitHub OAuth
- (Future: Apple, Discord, etc.)

### Other Manual Items

- Custom domain DNS configuration
- External SMTP provider setup (if not using sblite config)

## Migration Items Detail

### What Gets Migrated

| Item | Source | Destination | Notes |
|------|--------|-------------|-------|
| Schema | `_columns`, SQLite tables | Supabase Postgres | Via DDL |
| Data | User tables | Supabase Postgres | Bulk INSERT |
| Auth Users | `auth_users` | `auth.users` | Bcrypt hashes transfer directly |
| Auth Identities | `auth_identities` | `auth.identities` | OAuth links |
| RLS Policies | `_rls_policies` | Postgres policies | CREATE POLICY |
| Storage Buckets | `storage_buckets` | `storage.buckets` | Config only |
| Storage Files | Local/S3 backend | Supabase Storage | Via Storage API |
| Edge Functions | `./functions/` directory | Supabase Functions | Via deploy API |
| Secrets | `_functions_secrets` | Supabase Secrets | Names + values |
| Auth Config | `_dashboard` settings | Auth config API | SMTP, JWT, etc. |
| OAuth Config | `_dashboard` settings | Auth config API | Provider settings |
| Email Templates | `auth_email_templates` | Auth config API | Custom templates |

### What Cannot Be Automated

- OAuth app creation in Google/GitHub consoles (credentials can be auto-configured once provided)
- Custom domain DNS records
- Billing/plan upgrades

## Verification System

### Layer 1: Basic Checks (Automatic)

- Tables exist with correct column count
- Functions deployed and callable
- Storage buckets created with correct settings
- RLS enabled on correct tables
- Secrets exist (names only)
- Auth config values match expected

### Layer 2: Data Integrity (User-triggered)

- Row count comparison per table
- Sample row comparison (first 10, last 10, random 10)
- Foreign key relationship validation
- Storage file count per bucket
- User count matches

### Layer 3: Functional Tests (User-triggered)

- Execute test query, verify result
- Upload test file, download, verify, delete
- Invoke function with test payload
- Create test user, sign in, delete (requires confirmation)

### Results Display

```
✅ Basic Checks (12/12 passed)
⚠️ Data Integrity (47/48 passed) [View Details]
⏸️ Functional Tests [Run Tests]
```

## Files to Create

```
internal/dashboard/migration/
├── service.go              # Migration orchestrator
├── supabase_client.go      # Management API client
├── state.go                # State persistence
├── rollback.go             # Rollback operations
├── exporters/
│   ├── schema.go           # Schema export (enhance existing)
│   ├── data.go             # Data export
│   ├── functions.go        # Functions export
│   ├── storage.go          # Storage export
│   ├── auth.go             # Auth export
│   └── rls.go              # RLS export
└── verification/
    ├── basic.go            # Basic checks
    ├── integrity.go        # Data integrity
    └── functional.go       # Functional tests

internal/dashboard/static/
├── migration.js            # Migration UI
└── migration.css           # Migration styles

docs/
└── migrating-to-supabase.md  # User documentation
```

## Files to Modify

- `internal/dashboard/handler.go` - New API endpoints
- `internal/dashboard/static/app.js` - Navigation updates
- `internal/dashboard/static/index.html` - New tab
- `internal/db/migrations.go` - New tables (_migrations, _migration_items, _migration_verifications)
- `internal/migrate/export.go` - Enhanced exports
- `README.md` - Add link to migration docs in Documentation section and update "Migration path" bullet
- `CLAUDE.md` - Add migration endpoints to API reference

## Documentation

### User Documentation Structure

```markdown
# Migrating to Supabase

## Overview
## Prerequisites
## Automated Migration
## Manual Migration
## Manual Setup Steps
## Troubleshooting
## Post-Migration Checklist
```

### Key Sections

- Generating Management API token
- Connecting to Supabase
- Selecting items to migrate
- Running and monitoring migration
- Verification steps
- Rollback procedures
- Manual setup for OAuth providers
- Import instructions for each export type
- Common errors and fixes

### README Updates

Update `README.md` to link to the new documentation:

1. **Update "Why Supabase Lite?" section** (line 11):
   ```markdown
   - **Migration path** - [One-way migration](docs/migrating-to-supabase.md) to full Supabase when you outgrow it
   ```

2. **Add to Documentation → Guides section** (after line 322):
   ```markdown
   - [Migrating to Supabase](docs/migrating-to-supabase.md) - Export data and migrate to full Supabase with one-click or manual process
   ```

## Implementation Notes

### Security Considerations

- Management API tokens encrypted at rest in `_migrations.credentials_encrypted`
- Tokens never logged or exposed in error messages
- Secrets exported by name only in manual exports
- OAuth client secrets redacted in config exports

### Performance Considerations

- Large data migrations use batched INSERTs
- Storage file uploads parallelized (configurable concurrency)
- Progress updates via WebSocket to avoid polling
- Verification checks use sampling for large tables

### State Management

- All migration state persisted server-side
- Resume capability from any device
- Migration history preserved for audit
- Automatic cleanup of old migrations (configurable retention)

## Summary

| Decision | Choice |
|----------|--------|
| Auth method | Supabase Management API token |
| Migration scope | All 11 item types, user-selectable |
| UI structure | Hybrid tabs (Export, Migrate, Verify, History) |
| Verification | 3 layers, individually runnable |
| Manual guidance | Interactive checklist with test buttons |
| Error handling | Continue on error, full rollback option |
| Export format | Individual downloads per component |
| State persistence | Server-side in sblite database |
| Location | Dashboard → Export & Migration |
