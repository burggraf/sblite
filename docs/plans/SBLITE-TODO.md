# sblite Remaining Work

This document tracks main app features that haven't been implemented yet. Reference: `e2e/COMPATIBILITY.md`, `docs/plans/2026-01-16-supabase-lite-design.md`

## Major Features

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

### File Storage
Storage API for file uploads, downloads, and management. Used for user avatars, attachments, etc.

```javascript
// Client usage
supabase.storage.from('avatars').upload('path/file.png', file)
supabase.storage.from('avatars').getPublicUrl('path/file.png')
```

**Implementation approach:**
- Add `/storage/v1/` endpoints
- Store files on disk (configurable path)
- Store metadata in `_storage_objects` table
- Support buckets, policies, signed URLs

**Complexity:** Large
**Priority:** Medium

---

### Full-Text Search
Text search using SQLite FTS5 extension.

```javascript
// Client usage
supabase.from('posts').select().textSearch('content', 'search query')
```

**Implementation approach:**
- Create FTS5 virtual tables mirroring user tables
- Implement `textSearch()` filter translating to FTS5 MATCH
- Auto-sync FTS index on INSERT/UPDATE/DELETE
- Consider: manual FTS table creation vs automatic

**Complexity:** Medium
**Priority:** Low

---

## REST API Gaps

### Quoted Identifiers
Support column/table names with spaces or reserved words.

```javascript
supabase.from('my table').select('my column')
```

**Implementation:**
- Quote identifiers in SQL generation
- Handle in query parser

**Complexity:** Small
**Priority:** Low

---

## Auth API Gaps

### OAuth Providers
Social login with Google, GitHub, Apple, etc.

**Implementation approach:**
- Add OAuth endpoints (`/auth/v1/authorize`, `/auth/v1/callback`)
- Store provider configs in settings
- Handle OAuth flow, create/link users
- Requires: redirect URLs, client secrets management

**Complexity:** Large (per provider)
**Priority:** Low (email auth covers most use cases)

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

### Anonymous Sign-In
Allow users to start without credentials, convert later.

```javascript
supabase.auth.signInAnonymously()
```

**Implementation:**
- Create user with `is_anonymous` flag
- No email/password required
- Allow linking credentials later

**Complexity:** Small
**Priority:** Low

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

| Feature | Priority | Effort | Notes |
|---------|----------|--------|-------|
| Realtime subscriptions | Medium | Large | Major feature |
| File storage | Medium | Large | Major feature |
| Full-text search | Low | Medium | SQLite FTS5 |
| OAuth providers | Low | Large | Per-provider work |
| Anonymous sign-in | Low | Small | Nice to have |
| Phone auth | Low | Medium | Requires SMS provider |
| MFA | Low | Medium | Enterprise feature |
| Quoted identifiers | Low | Small | Edge case |

## Notes

- sblite is fully functional for typical use cases (CRUD + auth + RLS)
- Realtime and Storage are the biggest gaps vs full Supabase
- Most auth gaps are for enterprise/advanced use cases
- REST API gaps are edge cases that rarely come up
