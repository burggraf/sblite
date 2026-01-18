# Anonymous Sign-in Settings Design

**Date:** 2026-01-18
**Status:** Approved

## Overview

Add dashboard UI to control anonymous sign-in and improve visibility of anonymous users in the Users list.

## Requirements

- Enable/disable toggle for anonymous sign-in in Auth settings
- Display count of anonymous users
- Filter dropdown in Users list (All / Regular / Anonymous)
- "Anon" badge on anonymous user rows
- Allow deletion of anonymous users with warning

## Auth Settings Integration

### Location

New subsection within existing "Authentication" settings section, after email confirmation toggle:

```
┌─────────────────────────────────────────────────────┐
│ ▼ Authentication                                    │
├─────────────────────────────────────────────────────┤
│ ☑ Require email confirmation for new signups        │
│   When enabled, users must verify their email...    │
│                                                     │
│ ─────────────────────────────────────────────────── │
│                                                     │
│ ☑ Allow anonymous sign-in                           │
│   When enabled, users can sign in without email     │
│   or password. Anonymous users: 24                  │
│                                                     │
│ ─────────────────────────────────────────────────── │
│                                                     │
│ JWT Secret: ****...****                             │
│ ...                                                 │
└─────────────────────────────────────────────────────┘
```

### Backend Changes

Currently anonymous sign-in is hardcoded as always enabled in `internal/server/auth_handlers.go:606`. Changes needed:

1. Add `allow_anonymous` key to `_dashboard` settings store (default: true for backwards compatibility)
2. Modify `handleAnonymousSignup` to check this setting before allowing anonymous signup
3. Add dashboard API endpoint to toggle the setting
4. Update `/auth/v1/settings` endpoint to reflect actual stored state instead of hardcoded `true`

### Dashboard API Endpoints

```
GET  /_/api/settings/auth            Include allow_anonymous and anonymous_user_count
POST /_/api/settings/auth/anonymous  { enabled: true/false }
```

## Users List Integration

### Filter Dropdown

Add filter dropdown to Users list toolbar:

```
┌─────────────────────────────────────────────────────────────┐
│ Users                                     [+ Create User]   │
├─────────────────────────────────────────────────────────────┤
│ [Search users...      ] [All Users ▼]                       │
│                         ├─────────────┤                     │
│                         │ All Users   │                     │
│                         │ Regular     │                     │
│                         │ Anonymous   │                     │
│                         └─────────────┘                     │
├─────────────────────────────────────────────────────────────┤
│ Email              │ Status    │ Created     │ Actions      │
├────────────────────┼───────────┼─────────────┼──────────────┤
│ user@example.com   │ Confirmed │ Jan 15      │ [Edit] [Del] │
│ (anonymous)  [Anon]│ Active    │ Jan 18      │ [Edit] [Del] │
│ other@test.com     │ Pending   │ Jan 17      │ [Edit] [Del] │
└─────────────────────────────────────────────────────────────┘
```

### Anonymous User Row Display

- Email column: Shows "(anonymous)" in muted/italic text
- Badge: Small "Anon" tag displayed next to the text
- All other columns function normally (status, created date, actions)

### Delete Confirmation

When deleting an anonymous user, show specific warning:

```
┌─────────────────────────────────────────┐
│ Delete Anonymous User?               ✕  │
├─────────────────────────────────────────┤
│ This will permanently delete this       │
│ anonymous user and all associated data. │
│                                         │
│ User ID: abc123-def456-...              │
│ Created: Jan 18, 2026                   │
│                                         │
│ This action cannot be undone.           │
│                                         │
│           [Cancel]  [Delete User]       │
└─────────────────────────────────────────┘
```

### Backend Query Changes

Modify users list endpoint to support filtering:

```
GET /_/api/users?filter=all        Default, returns all users
GET /_/api/users?filter=regular    Returns users where is_anonymous = 0
GET /_/api/users?filter=anonymous  Returns users where is_anonymous = 1
```

## Implementation Summary

### Files to Modify

**Backend:**
- `internal/server/auth_handlers.go` - Check allow_anonymous setting in handleAnonymousSignup
- `internal/dashboard/handler.go` - Add anonymous setting endpoints, modify users list query

**Frontend:**
- `internal/dashboard/static/app.js` - Add toggle UI, filter dropdown, anonymous badge

### New Dashboard API Endpoints

```
POST /_/api/settings/auth/anonymous    Toggle anonymous sign-in { enabled: bool }
GET  /_/api/users?filter={filter}      Filter users list
```

## E2E Tests

Add to `e2e/tests/dashboard/` (new file or extend existing):

- Toggle anonymous sign-in setting on/off
- Verify anonymous signup returns error when disabled
- Verify anonymous signup works when enabled
- Filter users list by "anonymous" and verify results
- Filter users list by "regular" and verify results
- Delete anonymous user with confirmation dialog

## Documentation Updates

- Update `CLAUDE.md` with new dashboard endpoints
- Update `e2e/TESTS.md` with new test inventory
