# Dashboard Remaining Work

This document tracks dashboard features from the original design that haven't been implemented yet. Reference: `docs/plans/2026-01-17-dashboard-design.md`

## CLI Commands

### `sblite db reset`
Drop all tables and rerun all migrations from scratch. Useful for development.

```bash
sblite db reset --db data.db
```

**Implementation:**
- Add `cmd/db_reset.go`
- Drop all user tables (exclude `auth_*`, `_*` system tables)
- Clear `_schema_migrations`
- Re-run all migrations from `./migrations/`

### `sblite db diff`
Show unapplied schema changes as SQL. Compares current database state with migration files.

```bash
sblite db diff --db data.db
```

**Implementation:**
- Add `cmd/db_diff.go`
- Compare current schema against what migrations would produce
- Output SQL statements to sync them

### `sblite dashboard` subcommands

```bash
sblite dashboard setup              # Interactive: prompt for password
sblite dashboard reset-password     # Reset password (prompts securely)
sblite dashboard disable            # Disable dashboard entirely
sblite dashboard enable             # Re-enable dashboard
```

**Implementation:**
- Add `cmd/dashboard.go` with subcommands
- `setup` - only works if no password exists
- `reset-password` - invalidates all sessions
- `disable/enable` - set flag in `_dashboard` table

## Dashboard UI Features

### Login Rate Limiting
Limit login attempts to 5 per minute per IP.

**Implementation:**
- Add in-memory rate limiter to `internal/dashboard/auth.go`
- Track failed attempts by IP
- Return 429 Too Many Requests when exceeded

### Logs Auto-Refresh
Toggle to automatically poll for new logs every 2 seconds.

**Implementation:**
- Add `autoRefresh` state to logs view
- Toggle button in UI
- `setInterval` to fetch new logs when enabled
- Stop polling when navigating away

### Logs Time Range Filter
Filter logs by time range: Last hour, 24h, 7d, or custom date range.

**Implementation:**
- Add time range dropdown/picker to logs view
- Pass `since` and `until` params to `/_/api/logs`
- Backend already supports these params

### Dashboard Password Change
Allow changing the dashboard password from the Settings panel.

**Implementation:**
- Add "Change Password" section to Settings
- Require current password + new password + confirm
- Update `password_hash` in `_dashboard` table
- Invalidate current session (require re-login)

### Auth Configuration UI
Allow configuring JWT expiry times from the dashboard.

**Implementation:**
- Add editable fields for access token expiry (default: 1 hour)
- Add editable fields for refresh token expiry (default: 1 week)
- Store in `_dashboard` table or config
- Note: May require server restart to take effect

## Priority

| Feature | Priority | Effort |
|---------|----------|--------|
| Login rate limiting | High | Small |
| Dashboard password change | High | Small |
| Logs time range filter | Medium | Small |
| Logs auto-refresh | Medium | Small |
| `sblite db reset` | Medium | Medium |
| `sblite dashboard` CLI | Low | Medium |
| `sblite db diff` | Low | Large |
| Auth configuration UI | Low | Medium |

## Notes

- The dashboard is fully functional for typical use cases
- These are nice-to-have features for power users and production deployments
- `db diff` is the most complex as it requires schema comparison logic
