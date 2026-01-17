# Dashboard Table Management - Design Document

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add full table management UI to the sblite dashboard with data browsing, inline editing, and schema management.

**Architecture:** Frontend JavaScript SPA with dashboard API proxy routes to existing admin and REST APIs. Session-authenticated access with service-role bypass for RLS.

**Tech Stack:** Vanilla JavaScript, existing CSS variables, Playwright for testing

---

## Features

### Data Browsing
- Table list in sidebar showing all user tables
- Data grid view with sortable columns
- Fixed pagination (25/50/100 rows per page)
- Column-specific filters (text search, number ranges, date pickers based on type)

### Data Editing
- Inline cell editing: click a cell to edit in place
- Row modal: click row menu → "Edit row" opens full form with all fields
- Add new row via modal form
- Checkbox selection with bulk delete

### Schema Management
- Create new table wizard (name, columns with types)
- View/edit existing schema: add, rename, or drop columns
- Delete table (with confirmation)

---

## UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Tables                                    [+ New Table]     │
├─────────────────────────────────────────────────────────────┤
│ ┌─────────────┐  ┌────────────────────────────────────────┐ │
│ │ Table List  │  │  Data Grid                             │ │
│ │             │  │  ┌─────┬─────────┬──────────┬────────┐ │ │
│ │ ☑ users     │  │  │ ☐   │ id      │ name     │ email  │ │ │
│ │   posts     │  │  ├─────┼─────────┼──────────┼────────┤ │ │
│ │   comments  │  │  │ ☐   │ abc-123 │ Alice    │ a@...  │ │ │
│ │             │  │  │ ☐   │ def-456 │ Bob      │ b@...  │ │ │
│ │             │  │  └─────┴─────────┴──────────┴────────┘ │ │
│ │             │  │                                        │ │
│ │             │  │  [Delete Selected]    Page 1 of 5  ◀ ▶ │ │
│ └─────────────┘  └────────────────────────────────────────┘ │
│                                                             │
│ Schema tab: [Columns] [Add Column]                         │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ name (text) PK │ email (text) │ created_at (timestamptz)│ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Components

1. **Table List Panel** (left): Clickable list of tables, selected table highlighted
2. **Toolbar**: "New Table" button, table actions dropdown (rename, delete)
3. **Data Grid**: Sortable headers, filter dropdowns, inline editing, row checkboxes
4. **Pagination Bar**: Rows per page selector, page navigation
5. **Schema Panel** (collapsible): Column list with types, add/edit/delete column actions
6. **Modals**: New table wizard, edit row form, add column form, delete confirmations

---

## API Routes

### Dashboard Proxy Routes

All routes require valid dashboard session cookie.

```
GET    /_/api/tables              → List all tables
POST   /_/api/tables              → Create table
GET    /_/api/tables/{name}       → Get table schema
DELETE /_/api/tables/{name}       → Delete table
POST   /_/api/tables/{name}/columns    → Add column
PATCH  /_/api/tables/{name}/columns/{col} → Rename column
DELETE /_/api/tables/{name}/columns/{col} → Drop column

GET    /_/api/data/{table}        → Select rows
POST   /_/api/data/{table}        → Insert row
PATCH  /_/api/data/{table}        → Update row
DELETE /_/api/data/{table}        → Delete rows
```

### Column Operations (Backend)

- **Add column**: `ALTER TABLE {name} ADD COLUMN {col} {type}`
- **Rename column**: `ALTER TABLE {name} RENAME COLUMN {old} TO {new}`
- **Drop column**: Create new table, copy data, drop old, rename new (transaction)

---

## Frontend Structure

```javascript
const App = {
    state: { /* existing auth state */ },

    tables: {
        list: [],           // All table names
        selected: null,     // Currently selected table name
        schema: null,       // Selected table's column definitions
        data: [],           // Current page of rows
        page: 1,
        pageSize: 25,
        totalRows: 0,
        filters: {},        // { colName: { op: 'eq', value: 'x' } }
        sortColumn: null,
        sortDirection: 'asc',
        selectedRows: new Set(),
        editingCell: null,  // { rowId, column }
    },

    // Table list
    async loadTables() { },
    async selectTable(name) { },

    // Data operations
    async loadData() { },
    async insertRow(data) { },
    async updateCell(rowId, column, value) { },
    async deleteRows(rowIds) { },

    // Schema operations
    async createTable(name, columns) { },
    async addColumn(table, column) { },
    async renameColumn(table, oldName, newName) { },
    async dropColumn(table, column) { },
    async deleteTable(name) { },

    // Render methods
    renderTables() { },
    renderTableList() { },
    renderDataGrid() { },
    renderSchemaPanel() { },
    renderPagination() { },
    renderFilters() { },
    renderModals() { },
};
```

### Inline Editing Flow

1. Click cell → `editingCell = { rowId, column }`
2. Cell renders as `<input>` with current value
3. Blur or Enter → `updateCell()` → PATCH request → refresh cell
4. Escape → cancel edit

---

## Testing

### Playwright Tests

```
tests/dashboard/tables.test.ts
├── Table List
│   ├── displays list of tables
│   ├── selects table and loads data
│   └── shows empty state when no tables
├── Create Table
│   ├── opens create table modal
│   ├── validates required fields
│   ├── creates table with columns
│   └── shows new table in list
├── Data Grid
│   ├── displays rows with correct columns
│   ├── paginates through data
│   ├── sorts by column
│   └── filters by column value
├── Inline Editing
│   ├── edits cell on click
│   ├── saves on blur/enter
│   └── cancels on escape
├── Row Operations
│   ├── opens edit row modal
│   ├── selects multiple rows
│   ├── bulk deletes selected rows
│   └── inserts new row via modal
├── Schema Management
│   ├── displays column definitions
│   ├── adds new column
│   ├── renames column
│   └── drops column (with data preservation)
└── Delete Table
    ├── confirms before delete
    └── removes table from list
```

---

## Error Handling

| Operation | Error | User Feedback |
|-----------|-------|---------------|
| Load tables | Network/server error | "Failed to load tables" toast, retry button |
| Save cell | Validation error | Red border on cell, error tooltip |
| Save cell | Conflict/stale data | "Row was modified, refresh?" prompt |
| Delete rows | Foreign key constraint | "Cannot delete: referenced by other data" |
| Drop column | Has data | Confirmation: "This will delete data in X rows" |
| Create table | Name exists | Inline error under table name field |

### Loading States

- Skeleton loaders for table list and data grid
- Spinner overlay for save operations
- Disabled buttons during async operations
