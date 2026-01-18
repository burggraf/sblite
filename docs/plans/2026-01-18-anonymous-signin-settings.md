# Anonymous Sign-in Settings Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add dashboard UI to control anonymous sign-in and improve visibility of anonymous users in the Users list.

**Architecture:** Settings stored in `_dashboard` key-value table. Auth handlers check setting before allowing anonymous signup. Users list query supports filtering by `is_anonymous` column.

**Tech Stack:** Go (Chi router, sql), Vanilla JavaScript, existing CSS variables

---

## Task 1: Backend - Store Anonymous Setting

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add anonymous setting endpoints to RegisterRoutes**

Find the auth settings routes and add:

```go
// In RegisterRoutes, add to the settings group (around line 130)
r.Post("/auth/anonymous", h.handleSetAnonymousSetting)
```

**Step 2: Implement the handler**

Add at end of handler.go:

```go
func (h *Handler) handleSetAnonymousSetting(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	value := "false"
	if req.Enabled {
		value = "true"
	}

	if err := h.store.Set("allow_anonymous", value); err != nil {
		http.Error(w, `{"error":"Failed to save setting"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": req.Enabled})
}
```

**Step 3: Update handleGetAuthSettings to include anonymous setting**

Find `handleGetAuthSettings` (or similar) and add anonymous info:

```go
// Add to auth settings response:
allowAnonymous := true // Default
if val, err := h.store.Get("allow_anonymous"); err == nil {
	allowAnonymous = val == "true"
}

// Count anonymous users
var anonCount int
row := h.db.QueryRow("SELECT COUNT(*) FROM auth_users WHERE is_anonymous = 1")
row.Scan(&anonCount)

// Include in response:
response["allow_anonymous"] = allowAnonymous
response["anonymous_user_count"] = anonCount
```

**Step 4: Build and verify**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(dashboard): add anonymous sign-in setting storage"
```

---

## Task 2: Backend - Enforce Anonymous Setting in Auth

**Files:**
- Modify: `internal/server/auth_handlers.go`
- Modify: `internal/server/server.go`

**Step 1: Add dashboard store to server**

In `internal/server/server.go`, add a method to check anonymous setting:

```go
// Add to Server struct if not present
dashboardStore *dashboard.Store

// Add setter
func (s *Server) SetDashboardStore(store *dashboard.Store) {
	s.dashboardStore = store
}

// Add helper method
func (s *Server) isAnonymousSigninEnabled() bool {
	if s.dashboardStore == nil {
		return true // Default enabled if no store
	}
	val, err := s.dashboardStore.Get("allow_anonymous")
	if err != nil {
		return true // Default enabled on error
	}
	return val != "false"
}
```

**Step 2: Check setting in handleAnonymousSignup**

In `internal/server/auth_handlers.go`, find `handleAnonymousSignup` and add check at the start:

```go
func (s *Server) handleAnonymousSignup(w http.ResponseWriter, r *http.Request, userMetadata map[string]any) {
	// Check if anonymous sign-in is enabled
	if !s.isAnonymousSigninEnabled() {
		s.writeError(w, http.StatusForbidden, "anonymous_disabled", "Anonymous sign-in is disabled")
		return
	}

	// ... rest of existing code
}
```

**Step 3: Update settings endpoint to reflect actual state**

In `handleAuthSettings`, update the anonymous line (around line 606):

```go
// Change from hardcoded true to checking setting
"anonymous": s.isAnonymousSigninEnabled(),
```

**Step 4: Wire up dashboard store in server setup**

In `cmd/serve.go` or wherever server is initialized, add:

```go
// After creating dashboard handler
server.SetDashboardStore(dashboardHandler.GetStore())
```

Add getter to dashboard handler:

```go
// In handler.go
func (h *Handler) GetStore() *Store {
	return h.store
}
```

**Step 5: Build and verify**

Run: `go build ./...`
Expected: No errors

**Step 6: Commit**

```bash
git add internal/server/auth_handlers.go internal/server/server.go internal/dashboard/handler.go cmd/serve.go
git commit -m "feat(auth): enforce anonymous sign-in setting"
```

---

## Task 3: Backend - Users List Filter

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Update handleListUsers to support filter**

Find `handleListUsers` and add filter support:

```go
func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	// Existing pagination params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// NEW: Filter parameter
	filter := r.URL.Query().Get("filter") // "all", "regular", "anonymous"

	// Build WHERE clause based on filter
	whereClause := ""
	switch filter {
	case "regular":
		whereClause = "WHERE is_anonymous = 0"
	case "anonymous":
		whereClause = "WHERE is_anonymous = 1"
	// "all" or empty = no filter
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM auth_users %s", whereClause)
	h.db.QueryRow(countQuery).Scan(&total)

	// Query users
	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT id, email, email_confirmed_at, created_at, updated_at, is_anonymous
		FROM auth_users
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	rows, err := h.db.Query(query, pageSize, offset)
	if err != nil {
		http.Error(w, `{"error":"Failed to query users"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []map[string]any
	for rows.Next() {
		var id, createdAt, updatedAt string
		var email, emailConfirmedAt sql.NullString
		var isAnonymous int

		if err := rows.Scan(&id, &email, &emailConfirmedAt, &createdAt, &updatedAt, &isAnonymous); err != nil {
			continue
		}

		user := map[string]any{
			"id":           id,
			"email":        email.String,
			"confirmed":    emailConfirmedAt.Valid,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
			"is_anonymous": isAnonymous == 1,
		}
		users = append(users, user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"users": users,
		"total": total,
		"page":  page,
		"pageSize": pageSize,
	})
}
```

**Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(dashboard): add user filter by anonymous status"
```

---

## Task 4: Frontend - Anonymous Setting Toggle

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Update renderAuthSection to include anonymous toggle**

Find `renderAuthSection` and add after the email confirmation toggle:

```javascript
// Add after the email confirmation toggle div:
<div class="setting-group">
    <label class="setting-toggle">
        <input type="checkbox"
               ${authConfig.allow_anonymous ? 'checked' : ''}
               onchange="App.toggleAnonymousSignin(this.checked)">
        <span>Allow anonymous sign-in</span>
    </label>
    <p class="text-muted" style="margin-top: 4px; margin-left: 24px;">
        When enabled, users can sign in without email or password.
        ${authConfig.anonymous_user_count !== undefined ?
          `<br>Anonymous users: <strong>${authConfig.anonymous_user_count}</strong>` : ''}
    </p>
</div>

<hr style="margin: 16px 0; border: none; border-top: 1px solid var(--border-color);">
```

**Step 2: Implement toggleAnonymousSignin**

Add to App object:

```javascript
async toggleAnonymousSignin(enabled) {
    try {
        const res = await fetch('/_/api/settings/auth/anonymous', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled })
        });

        if (!res.ok) throw new Error('Failed to update setting');

        this.showToast(
            enabled ? 'Anonymous sign-in enabled' : 'Anonymous sign-in disabled',
            'success'
        );

        // Reload settings to refresh count
        await this.loadSettings();
    } catch (err) {
        this.showToast(err.message, 'error');
        // Revert checkbox on error
        this.render();
    }
},
```

**Step 3: Update loadSettings to include auth config**

Ensure auth config is loaded and includes anonymous settings:

```javascript
// In loadSettings or where auth settings are fetched:
const authRes = await fetch('/_/api/settings/auth');
if (authRes.ok) {
    const authData = await authRes.json();
    this.state.settings.authConfig = {
        ...this.state.settings.authConfig,
        allow_anonymous: authData.allow_anonymous,
        anonymous_user_count: authData.anonymous_user_count
    };
}
```

**Step 4: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Toggle anonymous sign-in in settings

**Step 5: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add anonymous sign-in toggle"
```

---

## Task 5: Frontend - Users List Filter and Badge

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add filter state to users**

In the state initialization, add filter:

```javascript
users: {
    loading: false,
    list: [],
    page: 1,
    pageSize: 20,
    totalUsers: 0,
    filter: 'all'  // ADD THIS: 'all', 'regular', 'anonymous'
},
```

**Step 2: Update renderUsersView toolbar**

Find `renderUsersView` and add filter dropdown in the toolbar:

```javascript
// Add after search input in toolbar:
<select class="form-select" style="width: auto;"
        onchange="App.setUserFilter(this.value)">
    <option value="all" ${this.state.users.filter === 'all' ? 'selected' : ''}>All Users</option>
    <option value="regular" ${this.state.users.filter === 'regular' ? 'selected' : ''}>Regular</option>
    <option value="anonymous" ${this.state.users.filter === 'anonymous' ? 'selected' : ''}>Anonymous</option>
</select>
```

**Step 3: Implement setUserFilter**

```javascript
setUserFilter(filter) {
    this.state.users.filter = filter;
    this.state.users.page = 1;
    this.loadUsers();
},
```

**Step 4: Update loadUsers to include filter**

```javascript
// In loadUsers, add filter to URL:
const { page, pageSize, filter } = this.state.users;
const res = await fetch(`/_/api/users?page=${page}&pageSize=${pageSize}&filter=${filter}`);
```

**Step 5: Update user row rendering to show badge**

Find where user rows are rendered and add anonymous badge:

```javascript
// In the email/name column:
<td>
    ${user.is_anonymous ? `
        <span class="text-muted">(anonymous)</span>
        <span class="badge badge-muted">Anon</span>
    ` : this.escapeHtml(user.email)}
</td>
```

**Step 6: Add CSS for badge**

```css
/* Anonymous badge */
.badge {
    display: inline-block;
    font-size: 0.7rem;
    padding: 0.125rem 0.375rem;
    border-radius: 3px;
    margin-left: 0.5rem;
    vertical-align: middle;
}

.badge-muted {
    background: var(--muted-bg);
    color: var(--muted);
}
```

**Step 7: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Filter users, verify badges

**Step 8: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): add user filter and anonymous badge"
```

---

## Task 6: Frontend - Anonymous User Delete Warning

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Update delete confirmation for anonymous users**

Find the user delete function and add specific warning:

```javascript
async deleteUser(userId) {
    const user = this.state.users.list.find(u => u.id === userId);
    if (!user) return;

    let message = `Delete user "${user.email}"? This cannot be undone.`;

    if (user.is_anonymous) {
        message = `Delete this anonymous user?\n\nUser ID: ${userId}\nCreated: ${new Date(user.created_at).toLocaleDateString()}\n\nThis will permanently delete this anonymous user and all associated data. This action cannot be undone.`;
    }

    if (!confirm(message)) return;

    try {
        const res = await fetch(`/_/api/users/${userId}`, {
            method: 'DELETE'
        });

        if (!res.ok) throw new Error('Failed to delete user');

        this.showToast('User deleted', 'success');
        await this.loadUsers();
    } catch (err) {
        this.showToast(err.message, 'error');
    }
},
```

**Step 2: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Delete an anonymous user, verify warning

**Step 3: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add specific warning for anonymous user deletion"
```

---

## Task 7: E2E Tests

**Files:**
- Create: `e2e/tests/dashboard/anonymous.test.ts`
- Modify: `e2e/tests/auth/anonymous.test.ts` (add disabled test)

**Step 1: Create dashboard anonymous tests**

```typescript
import { test, expect } from '@playwright/test';

test.describe('Dashboard Anonymous Settings', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:8080/_');
    const loginForm = page.locator('form');
    if (await loginForm.isVisible()) {
      await page.fill('input[type="password"]', 'testpassword');
      await page.click('button[type="submit"]');
    }
  });

  test('should toggle anonymous sign-in setting', async ({ page }) => {
    await page.click('text=Settings');
    await page.click('text=Authentication');

    const toggle = page.locator('input[onchange*="toggleAnonymousSignin"]');
    const initialState = await toggle.isChecked();

    // Toggle off
    if (initialState) {
      await toggle.click();
      await expect(page.locator('text=Anonymous sign-in disabled')).toBeVisible();
    }

    // Toggle on
    await toggle.click();
    await expect(page.locator('text=Anonymous sign-in enabled')).toBeVisible();
  });

  test('should display anonymous user count', async ({ page }) => {
    await page.click('text=Settings');
    await page.click('text=Authentication');

    await expect(page.locator('text=Anonymous users:')).toBeVisible();
  });

  test('should filter users by anonymous status', async ({ page }) => {
    await page.click('text=Users');

    // Filter to anonymous
    await page.selectOption('select', 'anonymous');
    await page.waitForLoadState('networkidle');

    // All visible users should be anonymous
    const badges = page.locator('.badge:has-text("Anon")');
    const userRows = page.locator('tbody tr');

    // Either no users or all have badges
    const rowCount = await userRows.count();
    if (rowCount > 0) {
      const badgeCount = await badges.count();
      expect(badgeCount).toBe(rowCount);
    }
  });

  test('should show anonymous badge on user row', async ({ page }) => {
    // First create an anonymous user via API
    await page.request.post('http://localhost:8080/auth/v1/signup', {
      data: {}
    });

    await page.click('text=Users');
    await page.selectOption('select', 'anonymous');

    await expect(page.locator('.badge:has-text("Anon")')).toBeVisible();
  });
});
```

**Step 2: Add test for disabled anonymous signup**

Add to `e2e/tests/auth/anonymous.test.ts`:

```typescript
test('should reject anonymous signup when disabled', async () => {
  // First disable via dashboard API
  await fetch('http://localhost:8080/_/api/settings/auth/anonymous', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled: false })
  });

  // Try anonymous signup
  const { error } = await supabase.auth.signInAnonymously();

  expect(error).not.toBeNull();
  expect(error?.message).toContain('disabled');

  // Re-enable for other tests
  await fetch('http://localhost:8080/_/api/settings/auth/anonymous', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled: true })
  });
});
```

**Step 3: Run tests**

Run: `cd e2e && npm test -- tests/dashboard/anonymous.test.ts tests/auth/anonymous.test.ts`
Expected: All tests pass

**Step 4: Commit**

```bash
git add e2e/tests/dashboard/anonymous.test.ts e2e/tests/auth/anonymous.test.ts
git commit -m "test(e2e): add anonymous sign-in settings tests"
```

---

## Task 8: Documentation Updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `e2e/TESTS.md`

**Step 1: Update CLAUDE.md**

Add to Dashboard endpoints:

```markdown
| `/_/api/settings/auth/anonymous` | POST | Toggle anonymous sign-in { enabled: bool } |
| `/_/api/users?filter={filter}` | GET | List users with filter (all, regular, anonymous) |
```

**Step 2: Update e2e/TESTS.md**

Add anonymous settings tests to inventory.

**Step 3: Commit**

```bash
git add CLAUDE.md e2e/TESTS.md
git commit -m "docs: add anonymous sign-in settings documentation"
```

---

## Summary

This plan implements Anonymous Sign-in Settings in 8 tasks:

1. Backend - Store anonymous setting
2. Backend - Enforce setting in auth handlers
3. Backend - Users list filter
4. Frontend - Anonymous toggle in settings
5. Frontend - Users filter and badge
6. Frontend - Delete warning for anonymous users
7. E2E tests
8. Documentation updates

Each task is self-contained with clear files, code, and commit points.
