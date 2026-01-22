# Email System

Supabase Lite includes a flexible email system for authentication flows. It supports three modes to fit different stages of development and deployment.

## Email Modes

### Log Mode (Default)

Prints emails to stdout. Perfect for quick local development when you just need to see what emails would be sent.

```bash
./sblite serve --mail-mode=log
```

Output example:
```
========================================
EMAIL SENT
To: user@example.com
From: noreply@localhost
Subject: Confirm your email
Type: confirmation
Time: 2024-01-17T10:30:00Z
----------------------------------------
<h2>Confirm your email</h2>
<p>Click the link below to confirm your email address:</p>
<p><a href="http://localhost:8080/auth/v1/verify?token=abc123&type=signup">Confirm Email</a></p>
========================================
```

### Catch Mode

Stores emails in the database and provides a web UI to view them. Ideal for development and testing when you need to interact with email links.

```bash
./sblite serve --mail-mode=catch
```

The mail viewer UI is available at `http://localhost:8080/mail` and shows:
- List of all caught emails with type, recipient, subject, and time
- Filter by email type (confirmation, recovery, magic link, etc.)
- Click to view full email content (HTML and plain text)
- Copy verification links directly
- Delete individual emails or clear all

### SMTP Mode

Sends real emails via an SMTP server. Use for staging and production environments.

```bash
./sblite serve --mail-mode=smtp
```

Requires SMTP configuration (see below).

## Configuration

### CLI Flags

| Flag | Description |
|------|-------------|
| `--mail-mode` | Email mode: `log`, `catch`, or `smtp` (default: `log`) |
| `--mail-from` | Default sender email address |
| `--site-url` | Base URL for email links (e.g., `https://myapp.com`) |

### Environment Variables

Environment variables can be used instead of (or in addition to) CLI flags. CLI flags take precedence.

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_MAIL_MODE` | `log` | Email mode |
| `SBLITE_MAIL_FROM` | `noreply@localhost` | Default sender address |
| `SBLITE_SITE_URL` | `http://localhost:8080` | Base URL for email links |
| `SBLITE_SMTP_HOST` | - | SMTP server hostname |
| `SBLITE_SMTP_PORT` | `587` | SMTP server port |
| `SBLITE_SMTP_USER` | - | SMTP authentication username |
| `SBLITE_SMTP_PASS` | - | SMTP authentication password |

### SMTP Configuration Examples

**Gmail (with App Password):**
```bash
export SBLITE_MAIL_MODE=smtp
export SBLITE_MAIL_FROM=yourapp@gmail.com
export SBLITE_SMTP_HOST=smtp.gmail.com
export SBLITE_SMTP_PORT=587
export SBLITE_SMTP_USER=yourapp@gmail.com
export SBLITE_SMTP_PASS=your-app-password
```

**Amazon SES:**
```bash
export SBLITE_MAIL_MODE=smtp
export SBLITE_MAIL_FROM=noreply@yourdomain.com
export SBLITE_SMTP_HOST=email-smtp.us-east-1.amazonaws.com
export SBLITE_SMTP_PORT=587
export SBLITE_SMTP_USER=your-ses-smtp-user
export SBLITE_SMTP_PASS=your-ses-smtp-password
```

**Mailgun:**
```bash
export SBLITE_MAIL_MODE=smtp
export SBLITE_MAIL_FROM=noreply@yourdomain.com
export SBLITE_SMTP_HOST=smtp.mailgun.org
export SBLITE_SMTP_PORT=587
export SBLITE_SMTP_USER=postmaster@yourdomain.com
export SBLITE_SMTP_PASS=your-mailgun-smtp-password
```

**Local Mailpit/Mailhog:**
```bash
export SBLITE_MAIL_MODE=smtp
export SBLITE_SMTP_HOST=localhost
export SBLITE_SMTP_PORT=1025
# No auth needed for local mail catchers
```

## Dashboard Configuration

Email settings can also be configured through the web dashboard at `/_` under Settings → Email.

**Available settings:**
- Email Mode (Log, Catch, SMTP)
- From Address
- SMTP Host, Port, Username, Password (when SMTP mode selected)

Changes made through the dashboard take effect immediately without server restart (hot-reload). Dashboard settings take priority over CLI flags and environment variables.

## Email Types

The system supports five email types for different authentication flows:

| Type | Trigger | Purpose |
|------|---------|---------|
| `confirmation` | User signup | Verify email address after registration |
| `recovery` | Password reset request | Reset forgotten password |
| `magic_link` | Magic link request | Passwordless sign-in |
| `email_change` | Email update | Verify new email address |
| `invite` | Admin invitation | Invite new user to create account |

## Authentication Flows

### Email Confirmation

When a user signs up, a confirmation email is sent automatically (if email is not auto-confirmed).

```javascript
// Sign up triggers confirmation email
const { data, error } = await supabase.auth.signUp({
  email: 'user@example.com',
  password: 'password123'
})

// User clicks link in email, which calls:
// GET /auth/v1/verify?token=<token>&type=signup
```

### Password Recovery

```javascript
// Request recovery email
const { error } = await supabase.auth.resetPasswordForEmail('user@example.com')

// User clicks link in email and submits new password:
// POST /auth/v1/verify
// { "type": "recovery", "token": "<token>", "password": "newpassword" }
```

### Magic Link

Passwordless authentication via email link.

```javascript
// Request magic link
const { error } = await supabase.auth.signInWithOtp({
  email: 'user@example.com'
})

// User clicks link in email, which creates a session:
// GET /auth/v1/verify?token=<token>&type=magiclink
```

### Resend Emails

Resend confirmation or recovery emails if the user didn't receive them.

```bash
# Resend confirmation
curl -X POST http://localhost:8080/auth/v1/resend \
  -H "Content-Type: application/json" \
  -d '{"type": "confirmation", "email": "user@example.com"}'

# Resend recovery
curl -X POST http://localhost:8080/auth/v1/resend \
  -H "Content-Type: application/json" \
  -d '{"type": "recovery", "email": "user@example.com"}'
```

### User Invitation (Admin Only)

Invite new users. Requires `service_role` authentication.

```bash
curl -X POST http://localhost:8080/auth/v1/invite \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <service_role_token>" \
  -H "apikey: <service_role_key>" \
  -d '{"email": "newuser@example.com"}'
```

The invited user receives an email with a link to set their password and complete registration.

## Email Templates

Email templates are stored in the database and can be customized. Default templates are created during database initialization.

### Template Variables

All templates have access to these variables:

| Variable | Description |
|----------|-------------|
| `{{.ConfirmationURL}}` | Full URL with verification token |
| `{{.Email}}` | Recipient email address |
| `{{.ExpiresIn}}` | Human-readable expiration time |
| `{{.SiteURL}}` | Base site URL |

### Customizing Templates

Templates can be updated via SQL:

```sql
UPDATE auth_email_templates
SET subject = 'Welcome to MyApp - Confirm your email',
    body_html = '<h1>Welcome!</h1><p>Click <a href="{{.ConfirmationURL}}">here</a> to confirm.</p>',
    body_text = 'Welcome! Click here to confirm: {{.ConfirmationURL}}',
    updated_at = datetime('now')
WHERE type = 'confirmation';
```

### Template Types

| Type | Default Subject |
|------|-----------------|
| `confirmation` | Confirm your email |
| `recovery` | Reset your password |
| `magic_link` | Your login link |
| `email_change` | Confirm email change |
| `invite` | You have been invited |

## Mail Viewer API

When running in catch mode, the following API endpoints are available:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/mail/` | GET | Mail viewer web UI |
| `/mail/api/emails` | GET | List all caught emails |
| `/mail/api/emails/:id` | GET | Get single email by ID |
| `/mail/api/emails/:id` | DELETE | Delete single email |
| `/mail/api/emails` | DELETE | Clear all emails |

### Query Parameters

The list endpoint supports pagination:

```bash
# Get first 10 emails
curl http://localhost:8080/mail/api/emails?limit=10

# Get next page
curl http://localhost:8080/mail/api/emails?limit=10&offset=10
```

## Development Workflow

### Recommended Setup

1. **Local Development**: Use `catch` mode to capture and inspect emails
   ```bash
   ./sblite serve --mail-mode=catch --site-url=http://localhost:8080
   ```

2. **Automated Testing**: Use `log` mode or `catch` mode
   ```bash
   ./sblite serve --mail-mode=log  # Just see output
   ./sblite serve --mail-mode=catch  # Query via API
   ```

3. **Staging**: Use `smtp` mode with a test SMTP service (Mailpit, Mailtrap)
   ```bash
   ./sblite serve --mail-mode=smtp --site-url=https://staging.myapp.com
   ```

4. **Production**: Use `smtp` mode with your production email provider
   ```bash
   ./sblite serve --mail-mode=smtp --site-url=https://myapp.com
   ```

### Testing Email Flows

With catch mode, you can programmatically verify email flows in tests:

```javascript
// After triggering an email...
const response = await fetch('http://localhost:8080/mail/api/emails');
const emails = await response.json();

// Find the confirmation email
const confirmEmail = emails.find(e => e.type === 'confirmation');

// Extract token from the confirmation URL in the email body
const tokenMatch = confirmEmail.body_html.match(/token=([^&"]+)/);
const token = tokenMatch[1];

// Complete verification
await fetch(`http://localhost:8080/auth/v1/verify?token=${token}&type=signup`);
```

## E2E Testing

The email system has comprehensive E2E tests located in `e2e/tests/email/`.

### Running Email Tests

**Catch Mode (Automated - Recommended)**

Catch mode stores emails in the database for programmatic verification:

```bash
# Start server in catch mode
./sblite serve --mail-mode=catch --db test.db

# Run email tests
cd e2e
npm run test:email
```

This runs all email tests (~37 tests):
- `mail-api.test.ts` - Mail viewer API endpoints
- `email-flows.test.ts` - Auth flows that trigger emails
- `verification.test.ts` - Complete verification flows

### SMTP Mode (Manual/Local Testing)

SMTP tests require a local mail server like Mailpit:

```bash
# 1. Start Mailpit
docker run -d --name mailpit -p 8025:8025 -p 1025:1025 axllent/mailpit

# 2. Start sblite with SMTP
export SBLITE_MAIL_MODE=smtp
export SBLITE_SMTP_HOST=localhost
export SBLITE_SMTP_PORT=1025
./sblite serve --db test.db

# 3. Run SMTP tests
cd e2e
SBLITE_TEST_SMTP=true npm run test:email:smtp

# 4. View emails in browser
open http://localhost:8025
```

SMTP tests are skipped by default and only run when `SBLITE_TEST_SMTP=true`.

### Test Helpers

The `e2e/setup/mail-helpers.ts` module provides utilities for email testing:

```typescript
import {
  getEmails,          // List all caught emails
  getEmail,           // Get single email by ID
  clearAllEmails,     // Clear all emails
  waitForEmail,       // Wait for email with timeout
  findEmail,          // Find email with polling
  extractToken,       // Extract verification token
  extractVerificationUrl,  // Extract full URL
  assertNoEmailSent,  // Security: verify no email sent
} from '../setup/mail-helpers'

// Example: Wait for recovery email and extract token
const email = await waitForEmail('user@example.com', 'recovery')
const token = extractToken(email)
```

### Environment Variables for Testing

| Variable | Description |
|----------|-------------|
| `SBLITE_TEST_SMTP` | Set to `true` to enable SMTP tests |
| `MAILPIT_API` | Mailpit API URL (default: `http://localhost:8025/api`) |

## Troubleshooting

### Emails not being sent (SMTP mode)

1. Verify SMTP credentials are correct
2. Check if your SMTP provider requires specific ports (try 587 or 465)
3. Ensure your sender address is verified with your email provider
4. Check server logs for SMTP connection errors

### Mail viewer not accessible

The `/mail` endpoint is only available in `catch` mode. Verify you're running with:
```bash
./sblite serve --mail-mode=catch
```

### Token expired errors

Tokens have expiration times:
- Confirmation: 24 hours
- Recovery: 24 hours
- Magic link: 1 hour
- Invite: 7 days

Use the resend endpoint to generate a new token if expired.

### Links point to wrong URL

Set the correct site URL:
```bash
./sblite serve --site-url=https://myapp.com
# or
export SBLITE_SITE_URL=https://myapp.com
```
