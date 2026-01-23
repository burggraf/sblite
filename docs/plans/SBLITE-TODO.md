# sblite Remaining Work

This document tracks main app features that haven't been implemented yet. Reference: `e2e/COMPATIBILITY.md`, `docs/plans/2026-01-16-supabase-lite-design.md`

## Future: sblite-hub (Multi-Tenant Control Plane)

A control plane for managing thousands of sblite instances across organizations and projects. Enables multi-tenancy at the org level with scale-to-zero capability.

**Design:** `docs/plans/2026-01-23-sblite-hub-design.md`
**Implementation Plan:** `docs/plans/2026-01-23-sblite-hub-implementation.md`

### Key Features
- **Multi-tenancy:** Organizations with multiple projects, per-project roles (owner/admin/developer/viewer)
- **Scale-to-zero:** Idle instances automatically shut down, wake on demand
- **Hybrid deployment:** Shared org instances + dedicated instances for high-traffic projects
- **Pluggable orchestration:** Process, Docker, and Kubernetes backends
- **Dogfooding:** Hub uses its own sblite instance for control plane data

### Implementation Phases

| Phase | Description | Status |
|-------|-------------|--------|
| 1. Foundation | Module, config, store layer, serve command | Planned |
| 2. Project Management | Org/project/member API handlers | Planned |
| 3. Proxy & Routing | Subdomain routing, reverse proxy, multi-project sblite mode | Planned |
| 4. Scale-to-Zero | Orchestrator interface, process impl, idle detection, wake-up | Planned |
| 5. Docker | DockerOrchestrator, docker-compose example | Planned |
| 6. Dashboard | Hub web UI for org/project management | Planned |
| 7. Kubernetes | KubernetesOrchestrator, Helm chart | Planned |
| 8. Production | Multi-hub federation, PostgreSQL support, rate limiting | Planned |

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                     sblite-hub                          │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │  Dashboard  │  │  Proxy/Router│  │  Orchestrator │  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
│                          │                              │
│  ┌───────────────────────▼────────────────────────┐    │
│  │  Internal sblite instance (dogfooding)          │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │  sblite    │  │  sblite    │  │  sblite    │
   │  Org A     │  │  Org B     │  │  Dedicated │
   └────────────┘  └────────────┘  └────────────┘
```

**Priority:** Future
**Effort:** Very Large (8 phases, ~32 tasks)

---

## Major Features

### ~~Vector Search (pgvector-compatible)~~ ✅ COMPLETE
AI/ML similarity search for embeddings, RAG applications, and recommendations.

```javascript
// Client usage
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: [0.1, 0.2, ...],
  match_count: 10,
  match_threshold: 0.7,
  metric: 'cosine'  // or 'l2', 'dot'
});
```

**Implemented:**
- `vector(N)` type in type system with dimension validation
- Built-in `vector_search` RPC function
- Distance metrics: cosine similarity, L2 (Euclidean), dot product
- Configurable match count and similarity threshold
- Optional column selection and filtering
- Dashboard UI for creating vector columns
- PostgreSQL DDL export with pgvector syntax
- Demo app: `test_apps/vector-feld` (Seinfeld script search with Gemini embeddings)

**Architecture:**
- Vectors stored as JSON arrays in TEXT columns
- Brute-force similarity search (streaming, memory-efficient)
- ~100-500ms latency for 50k vectors

See `docs/plans/2026-01-18-vector-search-design.md` for full design.

---

### ~~PostgreSQL Functions (RPC)~~ ✅ COMPLETE (Phase C)
Supabase RPC compatibility for calling server-side functions via `/rest/v1/rpc/{name}`.

```javascript
// Client usage
const { data } = await supabase.rpc('get_user_orders', { user_id: '...' })
```

**Implemented (Phase C - SQL functions):**
- Function metadata schema (`_functions`, `_function_args` tables)
- RPC HTTP handler at `/rest/v1/rpc/{name}`
- SQL dialect support with parameter binding
- Return type support (scalar, setof, table)
- Variadic function support
- Security definer functions (execute with elevated privileges)
- Dashboard UI for function management
- Admin API for function CRUD operations
- Full E2E test suite

**Future phases (optional):**
- **Phase D:** PL/pgSQL → TypeScript transpilation (~70-80% compatibility)
- **Phase A:** Full PL/pgSQL interpreter (~95% compatibility)

See `docs/rpc-functions.md` for documentation.

---

### ~~Realtime Subscriptions~~ ✅ COMPLETE
WebSocket-based real-time updates when data changes. Core Supabase feature for reactive apps.

```javascript
// Client usage
supabase
  .channel('posts')
  .on('postgres_changes', { event: '*', schema: 'public', table: 'posts' }, callback)
  .subscribe()
```

**Implemented:**
- Phoenix Protocol v1.0.0 WebSocket endpoint (`/realtime/v1/websocket`)
- Broadcast (ephemeral client-to-client messages)
- Presence (user online state tracking)
- Postgres Changes (INSERT/UPDATE/DELETE notifications via REST hooks)
- Filter operators (eq, neq, gt, gte, lt, lte, in)
- RLS enforcement (events filtered by user's row-level security policies)
- Broadcast replay (retrieve recent messages on subscribe)
- Dashboard stats endpoint for monitoring
- Full E2E test suite (42 tests)

See `docs/realtime.md` for documentation.

---

### ~~File Storage~~ ✅ COMPLETE
Storage API for file uploads, downloads, and management. Used for user avatars, attachments, etc.

```javascript
// Client usage
supabase.storage.from('avatars').upload('path/file.png', file)
supabase.storage.from('avatars').getPublicUrl('path/file.png')
supabase.storage.from('avatars').createSignedUrl('path/file.png', 60)
```

**Implemented:**
- Bucket operations (create, get, list, update, delete, empty)
- Object operations (upload, download, delete, list, copy, move)
- Public bucket access
- Row Level Security (RLS) with storage helper functions
- Signed URLs (download and upload)
- Multiple backends (local filesystem, S3-compatible)
- File size limits and MIME type restrictions

See `docs/STORAGE.md` for documentation.

**Remaining (optional enhancements):**
- Image transformations (resize, format conversion, quality) - Complexity: High

**TUS Resumable Uploads:** ✅ COMPLETE
- TUS 1.0.0 protocol implementation
- Endpoints: `/storage/v1/upload/resumable`
- Features: creation, termination, chunked uploads
- 24-hour session expiry with automatic cleanup
- RLS enforcement on session creation

---

### ~~Edge Functions~~ ✅ COMPLETE
Serverless TypeScript/JavaScript functions using Deno runtime. Enables custom backend logic.

```javascript
// Client usage
supabase.functions.invoke('hello-world', { body: { name: 'World' } })
```

**Implemented:**
- Supabase Edge Runtime integration (auto-downloaded binary)
- HTTP proxy for `/functions/v1/*` endpoints
- Process management with health checks
- Encrypted secrets storage with env var injection
- Per-function JWT verification toggle
- Dashboard UI for function management
- CLI commands for function scaffolding

See `docs/edge-functions.md` for documentation.

---

### ~~Full-Text Search~~ ✅ COMPLETE
Text search using SQLite FTS5 extension.

```javascript
// Client usage
supabase.from('posts').select().textSearch('content', 'search query')
```

**Implemented:**
- FTS5 virtual tables with external content (no data duplication)
- `textSearch()` filter with plain, phrase, websearch, and fts query types
- Auto-sync via triggers on INSERT/UPDATE/DELETE
- Relevance ordering via BM25 ranking
- Dashboard UI for index management
- PostgreSQL DDL export with GIN indexes

See `docs/full-text-search.md` for documentation.

---

## Auth API Gaps

### ~~OAuth Providers~~ ✅ COMPLETE
Social login with Google, GitHub.

**Implemented:**
- OAuth endpoints (`/auth/v1/authorize`, `/auth/v1/callback`)
- Google and GitHub providers
- PKCE flow support
- Identity linking/unlinking
- Dashboard UI for provider configuration

See `docs/oauth.md` for documentation.

---

### Phone Authentication
SMS/WhatsApp OTP authentication.

**Implementation:**
- Integrate SMS provider (Twilio, etc.)
- Add phone number to auth_users
- OTP generation and verification
- Requires: external SMS service

**Complexity:** Medium
**Priority:** Low

---

### ~~Anonymous Sign-In~~ ✅ COMPLETE
Allow users to start without credentials, convert later.

```javascript
supabase.auth.signInAnonymously()
```

**Implemented:**
- Create anonymous user with `is_anonymous` flag
- OAuth-based conversion (link Google/GitHub account)
- Password-based conversion (set email/password)
- Automatic identity merging on conversion

See `docs/anonymous-signin.md` for documentation.

---

### Multi-Factor Authentication (MFA)
TOTP-based second factor.

**Implementation:**
- Add MFA tables (`auth_mfa_factors`, `auth_mfa_challenges`)
- TOTP secret generation and verification
- Adjust auth flow for MFA challenge

**Complexity:** Medium
**Priority:** Low

---

## Priority Summary

| Feature | Priority | Effort | Status |
|---------|----------|--------|--------|
| **sblite-hub (multi-tenant control plane)** | Future | Very Large | Planned |
| PostgreSQL functions (RPC) | Medium | Medium-Very Large | ✅ Complete (SQL functions) |
| Vector search | Medium | Medium-Large | ✅ Complete |
| Realtime subscriptions | Medium | Large | ✅ Complete |
| File storage | Medium | Large | ✅ Complete |
| Edge functions | Medium | Large | ✅ Complete |
| Full-text search | Low | Medium | ✅ Complete |
| OAuth providers | Low | Large | ✅ Complete (Google, GitHub) |
| Anonymous sign-in | Low | Small | ✅ Complete |
| Phone auth | Low | Medium | Pending |
| MFA | Low | Medium | Pending |
| Image transformations | Low | High | Pending (storage enhancement) |
| Resumable uploads (TUS) | Low | Medium | ✅ Complete |

## Build/Release

### Version in Binary
The `--version` flag shows "dev" instead of the actual release version.

```bash
./sblite --version
sblite version dev  # Should show v0.2.12
```

**Fix:** Inject version via `-ldflags` during build in the release workflow.

**Complexity:** Small
**Priority:** Low

---

## Notes

- sblite is fully functional for typical use cases (CRUD + auth + RLS + FTS + OAuth + Storage + Realtime + Edge Functions + RPC + Vector Search)
- All major Supabase features are now implemented
- Remaining auth gaps (phone auth, MFA) are for enterprise/advanced use cases
- All core Supabase client SDK features are now supported
