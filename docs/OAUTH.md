# OAuth Authentication

sblite supports OAuth authentication with Google and GitHub providers, compatible with `@supabase/supabase-js` client methods.

## Features

- **PKCE Security** - All OAuth flows use Proof Key for Code Exchange (S256)
- **Auto-linking** - Accounts are automatically linked by matching email address
- **Auto-signup** - New users are created automatically on first OAuth sign-in
- **Multiple providers** - Users can link multiple OAuth providers to one account
- **Dashboard configuration** - Configure providers via the web dashboard

## Quick Start

### 1. Configure OAuth Provider

In the dashboard (`/_`), go to **Settings > OAuth Providers**:

1. Enable Google or GitHub
2. Enter your Client ID and Client Secret
3. Add allowed redirect URLs (e.g., `http://localhost:3000/auth/callback`)

### 2. Use with Supabase Client

```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

// Sign in with OAuth
const { data, error } = await supabase.auth.signInWithOAuth({
  provider: 'google',
  options: {
    redirectTo: 'http://localhost:3000/auth/callback'
  }
})

// Browser redirects to Google, then back to your callback URL with tokens
```

### 3. Handle the Callback

After authentication, the user is redirected to your `redirectTo` URL with tokens in the hash fragment:

```
http://localhost:3000/auth/callback#access_token=...&refresh_token=...&expires_in=3600&token_type=bearer
```

The Supabase client handles this automatically if you're using `@supabase/supabase-js`.

## Provider Setup

### Google

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Navigate to **APIs & Services > Credentials**
4. Click **Create Credentials > OAuth client ID**
5. Select **Web application**
6. Add authorized redirect URI: `http://localhost:8080/auth/v1/callback`
7. Copy the Client ID and Client Secret to sblite dashboard

**Required scopes:** `openid email profile` (requested automatically)

### GitHub

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Click **New OAuth App**
3. Set Authorization callback URL: `http://localhost:8080/auth/v1/callback`
4. Copy the Client ID and Client Secret to sblite dashboard

**Required scopes:** `user:email` (requested automatically)

## API Endpoints

### `GET /auth/v1/authorize`

Initiates OAuth flow by redirecting to the provider.

**Query Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `provider` | Yes | `google` or `github` |
| `redirect_to` | Yes | Your app's callback URL (must be in allowlist) |
| `scopes` | No | Additional OAuth scopes (comma-separated) |

**Example:**
```
GET /auth/v1/authorize?provider=google&redirect_to=http://localhost:3000/callback
```

### `GET /auth/v1/callback`

Handles the OAuth provider callback. This endpoint is called by the provider, not your application.

**Query Parameters (from provider):**
- `code` - Authorization code
- `state` - CSRF protection token

**Behavior:**
1. Validates state parameter
2. Exchanges code for tokens using PKCE
3. Fetches user profile from provider
4. Creates or links user account
5. Redirects to your `redirect_to` URL with tokens

### `GET /auth/v1/user/identities`

Lists OAuth identities linked to the current user.

**Authentication:** Required (Bearer token)

**Response:**
```json
{
  "identities": [
    {
      "id": "uuid",
      "provider": "google",
      "identity_data": {
        "email": "user@gmail.com",
        "name": "John Doe",
        "avatar_url": "https://..."
      },
      "created_at": "2026-01-18T12:00:00Z"
    }
  ]
}
```

### `DELETE /auth/v1/user/identities/{provider}`

Unlinks an OAuth provider from the current user.

**Authentication:** Required (Bearer token)

**Behavior:**
- Removes the OAuth identity for the specified provider
- Fails if this is the user's only authentication method (no password set and only one provider)

**Response:** `204 No Content` on success

### `GET /auth/v1/settings`

Returns authentication settings including enabled OAuth providers.

**Response:**
```json
{
  "external": {
    "google": true,
    "github": false,
    "email": true
  }
}
```

## Dashboard API

### `GET /_/api/settings/oauth`

Get OAuth provider configuration (secrets are masked).

### `PATCH /_/api/settings/oauth`

Update OAuth provider configuration.

**Request:**
```json
{
  "google_enabled": true,
  "google_client_id": "123456.apps.googleusercontent.com",
  "google_client_secret": "GOCSPX-..."
}
```

### `GET /_/api/settings/oauth/redirect-urls`

List allowed redirect URLs.

### `POST /_/api/settings/oauth/redirect-urls`

Add an allowed redirect URL.

**Request:**
```json
{
  "url": "http://localhost:3000/auth/callback"
}
```

### `DELETE /_/api/settings/oauth/redirect-urls`

Remove an allowed redirect URL.

**Request:**
```json
{
  "url": "http://localhost:3000/auth/callback"
}
```

## Account Linking

sblite automatically links OAuth accounts to existing users by email:

1. **Existing user with same email:** OAuth identity is linked to that account
2. **New email:** A new user account is created

Users can have multiple authentication methods:
- Email/password + Google
- Google + GitHub
- Any combination

The `app_metadata.providers` array tracks all linked providers:
```json
{
  "provider": "google",
  "providers": ["email", "google", "github"]
}
```

## Security

### PKCE (Proof Key for Code Exchange)

All OAuth flows use PKCE with S256 challenge method:
1. Server generates random `code_verifier` (43-128 characters)
2. Computes `code_challenge = base64url(sha256(code_verifier))`
3. Stores verifier server-side, sends challenge to provider
4. On callback, uses verifier to exchange code for tokens

This prevents authorization code interception attacks.

### State Parameter

A random state token is generated for each OAuth flow:
- Stored in `auth_flow_state` table with 10-minute expiry
- Validated on callback to prevent CSRF attacks
- Deleted after successful exchange

### Redirect URL Validation

Only URLs in the configured allowlist are accepted as `redirect_to` targets. Configure allowed URLs in the dashboard.

## Database Schema

### `auth_identities`

Stores OAuth provider identities linked to users.

```sql
CREATE TABLE auth_identities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    identity_data TEXT,  -- JSON: email, name, avatar_url
    last_sign_in_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(provider, provider_id)
);
```

### `auth_flow_state`

Temporary storage for OAuth flow state (PKCE verifier, redirect URL).

```sql
CREATE TABLE auth_flow_state (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    code_verifier TEXT NOT NULL,
    redirect_to TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
```

## Supabase Client Compatibility

sblite OAuth is compatible with these `@supabase/supabase-js` methods:

| Method | Status |
|--------|--------|
| `signInWithOAuth()` | Supported |
| `getUser()` with identities | Supported |
| `getUserIdentities()` | Supported |
| `unlinkIdentity()` | Supported |
| `linkIdentity()` | Not yet implemented |

## Troubleshooting

### "Invalid redirect URL"

The `redirect_to` parameter must match an allowed URL configured in the dashboard. Check:
- Exact match including protocol and port
- No trailing slash mismatch
- URL is in the allowlist

### "OAuth state not found or expired"

The state parameter in the callback doesn't match a valid flow:
- Flow expired (10-minute timeout)
- User navigated away and returned later
- State was already used

### "Provider not enabled"

The requested provider isn't configured:
- Check dashboard Settings > OAuth Providers
- Ensure provider is enabled and has valid credentials

### "Email required"

The OAuth provider didn't return an email address:
- User denied email scope permission
- Provider account has no verified email
- Try re-authenticating with proper scope consent
