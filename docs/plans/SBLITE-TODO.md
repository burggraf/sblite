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

### Many-to-Many Relationship Queries
Query through join tables.

```javascript
// Example: users -> user_roles -> roles
supabase.from('users').select('*, roles(*)')
```

**Current status:** One-to-many and many-to-one work. Join table traversal doesn't.

**Implementation:**
- Detect junction tables (two foreign keys, minimal other columns)
- Generate appropriate JOINs
- Flatten results correctly

**Complexity:** Medium
**Priority:** Low

---

### Aliased Joins / Self-Referential Queries
Query same table multiple times with different aliases.

```javascript
// Example: messages with sender and receiver from same users table
supabase.from('messages').select('*, sender:users!sender_id(*), receiver:users!receiver_id(*)')
```

**Implementation:**
- Parse alias syntax in select
- Generate aliased JOINs in SQL
- Map results back to aliases

**Complexity:** Medium
**Priority:** Low

---

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

### Password Recovery Email
Send password reset emails.

```javascript
supabase.auth.resetPasswordForEmail('user@example.com')
```

**Current status:** The mail system exists. Need to wire up the `/auth/v1/recover` endpoint to actually send emails.

**Implementation:**
- Generate recovery token
- Send email via configured mail service
- Handle token verification on `/auth/v1/verify`

**Complexity:** Small (infrastructure exists)
**Priority:** Medium

---

### Scoped Sign-Out
Sign out from specific sessions.

```javascript
supabase.auth.signOut({ scope: 'local' })  // Current session only
supabase.auth.signOut({ scope: 'others' }) // All other sessions
```

**Implementation:**
- Add scope parameter to logout endpoint
- Selectively revoke sessions/tokens

**Complexity:** Small
**Priority:** Low

---

## Priority Summary

| Feature | Priority | Effort | Notes |
|---------|----------|--------|-------|
| Password recovery email | High | Small | Infrastructure exists |
| Realtime subscriptions | Medium | Large | Major feature |
| File storage | Medium | Large | Major feature |
| Full-text search | Low | Medium | SQLite FTS5 |
| Many-to-many queries | Low | Medium | Edge case |
| Aliased joins | Low | Medium | Edge case |
| OAuth providers | Low | Large | Per-provider work |
| Anonymous sign-in | Low | Small | Nice to have |
| Phone auth | Low | Medium | Requires SMS provider |
| MFA | Low | Medium | Enterprise feature |
| Scoped sign-out | Low | Small | Nice to have |
| Quoted identifiers | Low | Small | Edge case |

## CLAUDE.md Correction

The "Planned" section in CLAUDE.md lists `or() / not() / match() filters` as not implemented, but these ARE implemented. Should be removed from planned list.

## Notes

- sblite is fully functional for typical use cases (CRUD + auth + RLS)
- Realtime and Storage are the biggest gaps vs full Supabase
- Most auth gaps are for enterprise/advanced use cases
- REST API gaps are edge cases that rarely come up
