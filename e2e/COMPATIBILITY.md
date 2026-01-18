# Supabase Compatibility Matrix

This document tracks sblite's compatibility with the Supabase JavaScript client API.

## Legend

- ✅ Fully implemented and tested
- 🔸 Partially implemented
- ❌ Not implemented
- 🚫 Not applicable (SQLite limitation)

---

## REST API Operations

### Select (`from().select()`)

| Example | Status | Notes |
|---------|--------|-------|
| Getting your data | ✅ | `.select()` |
| Selecting specific columns | ✅ | `.select('col1, col2')` |
| Column renaming | ✅ | `.select('alias:column')` |
| Query referenced tables | ✅ | Many-to-one via `table(columns)` |
| Query with spaces in names | ✅ | Tables/columns with spaces via quoted identifiers |
| Query through join table | ✅ | Many-to-many via junction table detection |
| Query same table multiple times | ✅ | Aliased joins with `!hint` syntax |
| Query nested foreign tables | ✅ | One-to-many via `table(columns)` |
| Filter through referenced tables | ✅ | `table.column` filter syntax |
| Query with count | ✅ | `count: 'exact' | 'planned' | 'estimated'` |
| Query with head: true | ✅ | HTTP HEAD method for count-only queries |
| Query JSON data | ✅ | `->` and `->>` operators via `json_extract()` |
| Query with inner join | ✅ | `table!inner(columns)` syntax |
| Switching schemas | 🚫 | SQLite doesn't have schemas |

### Insert (`from().insert()`)

| Example | Status | Notes |
|---------|--------|-------|
| Create a record | ✅ | `.insert({...})` |
| Create and return | ✅ | `.insert({...}).select()` |
| Bulk create | ✅ | `.insert([{...}, {...}])` |

### Update (`from().update()`)

| Example | Status | Notes |
|---------|--------|-------|
| Update record | ✅ | `.update({...}).eq()` |
| Update and return | ✅ | `.update({...}).select()` |
| Update JSON data | 🔸 | JSON stored as TEXT |

### Upsert (`from().upsert()`)

| Example | Status | Notes |
|---------|--------|-------|
| Upsert data | ✅ | `.upsert({...})` |
| Bulk upsert | ✅ | `.upsert([{...}])` |
| Upsert with onConflict | ✅ | `onConflict: 'column'` option |
| Upsert ignoreDuplicates | ✅ | `ignoreDuplicates: true` option |

### Delete (`from().delete()`)

| Example | Status | Notes |
|---------|--------|-------|
| Delete single record | ✅ | `.delete().eq()` |
| Delete and return | ✅ | `.delete().select()` |
| Delete multiple | ✅ | `.delete().in()` |

---

## Filters

### Comparison Filters

| Filter | Status | PostgREST Operator |
|--------|--------|-------------------|
| `eq()` | ✅ | `eq` |
| `neq()` | ✅ | `neq` |
| `gt()` | ✅ | `gt` |
| `gte()` | ✅ | `gte` |
| `lt()` | ✅ | `lt` |
| `lte()` | ✅ | `lte` |

### Pattern Filters

| Filter | Status | PostgREST Operator |
|--------|--------|-------------------|
| `like()` | ✅ | `like` |
| `ilike()` | ✅ | `ilike` |

### Special Filters

| Filter | Status | Notes |
|--------|--------|-------|
| `is()` | ✅ | For null/true/false |
| `in()` | ✅ | Match any in array |

### Array/Range Filters

| Filter | Status | Notes |
|--------|--------|-------|
| `contains()` | ❌ | Requires PostgreSQL arrays |
| `containedBy()` | ❌ | Requires PostgreSQL arrays |
| `rangeGt()` | ❌ | Requires PostgreSQL ranges |
| `rangeGte()` | ❌ | Requires PostgreSQL ranges |
| `rangeLt()` | ❌ | Requires PostgreSQL ranges |
| `rangeLte()` | ❌ | Requires PostgreSQL ranges |
| `rangeAdjacent()` | ❌ | Requires PostgreSQL ranges |
| `overlaps()` | ❌ | Requires PostgreSQL arrays/ranges |

### Text Search

| Filter | Status | Notes |
|--------|--------|-------|
| `textSearch()` | ❌ | Could use SQLite FTS5 |

### Logical Filters

| Filter | Status | Notes |
|--------|--------|-------|
| `match()` | ✅ | Matches all key-value pairs |
| `not()` | ✅ | Negation operator |
| `or()` | ✅ | PostgREST OR syntax |
| `filter()` | ✅ | Raw filter syntax |

---

## Modifiers

| Modifier | Status | Notes |
|----------|--------|-------|
| `select()` (after insert/update) | ✅ | Return modified rows |
| `order()` | ✅ | Sort results |
| `limit()` | ✅ | Limit row count |
| `range()` | ✅ | Pagination with Range header |
| `single()` | ✅ | Return single object |
| `maybeSingle()` | ✅ | Return object or null |
| `csv()` | ✅ | CSV response format |
| `explain()` | ✅ | Query execution plan |

---

## Response Headers

| Header | Status | Notes |
|--------|--------|-------|
| `Content-Range` | ✅ | Pagination info |
| `Range` (request) | ✅ | Range header pagination |
| `Prefer: count=exact` | ✅ | Exact row count |
| `Prefer: count=planned` | ✅ | Estimated count (uses exact) |
| `Prefer: count=estimated` | ✅ | Estimated count (uses exact) |

---

## Auth API

### User Registration

| Method | Status | Notes |
|--------|--------|-------|
| Email + password signup | ✅ | |
| Phone signup (SMS) | ❌ | |
| Phone signup (WhatsApp) | ❌ | |
| Signup with metadata | ✅ | |
| Signup with redirect | ❌ | Requires email |

### Authentication

| Method | Status | Notes |
|--------|--------|-------|
| `signInWithPassword` (email) | ✅ | |
| `signInWithPassword` (phone) | ❌ | |
| `signInWithOAuth` | ✅ | Google, GitHub |
| `signInWithOtp` | ❌ | |
| `signInWithIdToken` | ❌ | |
| `signInWithSSO` | ❌ | |
| `signInAnonymously` | ✅ | Full support with conversion to permanent user |

### Sign Out

| Method | Status | Notes |
|--------|--------|-------|
| `signOut()` | ✅ | All sessions |
| `signOut({ scope: 'local' })` | ❌ | |
| `signOut({ scope: 'others' })` | ❌ | |

### Session Management

| Method | Status | Notes |
|--------|--------|-------|
| `getSession()` | ✅ | |
| `refreshSession()` | ✅ | |
| `setSession()` | 🔸 | May work |

### User Management

| Method | Status | Notes |
|--------|--------|-------|
| `getUser()` | ✅ | |
| `getUser(jwt)` | ❌ | |
| `updateUser({ email })` | 🔸 | May require confirmation |
| `updateUser({ phone })` | ❌ | |
| `updateUser({ password })` | ✅ | |
| `updateUser({ data })` | ✅ | User metadata |
| `updateUser({ nonce })` | ❌ | |

### Password Recovery

| Method | Status | Notes |
|--------|--------|-------|
| `resetPasswordForEmail()` | ❌ | Requires email sending |

### Auth Events

| Event | Status | Notes |
|-------|--------|-------|
| `INITIAL_SESSION` | ❌ | |
| `SIGNED_IN` | ✅ | |
| `SIGNED_OUT` | ✅ | |
| `TOKEN_REFRESHED` | 🔸 | |
| `USER_UPDATED` | 🔸 | |
| `PASSWORD_RECOVERY` | ❌ | |

### Other Auth Methods

| Method | Status | Notes |
|--------|--------|-------|
| `getClaims()` | ❌ | |
| `reauthenticate()` | ❌ | |
| `resend()` | ❌ | |
| `verifyOtp()` | ❌ | |
| `exchangeCodeForSession()` | ❌ | |
| `mfa.*` | ❌ | MFA not implemented |
| `admin.*` | ❌ | Admin API not implemented |

### OAuth / Identity Management

| Method | Status | Notes |
|--------|--------|-------|
| `signInWithOAuth` | ✅ | Google, GitHub providers |
| `user.identities` | ✅ | Returns linked OAuth identities |
| `unlinkIdentity` | ✅ | Unlink OAuth provider |
| Auth settings endpoint | ✅ | Returns enabled OAuth providers |

**Supported OAuth Providers:**

| Provider | Status | Notes |
|----------|--------|-------|
| Google | ✅ | PKCE flow supported |
| GitHub | ✅ | PKCE flow supported |
| Apple | ❌ | Not implemented |
| Azure | ❌ | Not implemented |
| Discord | ❌ | Not implemented |
| Facebook | ❌ | Not implemented |
| Twitter | ❌ | Not implemented |
| Others | ❌ | Not implemented |

---

## API Differences

### SQLite vs PostgreSQL

| Feature | PostgreSQL | sblite (SQLite) |
|---------|------------|-----------------|
| Arrays | Native `[]` type | JSON text |
| JSON | `jsonb` type | TEXT (parse in app) |
| Ranges | Native range types | Not supported |
| Full-text search | `tsvector` | Could use FTS5 |
| Schemas | Multiple schemas | Single schema |
| Foreign keys | Full support | Full support |

### Not Applicable Features

These Supabase features are not applicable to sblite:

- Realtime subscriptions (WebSocket)
- Postgres extensions
- Database triggers
- pg_net / pg_cron

### Implemented Features (sblite-specific)

| Feature | Status | Notes |
|---------|--------|-------|
| Row Level Security (RLS) | ✅ | Policy-based query rewriting |
| API Key authentication | ✅ | `anon` and `service_role` keys |
| Email verification flows | ✅ | Magic link, password reset, invite |
| Mail catch mode | ✅ | Development email capture |
| Storage API | ✅ | Buckets, objects, signed URLs, RLS, local/S3 backends |
| Edge Functions | ✅ | Supabase Edge Runtime, secrets, per-function config |
| Full-text search | ✅ | SQLite FTS5 with auto-sync triggers |
| OAuth providers | ✅ | Google, GitHub with PKCE support |
| Anonymous sign-in | ✅ | With credential linking for conversion |

---

## Additional Features

### OpenAPI / Schema Introspection

| Feature | Status | Notes |
|---------|--------|-------|
| OpenAPI spec generation | ✅ | `GET /rest/v1/` returns OpenAPI 3.0 spec |
| Table schema introspection | ✅ | Via OpenAPI paths and schemas |

---

## Future Compatibility Roadmap

### Phase 4 (Completed)

- [x] Full-text search with SQLite FTS5
- [x] JSON path extraction (`->`, `->>`) - Implemented via `json_extract()`
- [x] HEAD method support for count-only queries
- [x] Many-to-many relationship queries
- [x] Aliased joins for self-referential queries

### Phase 5 (Completed)

- [x] Storage API (buckets, objects, signed URLs, RLS, local/S3 backends)
- [x] Edge Functions (Supabase Edge Runtime integration)
- [x] OAuth providers (Google, GitHub with PKCE)
- [x] Anonymous sign-in with credential linking

### Future Consideration

- [ ] Realtime simulation (WebSocket-based)
- [ ] Additional OAuth providers (Apple, Azure, Discord, etc.)
- [ ] Image transformations for storage (resize, crop, format conversion)
- [ ] Resumable uploads via TUS protocol
- [ ] Phone authentication (SMS OTP)
- [ ] Multi-factor authentication (TOTP)

---

## Testing Notes

Tests marked with `.skip()` indicate features that are documented but not yet implemented. These serve as a specification for future development.

Run the test suite to see current compatibility status:

```bash
cd e2e
npm test
```
