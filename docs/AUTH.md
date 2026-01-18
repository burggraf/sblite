# Authentication

sblite provides Supabase-compatible authentication with email/password, OAuth providers, magic links, and user invitations.

## Email Confirmation

By default, sblite requires email confirmation for new user signups, matching Supabase's default behavior. This can be configured via the dashboard or API.

### How It Works

**When email confirmation is required (default):**
1. User signs up with email/password
2. A confirmation email is sent with a verification link
3. User cannot log in until they click the verification link
4. After verification, normal login works

**When email confirmation is disabled:**
1. User signs up with email/password
2. User receives a session immediately (can use the app right away)
3. No confirmation email is sent

### Configuring Email Confirmation

#### Via Dashboard

1. Navigate to the dashboard at `http://localhost:8080/_`
2. Go to **Settings**
3. Expand the **Authentication** section
4. Toggle **"Require email confirmation for new signups"**

#### Via API

**Get current setting:**
```bash
curl http://localhost:8080/_/api/settings/auth-config
```

Response:
```json
{
  "require_email_confirmation": true
}
```

**Update setting:**
```bash
curl -X PATCH http://localhost:8080/_/api/settings/auth-config \
  -H "Content-Type: application/json" \
  -d '{"require_email_confirmation": false}'
```

### Public Settings Endpoint

The `/auth/v1/settings` endpoint returns `mailer_autoconfirm` which indicates whether email confirmation is required:

```bash
curl http://localhost:8080/auth/v1/settings
```

Response:
```json
{
  "external": {
    "email": true,
    "google": false,
    "github": false
  },
  "disable_signup": false,
  "mailer_autoconfirm": false
}
```

- `mailer_autoconfirm: false` = email confirmation is required
- `mailer_autoconfirm: true` = email confirmation is not required

### Signup Response

When email confirmation is **required**, signup returns:
```json
{
  "id": "user-uuid",
  "email": "user@example.com",
  "confirmation_sent_at": "2024-01-18T12:00:00Z",
  "email_confirmation_required": true
}
```

When email confirmation is **not required**, signup returns a full session:
```json
{
  "access_token": "eyJ...",
  "token_type": "bearer",
  "expires_in": 3600,
  "refresh_token": "...",
  "user": {
    "id": "user-uuid",
    "email": "user@example.com"
  }
}
```

### Login Error for Unconfirmed Email

If a user tries to log in before confirming their email:

```json
{
  "error": "email_not_confirmed",
  "message": "Email not confirmed. Please check your email for a confirmation link."
}
```

HTTP Status: `403 Forbidden`

## Email Verification

Users can verify their email by clicking the link in the confirmation email, which calls:

```
GET /auth/v1/verify?token=<token>&type=signup
```

Or via POST:
```bash
curl -X POST http://localhost:8080/auth/v1/verify \
  -H "Content-Type: application/json" \
  -d '{"type": "signup", "token": "<token>"}'
```

### Resending Confirmation Email

To resend a confirmation email:

```bash
curl -X POST http://localhost:8080/auth/v1/resend \
  -H "Content-Type: application/json" \
  -d '{"type": "confirmation", "email": "user@example.com"}'
```

## OAuth Authentication

sblite supports OAuth authentication with Google and GitHub. See [OAuth documentation](OAUTH.md) for configuration details.

## Magic Links

Users can sign in without a password using magic links:

```bash
curl -X POST http://localhost:8080/auth/v1/magiclink \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com"}'
```

The user receives an email with a link that logs them in directly.

## Password Recovery

To initiate password recovery:

```bash
curl -X POST http://localhost:8080/auth/v1/recover \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com"}'
```

The user receives an email with a link to reset their password.

## User Invitations

Admins can invite users (requires service_role key):

```bash
curl -X POST http://localhost:8080/auth/v1/invite \
  -H "Authorization: Bearer <service_role_key>" \
  -H "Content-Type: application/json" \
  -d '{"email": "newuser@example.com"}'
```

## Session Management

### Logout Scopes

The logout endpoint supports different scopes:

- `local` (default): Revokes only the current session
- `global`: Revokes all sessions for the user
- `others`: Revokes all sessions except the current one

```bash
curl -X POST http://localhost:8080/auth/v1/logout \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"scope": "global"}'
```

## Email Configuration

sblite supports multiple email modes for development and production:

| Mode | Description | Use Case |
|------|-------------|----------|
| `log` | Logs emails to console | Quick development |
| `catch` | Captures emails, web viewer at `/mail` | Testing email flows |
| `smtp` | Sends real emails via SMTP | Production |

Start the server with different modes:
```bash
./sblite serve --mail-mode=log      # Development
./sblite serve --mail-mode=catch    # Testing (view at /mail)
./sblite serve --mail-mode=smtp     # Production
```

For SMTP configuration, see [Email documentation](EMAIL.md).
