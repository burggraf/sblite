# Supabase Lite

A lightweight, single-binary backend that provides a subset of Supabase functionality. Built for fast startup, scale-to-zero deployments, and seamless migration to full Supabase when needed.

## Why Supabase Lite?

- **Fast startup** - Sub-second cold starts
- **Scale to zero** - No idle resource consumption
- **Single binary** - Easy deployment with no external dependencies
- **Supabase compatible** - Works with the official `@supabase/supabase-js` client
- **Migration path** - One-way migration to full Supabase when you outgrow it

## Features

| Category | Status |
|----------|--------|
| Email/password authentication | :white_check_mark: |
| OAuth authentication (Google, GitHub) | :white_check_mark: |
| JWT sessions with refresh tokens | :white_check_mark: |
| REST API (CRUD operations) | :white_check_mark: |
| Query filters (eq, neq, gt, lt, like, in, etc.) | :white_check_mark: |
| Query modifiers (select, order, limit, offset) | :white_check_mark: |
| SQLite with WAL mode | :white_check_mark: |
| Email system (log, catch, SMTP) | :white_check_mark: |
| Magic link authentication | :white_check_mark: |
| User invitations | :white_check_mark: |
| Configurable logging (console/file/database) | :white_check_mark: |
| Row Level Security | :white_check_mark: |
| Web Dashboard | :white_check_mark: |
| Full-text search (FTS5) | :white_check_mark: |
| PostgreSQL syntax translation | :white_check_mark: |
| Edge Functions | :white_check_mark: |
| Realtime subscriptions | :construction: Planned |
| File storage | :white_check_mark: |

## Quick Start

### Build

```bash
go build -o sblite
```

### Initialize

```bash
./sblite init
```

### Start the server

```bash
./sblite serve                        # Default: localhost:8080
./sblite serve --port 3000            # Custom port
./sblite serve --host 0.0.0.0         # Bind to all interfaces
./sblite serve --mail-mode=catch      # Development: captures emails, web UI at /mail
./sblite serve --mail-mode=smtp       # Production: sends real emails via SMTP
```

### Access the Dashboard

The built-in web dashboard is available at `http://localhost:8080/_`

On first access, you'll be prompted to set an admin password. The dashboard provides:
- **Table Management** - Create, modify, and delete tables with typed schemas
- **Data Browser** - View, filter, sort, and edit rows with pagination
- **Schema Editor** - Add, rename, and remove columns

You can also set the password via CLI:
```bash
./sblite dashboard setup              # Set initial password
./sblite dashboard reset-password     # Reset password and clear sessions
```

### Use with Supabase client

```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key', {
  auth: { autoRefreshToken: false, persistSession: false }
})

// Sign up
const { data, error } = await supabase.auth.signUp({
  email: 'user@example.com',
  password: 'password123'
})

// Or sign in with OAuth
await supabase.auth.signInWithOAuth({
  provider: 'google',
  options: { redirectTo: 'http://localhost:3000/callback' }
})

// Query data
const { data: todos } = await supabase
  .from('todos')
  .select('id, title, completed')
  .eq('completed', false)
  .order('created_at', { ascending: false })
  .limit(10)

// Insert
await supabase.from('todos').insert({ title: 'New task' })

// Update
await supabase.from('todos').update({ completed: true }).eq('id', 1)

// Delete
await supabase.from('todos').delete().eq('id', 1)
```

## Configuration

### Core Settings

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `SBLITE_JWT_SECRET` | (generated) | JWT signing secret (32+ characters recommended) |
| `SBLITE_HOST` | `localhost` | Server bind address |
| `SBLITE_PORT` | `8080` | Server port |
| `SBLITE_DB_PATH` | `./data.db` | SQLite database path |

### Email Settings

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `SBLITE_MAIL_MODE` | `log` | Email mode: `log`, `catch`, or `smtp` |
| `SBLITE_MAIL_FROM` | `noreply@localhost` | Default sender email address |
| `SBLITE_SITE_URL` | `http://localhost:8080` | Base URL for email links |
| `SBLITE_SMTP_HOST` | - | SMTP server hostname (required for smtp mode) |
| `SBLITE_SMTP_PORT` | `587` | SMTP server port |
| `SBLITE_SMTP_USER` | - | SMTP username |
| `SBLITE_SMTP_PASS` | - | SMTP password |

See [Email System Documentation](docs/EMAIL.md) for detailed configuration and usage.

### Logging Settings

| Environment Variable | CLI Flag | Default | Description |
|---------------------|----------|---------|-------------|
| `SBLITE_LOG_MODE` | `--log-mode` | `console` | Output: `console`, `file`, or `database` |
| `SBLITE_LOG_LEVEL` | `--log-level` | `info` | Level: `debug`, `info`, `warn`, `error` |
| `SBLITE_LOG_FORMAT` | `--log-format` | `text` | Format: `text` or `json` |
| `SBLITE_LOG_FILE` | `--log-file` | `sblite.log` | Log file path (file mode) |
| `SBLITE_LOG_DB` | `--log-db` | `log.db` | Log database path (database mode) |
| `SBLITE_LOG_MAX_SIZE` | `--log-max-size` | `100` | Max file size in MB before rotation |
| `SBLITE_LOG_MAX_AGE` | `--log-max-age` | `7` | Days to retain old logs |
| `SBLITE_LOG_MAX_BACKUPS` | `--log-max-backups` | `3` | Number of backup files to keep |
| `SBLITE_LOG_FIELDS` | `--log-fields` | - | DB fields: `source,request_id,user_id,extra` |

**Examples:**
```bash
# JSON output for log aggregators
./sblite serve --log-format=json

# File logging with rotation
./sblite serve --log-mode=file --log-file=/var/log/sblite.log

# Database logging for queryable logs
./sblite serve --log-mode=database --log-fields=request_id,user_id
```

## API Endpoints

### Authentication (`/auth/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/v1/signup` | POST | Create new user |
| `/auth/v1/token?grant_type=password` | POST | Sign in with email/password |
| `/auth/v1/token?grant_type=refresh_token` | POST | Refresh access token |
| `/auth/v1/logout` | POST | Sign out (invalidate session) |
| `/auth/v1/user` | GET | Get current user |
| `/auth/v1/user` | PUT | Update current user |
| `/auth/v1/recover` | POST | Request password recovery email |
| `/auth/v1/verify` | POST/GET | Verify email or reset password |
| `/auth/v1/magiclink` | POST | Request magic link login email |
| `/auth/v1/resend` | POST | Resend confirmation or recovery email |
| `/auth/v1/invite` | POST | Invite new user (admin only) |
| `/auth/v1/authorize` | GET | Initiate OAuth flow |
| `/auth/v1/callback` | GET | OAuth provider callback |
| `/auth/v1/user/identities` | GET | List linked OAuth identities |
| `/auth/v1/user/identities/{provider}` | DELETE | Unlink OAuth provider |
| `/auth/v1/settings` | GET | Get auth settings (includes OAuth) |

### REST API (`/rest/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/rest/v1/{table}` | GET | Select rows with filters |
| `/rest/v1/{table}` | POST | Insert rows |
| `/rest/v1/{table}` | PATCH | Update rows matching filters |
| `/rest/v1/{table}` | DELETE | Delete rows matching filters |

### Storage API (`/storage/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/storage/v1/bucket` | GET | List all buckets |
| `/storage/v1/bucket` | POST | Create a new bucket |
| `/storage/v1/bucket/{id}` | GET | Get bucket details |
| `/storage/v1/bucket/{id}` | PUT | Update bucket settings |
| `/storage/v1/bucket/{id}` | DELETE | Delete an empty bucket |
| `/storage/v1/object/list/{bucket}` | POST | List objects in bucket |
| `/storage/v1/object/{bucket}/*` | POST | Upload a file |
| `/storage/v1/object/{bucket}/*` | GET | Download a file |
| `/storage/v1/object/{bucket}/*` | DELETE | Delete a file |
| `/storage/v1/object/public/{bucket}/*` | GET | Download from public bucket (no auth) |
| `/storage/v1/object/copy` | POST | Copy a file |
| `/storage/v1/object/move` | POST | Move/rename a file |

### Dashboard (`/_`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_/` | GET | Dashboard web interface |
| `/_/api/auth/status` | GET | Check auth/setup status |
| `/_/api/auth/setup` | POST | Set initial password |
| `/_/api/auth/login` | POST | Login to dashboard |
| `/_/api/auth/logout` | POST | Logout from dashboard |
| `/_/api/tables` | GET | List all tables |
| `/_/api/tables` | POST | Create table |
| `/_/api/tables/{name}` | GET | Get table schema |
| `/_/api/tables/{name}` | DELETE | Drop table |
| `/_/api/data/{table}` | GET | Select rows (paginated) |
| `/_/api/data/{table}` | POST | Insert row |
| `/_/api/data/{table}` | PATCH | Update rows |
| `/_/api/data/{table}` | DELETE | Delete rows |

### Health

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |

## Documentation

### Guides
- [Authentication](docs/AUTH.md) - Email confirmation, password recovery, magic links, and session management
- [OAuth Authentication](docs/OAUTH.md) - Google and GitHub OAuth setup, API reference, and troubleshooting
- [Email System](docs/EMAIL.md) - Complete guide to email modes, configuration, and authentication flows
- [Logging System](docs/LOGGING.md) - Logging modes, rotation, database queries, and configuration
- [Full-Text Search](docs/full-text-search.md) - FTS5 indexing, query types, and Supabase textSearch compatibility
- [PostgreSQL Translation](docs/postgres-translation.md) - Write PostgreSQL syntax that translates to SQLite automatically
- [Storage API](docs/STORAGE.md) - File uploads, downloads, buckets, and public access
- [Edge Functions](docs/edge-functions.md) - Serverless TypeScript/JavaScript functions with secrets and configuration

### Design & Implementation
- [Design Document](docs/plans/2026-01-16-supabase-lite-design.md) - Architecture, schema design, and roadmap
- [Phase 1 Implementation Plan](docs/plans/2026-01-16-phase1-foundation.md) - Foundation implementation details

### Testing & Compatibility
- [E2E Test Suite](e2e/README.md) - Setup, installation, and how to run tests
- [Test Inventory](e2e/TESTS.md) - Complete list of all 173 test cases with status
- [Compatibility Matrix](e2e/COMPATIBILITY.md) - Feature-by-feature Supabase compatibility status

The test suite validates compatibility with the official `@supabase/supabase-js` client. Tests are based on examples from the [Supabase JavaScript documentation](https://supabase.com/docs/reference/javascript/introduction).

## Running Tests

### Go unit tests

```bash
go test ./...
```

### E2E tests (requires Node.js 18+)

```bash
cd e2e
npm install
npm run setup    # Initialize test database
npm test         # Run all tests
```

## Architecture

```
┌─────────────────────────────────────┐
│     @supabase/supabase-js client    │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│      Supabase Lite (Go binary)      │
├─────────────────────────────────────┤
│  /auth/v1/*     →  Auth Service     │
│  /rest/v1/*     →  REST Handler     │
│  /storage/v1/*  →  Storage Service  │
│  /_/*           →  Web Dashboard    │
│  /mail/*        →  Mail Viewer      │
├─────────────────────────────────────┤
│  Email Service (log/catch/smtp)     │
├─────────────────────────────────────┤
│  SQLite (WAL mode)  │  Local Storage│
└─────────────────────────────────────┘
```

## Tech Stack

- **Go 1.25** - Core runtime
- **Chi** - HTTP router
- **modernc.org/sqlite** - Pure Go SQLite (no CGO)
- **golang-jwt** - JWT signing/validation
- **bcrypt** - Password hashing
- **Cobra** - CLI framework

## License

MIT
