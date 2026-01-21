# ShopLite Admin Roles Design

## Overview

Add admin user functionality to the shoplite app with role-based access control stored in a `user_roles` table. The logged-in user's details and role will display in the app header.

## Requirements

- Admin users can add & edit products, prices, etc. (admin actions to be added later)
- Role stored in database `user_roles` table
- First registered user automatically becomes admin
- User details and role shown in navbar header

## Database Schema

### user_roles table

```sql
CREATE TABLE user_roles (
  user_id TEXT PRIMARY KEY,
  role TEXT NOT NULL DEFAULT 'admin',
  created_at TEXT DEFAULT (datetime('now'))
);
```

**Design decisions:**
- **Admin-only entries**: Table only contains entries for admin users. No entry = customer role.
- **Single role per user**: user_id is PRIMARY KEY (not composite key with role)
- **No seed data**: First user auto-promoted to admin via app logic

### RLS Policies

```sql
-- Enable RLS
UPDATE _rls SET enabled = 1 WHERE table_name = 'user_roles';

-- SELECT: Own row OR requester is admin
INSERT INTO _policies (table_name, name, operation, check_expression)
VALUES ('user_roles', 'Users can view own role or admins view all', 'SELECT',
  'user_id = auth.uid() OR EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- INSERT: Own user_id only
INSERT INTO _policies (table_name, name, operation, check_expression)
VALUES ('user_roles', 'Users can insert own role', 'INSERT',
  'user_id = auth.uid()');
```

## Auto-Admin Logic

On signup, the app will:

1. Complete normal signup flow via `supabase.auth.signUp()`
2. Query to count existing users
3. If this is the first user, insert into `user_roles` with role='admin'

```javascript
// After successful signup
const { count } = await supabase
  .from('auth_users')
  .select('*', { count: 'exact', head: true });

if (count === 1) {
  await supabase
    .from('user_roles')
    .insert({ user_id: user.id, role: 'admin' });
}
```

## Frontend Changes

### AuthContext Updates

Extend AuthContext to include role:

```javascript
// Current
{ user, loading, signIn, signUp, signOut, resendConfirmation }

// New
{ user, role, loading, signIn, signUp, signOut, resendConfirmation }
```

Role fetched on auth state change:

```javascript
const fetchUserRole = async (userId) => {
  const { data } = await supabase
    .from('user_roles')
    .select('role')
    .eq('user_id', userId)
    .maybeSingle();

  return data?.role || 'customer';
};
```

### Navbar Display

Show user info in small muted text near sign-out button:

```
{displayName} • {role}
```

**Display name logic:**
- Use `user.user_metadata.name` if available
- Fall back to `user.email`

**Role display:**
- "Admin" if role is 'admin'
- "Customer" otherwise

**Styling:**
- Small font size (0.75rem)
- Muted color
- Positioned in navbar, left of sign-out button

## Implementation Tasks

1. Create migration for `user_roles` table with RLS policies
2. Update `signUp` in App.jsx to auto-promote first user
3. Add role fetching to AuthContext on auth state change
4. Update Navbar.jsx to display user email/name and role
5. Add CSS styles for user info display

## Future Considerations

- Admin-only routes/pages for product management
- Middleware to check admin role before certain actions
- Dashboard for managing user roles (promote/demote)
