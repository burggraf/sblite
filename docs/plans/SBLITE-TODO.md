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

| Feature | Priority | Effort | Status |
|---------|----------|--------|--------|
| Realtime subscriptions | Medium | Large | Pending |
| File storage | Medium | Large | Pending |
| Full-text search | Low | Medium | ✅ Complete |
| OAuth providers | Low | Large | ✅ Complete (Google, GitHub) |
| Anonymous sign-in | Low | Small | Pending |
| Phone auth | Low | Medium | Pending |
| MFA | Low | Medium | Pending |

## Notes

- sblite is fully functional for typical use cases (CRUD + auth + RLS + FTS + OAuth)
- Realtime and Storage are the biggest gaps vs full Supabase
- Remaining auth gaps are for enterprise/advanced use cases
