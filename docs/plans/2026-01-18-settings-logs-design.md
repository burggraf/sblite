# Settings & Logs Views Design

## Overview

Phase 6 implements the Settings and Logs views for the sblite dashboard, completing the admin interface.

## Settings View

### Structure

Organized into collapsible accordion sections:

1. **Server Information** (expanded by default)
   - Version, host, port, database path, log mode
   - Runtime stats: uptime, memory usage, active connections
   - Request statistics: total requests, requests/minute

2. **Authentication**
   - JWT secret display (masked: `***...abc123`)
   - "Regenerate Secret" button with confirmation modal
   - Current token expiry settings display

3. **Email Templates**
   - List of 5 templates: confirmation, recovery, magic_link, email_change, invite
   - Inline editing of subject and body
   - Save button per template, "Reset to Default" option

4. **Export & Backup**
   - "Export PostgreSQL Schema" - downloads `.sql` file
   - "Export Data" - select table(s), format (JSON/CSV)
   - "Download Database Backup" - downloads entire `.db` file

### API Endpoints

**Server Info**
- `GET /_/api/settings/server` - Server config and runtime stats

**Authentication**
- `GET /_/api/settings/auth` - Masked JWT info and token settings
- `POST /_/api/settings/auth/regenerate-secret` - Generate new JWT secret

**Email Templates**
- `GET /_/api/settings/templates` - List all templates
- `PATCH /_/api/settings/templates/{type}` - Update template
- `POST /_/api/settings/templates/{type}/reset` - Reset to default

**Export**
- `GET /_/api/export/schema` - PostgreSQL DDL
- `GET /_/api/export/data?tables=x&format=json` - Export table data
- `GET /_/api/export/backup` - Download database file

## Logs View

### Structure

Two modes based on server log configuration:

**When log-mode=database:**
- Filtering toolbar: level dropdown, time range picker, text search, user_id filter, request_id filter
- Log table: timestamp, level (color-coded), message, source, user_id
- Click row to expand full details with `extra` JSON
- Pagination (50 per page)
- Refresh button

**When log-mode=console or file:**
- Info message about current log destination
- "Tail Recent Logs" button for file mode (last 100 lines)
- Suggestion to enable database logging

### API Endpoints

**Log Query**
- `GET /_/api/logs` - Query with filters (level, since, until, search, user_id, request_id, limit, offset)

**Log Config**
- `GET /_/api/logs/config` - Current log mode and settings

**File Tail**
- `GET /_/api/logs/tail?lines=100` - Last N lines from log file

## Error Handling

### Settings
- Template save fails: inline error, keep form editable
- JWT regeneration fails: error in modal, don't close
- Export fails: toast error with retry option
- Large backup: progress indicator, handle timeout

### Logs
- No log database: show setup instructions
- File not readable: show permission error
- No results: "No logs match filters" with clear button
- Timeout: limit default to 24 hours, warn on broader queries

### JWT Regeneration Safety
- Require typing "REGENERATE" to confirm
- Store new secret in `_dashboard` table
- Invalidate all refresh tokens immediately
- Dashboard session remains valid (separate cookie auth)

## E2E Tests

### Settings Tests
- Navigate to Settings section
- Display server information
- Show runtime stats
- Display masked JWT secret
- JWT regeneration flow with confirmation
- List email templates
- Edit template subject and body
- Reset template to default
- Export PostgreSQL schema
- Export table data as JSON/CSV
- Download database backup

### Logs Tests
- Navigate to Logs section
- Show "not enabled" message when log-mode=console
- Display logs when database logging enabled
- Filter by log level
- Filter by time range
- Search log messages
- Pagination
- Expand log entry details
- Refresh button reloads logs
