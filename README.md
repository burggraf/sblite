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
| JWT sessions with refresh tokens | :white_check_mark: |
| REST API (CRUD operations) | :white_check_mark: |
| Query filters (eq, neq, gt, lt, like, in, etc.) | :white_check_mark: |
| Query modifiers (select, order, limit, offset) | :white_check_mark: |
| SQLite with WAL mode | :white_check_mark: |
| Row Level Security | :construction: Planned |
| Realtime subscriptions | :construction: Planned |
| File storage | :construction: Planned |

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
./sblite serve                      # Default: localhost:8080
./sblite serve --port 3000          # Custom port
./sblite serve --host 0.0.0.0       # Bind to all interfaces
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

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `SBLITE_JWT_SECRET` | (generated) | JWT signing secret (32+ characters recommended) |
| `SBLITE_HOST` | `localhost` | Server bind address |
| `SBLITE_PORT` | `8080` | Server port |
| `SBLITE_DB_PATH` | `./data.db` | SQLite database path |

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

### REST API (`/rest/v1`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/rest/v1/{table}` | GET | Select rows with filters |
| `/rest/v1/{table}` | POST | Insert rows |
| `/rest/v1/{table}` | PATCH | Update rows matching filters |
| `/rest/v1/{table}` | DELETE | Delete rows matching filters |

### Health

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |

## Documentation

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
│  /auth/v1/*  →  Auth Service        │
│  /rest/v1/*  →  REST Handler        │
├─────────────────────────────────────┤
│         SQLite (WAL mode)           │
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
