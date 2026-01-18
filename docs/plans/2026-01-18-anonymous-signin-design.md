# Anonymous Sign-In Design

## Overview

Implement Supabase-compatible anonymous sign-in for sblite, allowing users to authenticate without providing credentials and later convert to permanent accounts.

## Database Changes

### Schema Migration

Add `is_anonymous` column to `auth_users` table:

```sql
ALTER TABLE auth_users ADD COLUMN is_anonymous INTEGER DEFAULT 0;
CREATE INDEX idx_auth_users_is_anonymous ON auth_users(is_anonymous);
```

### User Struct

```go
type User struct {
    // ... existing fields
    IsAnonymous bool `json:"is_anonymous"`
}
```

## API Design

### Anonymous Sign-In

**Endpoint**: `POST /auth/v1/signup`

**Request** (empty body or metadata only):
```json
{}
// or with optional metadata:
{"data": {"theme": "dark"}}
```

**Detection**: If request has no `email` AND no `password` fields, treat as anonymous.

**Response**:
```json
{
  "access_token": "eyJ...",
  "token_type": "bearer",
  "expires_in": 3600,
  "refresh_token": "abc123",
  "user": {
    "id": "uuid-here",
    "email": null,
    "role": "authenticated",
    "is_anonymous": true,
    "app_metadata": {"provider": "anonymous", "providers": ["anonymous"]},
    "user_metadata": {},
    "created_at": "...",
    "updated_at": "..."
  }
}
```

### JWT Claims

```json
{
  "sub": "user-id",
  "role": "authenticated",
  "is_anonymous": true,
  "session_id": "...",
  "app_metadata": {"provider": "anonymous", "providers": ["anonymous"]},
  "user_metadata": {}
}
```

### Settings Endpoint

`GET /auth/v1/settings` includes:
```json
{
  "external": {
    "anonymous": true,
    ...
  }
}
```

## Converting Anonymous to Permanent

### Method 1: Email/Password

**Endpoint**: `PUT /auth/v1/user`

**Request**:
```json
{"email": "user@example.com", "password": "secret123"}
```

**Behavior**:
1. Validate email is not already taken
2. Hash password and store
3. Set `is_anonymous = 0`
4. Update `app_metadata.provider` to `"email"`, add `"email"` to providers
5. Trigger email confirmation if `require_email_confirmation` is enabled

### Method 2: OAuth Linking

Use existing `/auth/v1/authorize` flow. When anonymous user completes OAuth:
1. Link identity as normal
2. Set `is_anonymous = 0`
3. Update `app_metadata` with new provider

## Implementation Files

| File | Changes |
|------|---------|
| `internal/db/migrations.go` | Add `is_anonymous` column |
| `internal/auth/user.go` | User struct, CreateAnonymousUser, update GetUserByID |
| `internal/auth/jwt.go` | Add `is_anonymous` claim to JWT |
| `internal/server/auth_handlers.go` | Modify handleSignup, handleUpdateUser |
| `internal/server/oauth_handlers.go` | Convert anonymous on OAuth callback |

## E2E Tests

### `e2e/tests/auth/anonymous-signin.test.ts`
- Sign in anonymously
- Verify JWT contains `is_anonymous: true`
- Verify user object has `is_anonymous: true`
- Session refresh works
- Logout works

### `e2e/tests/auth/anonymous-convert.test.ts`
- Convert via email/password
- Verify `is_anonymous` becomes false
- Verify email confirmation flow (if enabled)
- Convert via OAuth linking
- Verify cannot reuse email that exists

## RLS Policy Support

Anonymous users can be distinguished in RLS policies:

```sql
-- Block anonymous users from inserting
CREATE POLICY "no_anonymous_insert" ON my_table
FOR INSERT TO authenticated
WITH CHECK ((auth.jwt()->>'is_anonymous')::boolean IS FALSE);
```

## Security Considerations

- Anonymous users get full `authenticated` role (same as regular users)
- Rate limiting applies to anonymous signup (same as regular signup)
- Anonymous sessions cannot be recovered after logout (no credentials)
- Email must be unique when converting to permanent
