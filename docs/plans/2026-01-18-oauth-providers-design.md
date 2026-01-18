# OAuth Providers Design

**Date:** 2026-01-18
**Status:** Approved
**Author:** Claude + Mark

## Overview

Add OAuth authentication support to sblite, compatible with Supabase's `@supabase/supabase-js` client. Initial implementation supports Google and GitHub providers with PKCE security.

## Decisions

| Aspect | Decision |
|--------|----------|
| Providers | Google + GitHub (extensible) |
| Account linking | Auto-link by matching email |
| Configuration | Dashboard UI, stored in database |
| Security | PKCE for all flows |
| Redirects | Supabase-compatible `redirect_to` param |
| Storage | Separate `auth_identities` table |
| Profile data | Email, name, avatar_url |
| New accounts | Auto-create on first OAuth sign-in |

## API Endpoints

### `GET /auth/v1/authorize` - Initiate OAuth flow

**Query parameters:**
- `provider` (required): `google` or `github`
- `redirect_to` (required): Client callback URL (must be in allowlist)
- `scopes` (optional): Additional OAuth scopes

**Behavior:**
1. Generate PKCE `code_verifier` (43-128 char random string)
2. Compute `code_challenge` = base64url(sha256(code_verifier))
3. Generate `state` (random string)
4. Store state + verifier in `auth_flow_state` table (10-minute expiry)
5. Redirect to provider's authorization URL

### `GET /auth/v1/callback` - OAuth provider callback

**Query parameters (from provider):**
- `code`: Authorization code
- `state`: State parameter for CSRF protection

**Behavior:**
1. Look up `state` in `auth_flow_state`, retrieve `code_verifier` and `redirect_to`
2. Exchange `code` for tokens using PKCE verifier
3. Fetch user profile from provider API
4. Create or link user (see linking logic below)
5. Create session, generate sblite JWT
6. Redirect to client's `redirect_to` with tokens in hash fragment:
   ```
   https://app.example.com/#access_token=...&refresh_token=...&expires_in=3600&token_type=bearer
   ```

### `GET /auth/v1/user/identities` - List linked identities

**Authentication:** Required (Bearer token)

**Response:**
```json
{
  "identities": [
    {
      "id": "uuid",
      "provider": "google",
      "provider_id": "123456789",
      "identity_data": {
        "email": "user@gmail.com",
        "name": "John Doe",
        "avatar_url": "https://..."
      },
      "last_sign_in_at": "2026-01-18T12:00:00Z",
      "created_at": "2026-01-18T12:00:00Z"
    }
  ]
}
```

### `DELETE /auth/v1/user/identities/{provider}` - Unlink provider

**Authentication:** Required (Bearer token)

**Behavior:**
- Removes OAuth identity for the specified provider
- Fails if this is the user's only authentication method

### `POST /auth/v1/token?grant_type=id_token` - Native app flow

**Request body:**
```json
{
  "provider": "google",
  "id_token": "eyJ..."
}
```

**Behavior:**
- For mobile apps using native Google/Apple SDKs
- Validates provider's ID token directly
- Creates/links user and returns session tokens

## Database Schema

### New table: `auth_identities`

```sql
CREATE TABLE auth_identities (
    id TEXT PRIMARY KEY,           -- UUID
    user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,        -- 'google', 'github'
    provider_id TEXT NOT NULL,     -- Provider's user ID (sub claim)
    identity_data TEXT,            -- JSON: email, name, avatar_url, etc.
    last_sign_in_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(provider, provider_id)  -- One identity per provider account
);
CREATE INDEX idx_identities_user ON auth_identities(user_id);
```

### New table: `auth_flow_state`

```sql
CREATE TABLE auth_flow_state (
    id TEXT PRIMARY KEY,           -- State parameter (random string)
    provider TEXT NOT NULL,
    code_verifier TEXT NOT NULL,   -- PKCE verifier (stored server-side)
    redirect_to TEXT,              -- Client's callback URL
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL       -- 10-minute expiry
);
```

### OAuth config in `_dashboard` table

Keys:
- `oauth_google_client_id`
- `oauth_google_client_secret`
- `oauth_google_enabled`
- `oauth_github_client_id`
- `oauth_github_client_secret`
- `oauth_github_enabled`
- `oauth_redirect_urls` (JSON array)

### Updates to `auth_users.raw_app_meta_data`

```json
{
  "provider": "google",
  "providers": ["google", "github", "email"]
}
```

## User Creation & Linking Logic

```
1. Look up identity by (provider, provider_id)
   ├── FOUND: User exists with this OAuth account
   │   └── Update last_sign_in_at, create session, done
   │
   └── NOT FOUND: First time seeing this OAuth account
       │
       ├── Look up user by email
       │   ├── FOUND: Existing user with same email
       │   │   └── AUTO-LINK: Create identity record, add provider to
       │   │       app_metadata.providers[], create session
       │   │
       │   └── NOT FOUND: Completely new user
       │       └── CREATE: New auth_users record + identity record
       │           - email_confirmed_at = NOW (OAuth emails are verified)
       │           - app_metadata.provider = 'google'
       │           - app_metadata.providers = ['google']
       │           - user_metadata = {name, avatar_url} from provider
```

### Edge cases

- **OAuth email is null:** Reject sign-in with error (user denied email scope)
- **User has password + OAuth:** Both work independently, `providers: ["email", "google"]`
- **Unlinking last provider:** Blocked (must have at least one auth method)

## OAuth Flow (Google Example)

### 1. Client initiates

```javascript
const { data, error } = await supabase.auth.signInWithOAuth({
  provider: 'google',
  options: { redirectTo: 'https://myapp.com/callback' }
})
// Browser redirects to: /auth/v1/authorize?provider=google&redirect_to=...
```

### 2. Server generates PKCE and redirects

```
https://accounts.google.com/o/oauth2/v2/auth?
  client_id={CLIENT_ID}&
  redirect_uri={SBLITE_URL}/auth/v1/callback&
  response_type=code&
  scope=openid email profile&
  state={STATE}&
  code_challenge={CHALLENGE}&
  code_challenge_method=S256
```

### 3. User authenticates, redirected back

```
/auth/v1/callback?code={AUTH_CODE}&state={STATE}
```

### 4. Server exchanges code for tokens

- Look up `state` in `auth_flow_state`, get `code_verifier`
- POST to Google's token endpoint with code + verifier
- Receive `access_token` and `id_token`
- Decode ID token to get user info (sub, email, name, picture)

### 5. Create/link user and redirect to client

- Find or create user, create session, generate sblite JWT
- Redirect to client's `redirect_to` with tokens in fragment

## Dashboard Configuration

### OAuth settings UI

New "Authentication > OAuth Providers" section:

- Google: Client ID, Client Secret, Enable/Disable toggle
- GitHub: Client ID, Client Secret, Enable/Disable toggle
- Allowed Redirect URLs: List with add/remove

### Dashboard API endpoints

- `GET /_/api/settings/oauth` - Get OAuth config (secrets masked)
- `PATCH /_/api/settings/oauth` - Update OAuth config
- `GET /_/api/settings/oauth/redirect-urls` - List allowed redirects
- `POST /_/api/settings/oauth/redirect-urls` - Add redirect URL
- `DELETE /_/api/settings/oauth/redirect-urls/{id}` - Remove redirect URL

### Settings endpoint update

`GET /auth/v1/settings` response:
```json
{
  "external": {
    "google": true,
    "github": true,
    "email": true
  }
}
```

## Implementation Structure

### New package: `internal/oauth/`

```
internal/oauth/
├── oauth.go        # Core types: Provider interface, Config, Identity
├── google.go       # Google provider implementation
├── github.go       # GitHub provider implementation
├── pkce.go         # PKCE code_verifier/challenge generation
└── state.go        # Flow state storage (auth_flow_state table)
```

### Provider interface

```go
type Provider interface {
    Name() string
    AuthURL(state, codeChallenge, redirectURI string) string
    ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*Tokens, error)
    GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
}

type UserInfo struct {
    ProviderID    string
    Email         string
    Name          string
    AvatarURL     string
    EmailVerified bool
}
```

### Files to modify

- `internal/db/migrations.go` - Add auth_identities, auth_flow_state tables
- `internal/server/server.go` - Register new routes
- `internal/server/auth_handlers.go` - Add authorize, callback, identities handlers
- `internal/auth/user.go` - Add identity CRUD methods
- `internal/dashboard/handler.go` - Add OAuth settings endpoints
- `internal/dashboard/static/` - OAuth config UI

### Dependencies to add

- `golang.org/x/oauth2` - OAuth2 client library
- `golang.org/x/oauth2/google` - Google provider config

## Testing Strategy

### Unit tests (`internal/oauth/`)

- PKCE generation (verifier length, challenge computation)
- State storage and expiry
- Provider URL construction
- Token exchange (with mocked HTTP)
- User info parsing

### Integration tests (`internal/server/`)

- Authorize endpoint redirects correctly
- Callback validates state, rejects expired/invalid
- User creation and linking logic
- Identity list and unlink endpoints
- Redirect URL validation (rejects non-allowlisted URLs)

### E2E tests (`e2e/tests/auth/oauth.test.ts`)

- `signInWithOAuth()` returns redirect URL
- Settings endpoint shows enabled providers
- Callback with invalid state returns error
- Callback with invalid code returns error
- `user.identities` returns linked providers
- `unlinkIdentity` removes provider
- `unlinkIdentity` blocks if last auth method

### Mock provider mode

Add `SBLITE_OAUTH_MOCK=true` for automated testing:
- Simulates provider responses without real credentials
- Allows full-flow E2E tests in CI

## supabase-js Compatibility

Works with standard supabase-js methods:
- `supabase.auth.signInWithOAuth({ provider: 'google' })`
- `supabase.auth.getUser()` - returns `user.identities`
- `supabase.auth.unlinkIdentity({ provider: 'google' })`
