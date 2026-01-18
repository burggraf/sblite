# sblite Remaining Work

This document tracks main app features that haven't been implemented yet. Reference: `e2e/COMPATIBILITY.md`, `docs/plans/2026-01-16-supabase-lite-design.md`

## Major Features

### Vector Search (pgvector-compatible)
AI/ML similarity search for embeddings, RAG applications, and recommendations.

```javascript
// Client usage
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: [0.1, 0.2, ...],
  match_count: 10,
  match_threshold: 0.7
});
```

**Implementation approach:**
- Store vectors as JSON arrays in TEXT columns
- New `vector(n)` type in type system for validation/export
- Built-in RPC functions (`vector_search`, `vector_match`)
- Distance metrics: cosine, L2, dot product
- Brute force search initially, optional HNSW index for performance
- Export generates pgvector DDL and data conversion

**Complexity:** Medium-Large
**Priority:** Medium (enables AI/ML use cases)

See `docs/plans/2026-01-18-vector-search-design.md` for full design.

---

### Realtime Subscriptions
WebSocket-based real-time updates when data changes. Core Supabase feature for reactive apps.

```javascript
// Client usage
supabase
  .channel('posts')
  .on('postgres_changes', { event: '*', schema: 'public', table: 'posts' }, callback)
  .subscribe()
```

**Implementation approach:**
- Add WebSocket endpoint (`/realtime/v1/`)
- Track subscriptions per connection
- Emit events on INSERT/UPDATE/DELETE via REST API hooks
- Consider: polling fallback vs true WebSocket

**Complexity:** Large
**Priority:** Medium (many apps don't need realtime)

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
- Resumable uploads via TUS protocol - Complexity: Medium

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
| Vector search | Medium | Medium-Large | Pending |
| Realtime subscriptions | Medium | Large | Pending |
| File storage | Medium | Large | ✅ Complete |
| Edge functions | Medium | Large | Pending |
| Full-text search | Low | Medium | ✅ Complete |
| OAuth providers | Low | Large | ✅ Complete (Google, GitHub) |
| Anonymous sign-in | Low | Small | Pending |
| Phone auth | Low | Medium | Pending |
| MFA | Low | Medium | Pending |
| Image transformations | Low | High | Pending (storage enhancement) |
| Resumable uploads (TUS) | Low | Medium | Pending (storage enhancement) |

## Notes

- sblite is fully functional for typical use cases (CRUD + auth + RLS + FTS + OAuth + Storage)
- Realtime and Edge Functions are the biggest gaps vs full Supabase
- Edge Functions achievable via embedding Supabase's open-source Edge Runtime
- Remaining auth gaps are for enterprise/advanced use cases
