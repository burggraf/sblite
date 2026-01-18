# SQL Browser Design

## Overview

The Database Browser provides a SQL query interface with an IDE-like editor, paginated results table, and query history.

## Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Database Browser                          [History ▼]       │
├─────────────────────────────────────────────────────────────┤
│ SQL Editor                                                  │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ SELECT * FROM users WHERE email LIKE '%@example.com'    │ │
│ │ LIMIT 100;                                              │ │
│ │                                           [tables ▼]    │ │
│ └─────────────────────────────────────────────────────────┘ │
│ [Run Query]  [Clear]                    [Export CSV] [JSON] │
├─────────────────────────────────────────────────────────────┤
│ Results (47 rows, 12ms)                     Page 1 of 5     │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ id       │ email              │ created_at             │ │
│ │──────────┼────────────────────┼────────────────────────│ │
│ │ abc-123  │ user@example.com   │ 2024-01-15T10:30:00Z   │ │
│ │ def-456  │ admin@example.com  │ 2024-01-14T09:15:00Z   │ │
│ └─────────────────────────────────────────────────────────┘ │
│                              [< Prev] [1] [2] [3] [Next >]  │
└─────────────────────────────────────────────────────────────┘
```

## SQL Editor

The editor provides an IDE-like experience for writing SQL queries.

### Syntax Highlighting
- Keywords (SELECT, FROM, WHERE, etc.) in blue
- Strings in green
- Numbers in orange
- Comments in gray
- Table/column names in default color

### Autocomplete
- Triggered by typing or pressing Ctrl+Space
- Shows dropdown with matching suggestions
- Sources: SQL keywords, table names, column names (from loaded schema)
- Tab or Enter to accept suggestion
- Escape to dismiss

### Implementation Approach
Rather than adding a heavy library like CodeMirror, we'll use a custom lightweight solution:
- Transparent `<textarea>` for input (handles typing, cursor, selection)
- Overlaid `<pre>` element with highlighted HTML (display only)
- Sync scroll position between the two
- Simple regex-based tokenizer for highlighting
- Autocomplete popup positioned at cursor location

### Keyboard Shortcuts
- Ctrl+Enter / Cmd+Enter: Run query
- Ctrl+Space: Trigger autocomplete
- Escape: Close autocomplete, clear selection

### Table/Column Picker
A dropdown button showing available tables. Clicking a table shows its columns. Clicking a column inserts it at cursor position.

## Results Display

The results panel shows query output in a paginated table.

### Results Header
- Row count and execution time: "47 rows, 12ms"
- Page indicator: "Page 1 of 5"
- Export buttons: [CSV] [JSON]

### Table Features
- Column headers from query result
- Sortable columns (click header to sort, click again to reverse)
- Horizontal scroll for wide tables
- Truncate long cell values with ellipsis (hover to see full value)
- NULL values displayed as `null` in italic gray

### Pagination
- 50 rows per page (configurable via dropdown)
- Previous/Next buttons
- Page number buttons (1, 2, 3... with ellipsis for many pages)
- Jump to first/last page

### Empty States
- Before query: "Run a query to see results"
- No results: "Query returned 0 rows"
- Error: Red error message with SQL error details

### Non-SELECT Queries
For INSERT, UPDATE, DELETE, CREATE, etc.:
- Show success message: "Query executed successfully"
- Show affected row count: "3 rows affected"
- No table displayed (just the message)

## History & Safety

### Query History
- Stored in localStorage (last 30 queries)
- Dropdown in header shows recent queries
- Each entry shows: truncated SQL, row count, timestamp
- Click to restore query to editor
- Clear History button at bottom

### History Entry Format
```
SELECT * FROM users WHERE...     47 rows   2 min ago
INSERT INTO products...          1 row     5 min ago
DROP TABLE temp_data             -         10 min ago
```

### Destructive Query Protection
When executing DELETE, DROP, TRUNCATE, or ALTER statements:
- Show confirmation modal before executing
- Modal displays: "This query may modify or delete data"
- Shows the full SQL statement
- Require typing "CONFIRM" to proceed (for DROP/TRUNCATE)
- Simple Yes/No for DELETE with WHERE clause
- Warn if DELETE has no WHERE clause: "This will delete ALL rows"

### Backend Endpoint
New endpoint `POST /_/api/sql` accepts:
```json
{
  "query": "SELECT * FROM users LIMIT 50",
  "params": []
}
```

Returns:
```json
{
  "columns": ["id", "email", "created_at"],
  "rows": [["abc-123", "user@example.com", "2024-01-15"]],
  "row_count": 47,
  "execution_time_ms": 12,
  "type": "SELECT"
}
```

## Implementation

### State Structure
```javascript
state.sqlBrowser = {
  query: '',
  results: null,       // { columns, rows, rowCount, time, type }
  loading: false,
  error: null,
  history: [],         // from localStorage
  page: 1,
  pageSize: 50,
  sort: { column: null, direction: null },
  autocomplete: { show: false, items: [], selected: 0, position: {} },
  tables: [],          // for autocomplete
  showHistory: false,
}
```

### New Backend Endpoint
`POST /_/api/sql` in `internal/dashboard/handler.go`:
- Requires dashboard auth
- Executes raw SQL against the database
- Returns columns, rows, timing info
- Detects query type (SELECT, INSERT, etc.)

### Key Frontend Functions
- `initSqlBrowser()` - Load history, fetch table list for autocomplete
- `runSqlQuery()` - Execute query, update results
- `highlightSql(sql)` - Return HTML with syntax highlighting
- `getSqlAutocomplete(sql, cursorPos)` - Get suggestions at cursor
- `exportResults(format)` - Download as CSV or JSON
- `confirmDestructiveQuery(sql)` - Show confirmation modal

### Files Changed
- `internal/dashboard/handler.go` - Add SQL endpoint
- `internal/dashboard/static/app.js` - SQL Browser UI + logic
- `internal/dashboard/static/style.css` - Editor and results styling

## Testing

E2E tests in `e2e/tests/dashboard/sql-browser.test.ts`:

### SQL Browser View
- navigates to SQL Browser section
- displays SQL editor and results panel
- shows table picker dropdown

### SQL Editor
- can type SQL in editor
- syntax highlighting colors keywords
- Ctrl+Enter runs query
- autocomplete shows on Ctrl+Space
- clicking table in picker inserts name

### Query Execution
- SELECT query displays results table
- shows row count and execution time
- INSERT query shows success message
- invalid SQL shows error message
- empty query shows warning

### Results Table
- pagination controls work
- clicking column header sorts results
- export CSV downloads file
- export JSON downloads file

### Destructive Query Protection
- DELETE shows confirmation dialog
- DROP requires typing CONFIRM
- canceling prevents execution

### History
- query added to history after execution
- clicking history item restores query
- history persists after reload
- clear history removes entries

~24 tests covering the main functionality.
