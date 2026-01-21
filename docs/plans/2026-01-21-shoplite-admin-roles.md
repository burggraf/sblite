# ShopLite Admin Roles Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add admin user functionality with role stored in database, first user auto-promoted to admin, and user info displayed in navbar.

**Architecture:** Create `user_roles` table (admin entries only, no entry = customer). On signup, check if first user and auto-insert admin role. Extend AuthContext to fetch and expose role. Display user email/name + role in Navbar.

**Tech Stack:** React, Supabase JS client, SQLite migrations, RLS policies

---

## Task 1: Create user_roles Migration

**Files:**
- Create: `test_apps/shoplite/migrations/20260121000001_user_roles.sql`

**Step 1: Create the migration file**

```sql
-- User roles table (only admins have entries, no entry = customer)
CREATE TABLE user_roles (
    user_id TEXT PRIMARY KEY,
    role TEXT NOT NULL DEFAULT 'admin',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Register user_roles columns with types
INSERT INTO _columns (table_name, column_name, pg_type) VALUES
    ('user_roles', 'user_id', 'uuid'),
    ('user_roles', 'role', 'text'),
    ('user_roles', 'created_at', 'timestamptz');

-- Enable RLS on user_roles
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('user_roles', 1);

-- SELECT: Users can read their own role, admins can read all
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('user_roles', 'user_roles_select', 'SELECT',
     'user_id = auth.uid() OR EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- INSERT: Users can insert their own role entry
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('user_roles', 'user_roles_insert', 'INSERT', 'user_id = auth.uid()');
```

**Step 2: Verify file created**

Run: `cat test_apps/shoplite/migrations/20260121000001_user_roles.sql`
Expected: File contents match above

**Step 3: Commit**

```bash
cd test_apps/shoplite && git add migrations/20260121000001_user_roles.sql
git commit -m "feat(shoplite): add user_roles table migration"
```

---

## Task 2: Add Role Fetching to AuthContext

**Files:**
- Modify: `test_apps/shoplite/src/App.jsx`

**Step 1: Add role state and fetchUserRole function**

In App.jsx, after line 44 (`const [cartCount, setCartCount] = useState(0)`), add:

```jsx
const [role, setRole] = useState(null)
```

After the `fetchCartCount` function (after line 81), add:

```jsx
async function fetchUserRole(userId) {
  const { data } = await supabase
    .from('user_roles')
    .select('role')
    .eq('user_id', userId)
    .maybeSingle()

  return data?.role || 'customer'
}
```

**Step 2: Fetch role on auth state change**

Modify the auth state change effect (lines 46-61). Replace the effect with:

```jsx
useEffect(() => {
  // Get initial session
  supabase.auth.getSession().then(async ({ data: { session } }) => {
    setUser(session?.user ?? null)
    if (session?.user) {
      const userRole = await fetchUserRole(session.user.id)
      setRole(userRole)
    }
    setLoading(false)
  })

  // Listen for auth changes
  const { data: { subscription } } = supabase.auth.onAuthStateChange(
    async (_event, session) => {
      setUser(session?.user ?? null)
      if (session?.user) {
        const userRole = await fetchUserRole(session.user.id)
        setRole(userRole)
      } else {
        setRole(null)
      }
    }
  )

  return () => subscription.unsubscribe()
}, [])
```

**Step 3: Add role to authValue**

Modify authValue (around line 83) to include role:

```jsx
const authValue = {
  user,
  role,
  loading,
  // ... rest of methods unchanged
}
```

**Step 4: Verify changes compile**

Run: `cd test_apps/shoplite && npm run build`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add src/App.jsx
git commit -m "feat(shoplite): add role fetching to AuthContext"
```

---

## Task 3: Auto-Promote First User to Admin

**Files:**
- Modify: `test_apps/shoplite/src/App.jsx`

**Step 1: Modify signUp to check if first user**

Replace the signUp function in authValue with:

```jsx
signUp: async (email, password) => {
  const { data, error } = await supabase.auth.signUp({
    email,
    password
  })

  if (!error && data.user) {
    // Check if this is the first user
    const { count } = await supabase
      .from('auth_users')
      .select('*', { count: 'exact', head: true })

    if (count === 1) {
      // First user becomes admin
      await supabase
        .from('user_roles')
        .insert({ user_id: data.user.id, role: 'admin' })
      setRole('admin')
    }
  }

  return { data, error }
},
```

**Step 2: Verify changes compile**

Run: `cd test_apps/shoplite && npm run build`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add src/App.jsx
git commit -m "feat(shoplite): auto-promote first user to admin on signup"
```

---

## Task 4: Display User Info in Navbar

**Files:**
- Modify: `test_apps/shoplite/src/components/Navbar.jsx`

**Step 1: Update useAuth destructuring**

Replace line 5:
```jsx
const { user, signOut } = useAuth()
```

With:
```jsx
const { user, role, signOut } = useAuth()
```

**Step 2: Add user info display**

After line 32 (`<Link to="/orders">Orders</Link>`) and before line 33 (the sign out button), add:

```jsx
<span className="user-info">
  {user.user_metadata?.name || user.email}
  <span className="user-role">{role === 'admin' ? 'Admin' : 'Customer'}</span>
</span>
```

**Step 3: Verify changes compile**

Run: `cd test_apps/shoplite && npm run build`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add src/components/Navbar.jsx
git commit -m "feat(shoplite): display user info and role in navbar"
```

---

## Task 5: Add User Info CSS Styles

**Files:**
- Modify: `test_apps/shoplite/src/index.css`

**Step 1: Add user-info styles**

After the `.cart-badge` styles (after line 96), add:

```css
.user-info {
  font-size: 0.75rem;
  color: var(--text-light);
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  line-height: 1.3;
}

.user-role {
  font-size: 0.625rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--secondary);
}
```

**Step 2: Verify changes compile**

Run: `cd test_apps/shoplite && npm run build`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add src/index.css
git commit -m "style(shoplite): add user info display styles"
```

---

## Task 6: Manual Integration Test

**Step 1: Reset and rebuild test database**

```bash
cd test_apps/shoplite
rm -f shoplite.db
cd ../..
go build -o sblite .
./sblite init --db test_apps/shoplite/shoplite.db
./sblite db push --db test_apps/shoplite/shoplite.db --migrations-dir test_apps/shoplite/migrations
```

**Step 2: Seed products**

```bash
sqlite3 test_apps/shoplite/shoplite.db < test_apps/shoplite/seed.sql
```

**Step 3: Start the server**

```bash
./sblite serve --db test_apps/shoplite/shoplite.db
```

**Step 4: Start the frontend (in another terminal)**

```bash
cd test_apps/shoplite && npm run dev
```

**Step 5: Manual verification checklist**

1. Open http://localhost:3000
2. Click "Sign Up" and register a new user
3. Verify navbar shows: `{email} Admin` (small text)
4. Sign out
5. Register a second user
6. Verify navbar shows: `{email} Customer` (small text)

**Step 6: Final commit if all tests pass**

```bash
git add -A
git commit -m "feat(shoplite): complete admin roles implementation"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create user_roles migration | migrations/20260121000001_user_roles.sql |
| 2 | Add role fetching to AuthContext | src/App.jsx |
| 3 | Auto-promote first user to admin | src/App.jsx |
| 4 | Display user info in Navbar | src/components/Navbar.jsx |
| 5 | Add user info CSS styles | src/index.css |
| 6 | Manual integration test | - |
