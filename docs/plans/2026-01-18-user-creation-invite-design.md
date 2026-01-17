# User Creation and Invite Features Design

## Overview

Add two Supabase-style user management features to the dashboard:
1. **Create User** - Admin directly creates a user with email/password
2. **Invite User** - Admin sends invitation email, user sets own password

## UI Changes

### Users View Toolbar
```
Users                                    [+ Create User] [✉ Invite User]  42 users
```

### Create User Modal
- Email field (required, validated)
- Password field (required, min 6 chars)
- "Auto-confirm email" checkbox (default: checked)
- Create / Cancel buttons

### Invite User Modal
- Email field (required, validated)
- Send Invite / Cancel buttons
- On success: Shows invite link with "Copy Link" button

## API Endpoints

### POST /_/api/users
Creates a new user directly.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepass123",
  "auto_confirm": true
}
```

**Response:**
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "created_at": "2026-01-18T...",
  "email_confirmed_at": "2026-01-18T..."
}
```

### POST /_/api/users/invite
Sends an invitation to a new user.

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response:**
```json
{
  "success": true,
  "invite_link": "http://localhost:8080/auth/v1/verify?token=xxx&type=invite"
}
```

## Backend Changes

### Dashboard Handler (internal/dashboard/handler.go)
- Add `handleCreateUser` - creates user via auth service
- Add `handleInviteUser` - creates invite token and sends email

### Auth Service (internal/auth/)
- Add `CreateInviteToken(email string)` method:
  - Creates user record with no password
  - Generates invite token in auth_tokens table (type='invite', expires 7 days)
  - Returns token string

## Token Flow

### Invite Flow
1. Admin clicks "Send Invite" for email
2. Backend creates user (no password, unconfirmed)
3. Backend creates invite token in auth_tokens
4. Backend calls mail.SendInvite()
5. Backend returns invite_link to frontend
6. Frontend displays link in success modal

### Token Redemption
1. User clicks invite link
2. GET /auth/v1/verify?token=xxx&type=invite (existing endpoint)
3. User redirected to set-password page
4. User sets password, email_confirmed_at set
5. User can now log in

## Error Handling

### Create User
- Email exists → "A user with this email already exists"
- Invalid email → "Please enter a valid email address"
- Short password → "Password must be at least 6 characters"

### Invite User
- Email exists (confirmed) → "A user with this email already exists"
- Invalid email → "Please enter a valid email address"
- Pending invite → Generate new token and resend

## No Changes Required
- Email templates (invite template exists)
- Verify endpoint (handles invite type)
- User list/edit/delete functionality
