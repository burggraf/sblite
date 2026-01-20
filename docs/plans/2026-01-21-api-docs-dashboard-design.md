# API Docs Dashboard Design

## Overview

Implement a Supabase-compatible "API Docs" dashboard feature that provides auto-generated API documentation based on the database schema. This feature displays documentation for tables, columns, and RPC functions with code examples in JavaScript (supabase-js) and Bash (curl).

## Sidebar Structure

```
API Docs
├── Introduction
├── Authentication
├── User Management
├── Tables and Views
│   ├── Introduction
│   └── [dynamic list of user tables]
└── Stored Procedures
    ├── Introduction
    └── [dynamic list of RPC functions]
```

## URL Routing

- `/_?view=apiDocs` → API Docs Introduction
- `/_?view=apiDocs&page=auth` → Authentication page
- `/_?view=apiDocs&page=users-management` → User Management
- `/_?view=apiDocs&page=tables-intro` → Tables Introduction
- `/_?view=apiDocs&resource=tablename` → Specific table docs
- `/_?view=apiDocs&page=rpc-intro` → RPC Introduction
- `/_?view=apiDocs&rpc=functionname` → Specific function docs

## State Management

```javascript
apiDocs: {
    page: 'intro',      // 'intro', 'auth', 'users-management', 'tables-intro', 'rpc-intro'
    resource: null,     // table name when viewing table docs
    rpc: null,          // function name when viewing RPC docs
    language: 'javascript', // 'javascript' or 'bash'
    tables: [],         // cached table list with schemas
    functions: [],      // cached RPC function list
}
```

## Backend API Endpoints

### New Dashboard API Endpoints

```
GET  /_/api/apidocs/tables
     Returns all user tables with column metadata (name, type, format, required, description)

GET  /_/api/apidocs/tables/{name}
     Returns detailed schema for a specific table

PATCH /_/api/apidocs/tables/{name}/description
      Updates the table description
      Body: { "description": "..." }

PATCH /_/api/apidocs/columns/{table}/{column}/description
      Updates a column description
      Body: { "description": "..." }

GET  /_/api/apidocs/functions
     Returns all RPC functions with parameter metadata

GET  /_/api/apidocs/functions/{name}
     Returns detailed info for a specific function

PATCH /_/api/apidocs/functions/{name}/description
      Updates a function description
      Body: { "description": "..." }
```

### Database Schema Changes

Add `description` column to `_columns` table:
```sql
ALTER TABLE _columns ADD COLUMN description TEXT DEFAULT '';
```

Add new `_table_descriptions` table for table-level descriptions:
```sql
CREATE TABLE _table_descriptions (
    table_name TEXT PRIMARY KEY,
    description TEXT DEFAULT ''
);
```

Add new `_function_descriptions` table for RPC function descriptions:
```sql
CREATE TABLE _function_descriptions (
    function_name TEXT PRIMARY KEY,
    description TEXT DEFAULT ''
);
```

## Static Pages Content

### Introduction Page
- Heading: "Connect To Your Project"
- Explanation of RESTful endpoint and API keys
- Code example showing `createClient()` initialization
- Uses actual sblite URL (from window.location) and placeholder for API key

### Authentication Page
- Heading: "Authentication"
- Explanation of JWT and Key auth
- **Client API Keys** section with anon key explanation and example
- **Service Keys** section with service_role key explanation and warning about server-only use
- Links to API Settings (existing settings page)

### User Management Page
- Heading: "User Management"
- Sections for each auth operation sblite supports:
  - Sign Up (email/password)
  - Log In With Email/Password
  - Log In With Magic Link (signInWithOtp)
  - Log In With OAuth (if configured)
  - Get User
  - Update User
  - Log Out
  - Password Recovery
  - Invite User (admin)
- Each section has description + code example

### Tables Introduction Page
- Heading: "Tables and Views"
- Brief explanation that all tables are listed in sidebar
- Link to REST API documentation

### RPC Introduction Page
- Heading: "Stored Procedures"
- Explanation of RPC calls via `supabase.rpc()`
- Note that functions are listed in sidebar

## Dynamic Table Documentation

### Layout

```
┌─────────────────────────────────────────────────────────┐
│ [JavaScript] [Bash]                        (top right)  │
├─────────────────────────────────────────────────────────┤
│ ## {table_name}                                         │
│                                                         │
│ DESCRIPTION                                             │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ {editable description text}              [Save]     │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ─── Column: id ─────────────────────────────────────── │
│ REQUIRED • TYPE: string • FORMAT: uuid                  │
│ DESCRIPTION: {editable}                                 │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ SELECT ID                                           │ │
│ │ let { data, error } = await supabase                │ │
│ │   .from('{table}')                                  │ │
│ │   .select('id')                                     │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ (repeat for each column...)                             │
│                                                         │
│ ─── Read rows ──────────────────────────────────────── │
│ • READ ALL ROWS example                                 │
│ • READ SPECIFIC COLUMNS example                         │
│ • WITH PAGINATION example                               │
│ • WITH FILTERING example (shows eq, gt, lt, etc.)       │
│                                                         │
│ ─── Insert rows ────────────────────────────────────── │
│ • INSERT A ROW example                                  │
│ • INSERT MANY ROWS example                              │
│ • UPSERT example                                        │
│                                                         │
│ ─── Update rows ────────────────────────────────────── │
│ • UPDATE MATCHING ROWS example                          │
│                                                         │
│ ─── Delete rows ────────────────────────────────────── │
│ • DELETE MATCHING ROWS example                          │
└─────────────────────────────────────────────────────────┘
```

### Code Generation
- All examples use actual table name and column names
- JavaScript examples use `@supabase/supabase-js` syntax
- Bash examples use `curl` with proper headers and endpoints

## Dynamic RPC Function Documentation

### Layout

```
┌─────────────────────────────────────────────────────────┐
│ [JavaScript] [Bash]                        (top right)  │
├─────────────────────────────────────────────────────────┤
│ ## {function_name}                                      │
│                                                         │
│ DESCRIPTION                                             │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ {editable description text}              [Save]     │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ─── INVOKE FUNCTION ────────────────────────────────── │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ let { data, error } = await supabase                │ │
│ │   .rpc('{function_name}', {                         │ │
│ │     param1,                                         │ │
│ │     param2                                          │ │
│ │   })                                                │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ─── Function Arguments ─────────────────────────────── │
│                                                         │
│ PARAMETER: param1                                       │
│ REQUIRED • TYPE: string • FORMAT: text                  │
│                                                         │
│ PARAMETER: param2                                       │
│ OPTIONAL • TYPE: integer • FORMAT: integer              │
│                                                         │
│ (repeat for each parameter...)                          │
└─────────────────────────────────────────────────────────┘
```

### Bash Example for RPC
```bash
curl -X POST 'http://localhost:8080/rest/v1/rpc/{function_name}' \
  -H "apikey: YOUR_API_KEY" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "param1": "value1", "param2": 123 }'
```

### Function Metadata Source
- Query `_rpc_functions` table for function definitions
- Extract parameter names, types, and required/optional status
- Generate code examples using actual function signature

## Design Decisions

1. **JavaScript/Bash tabs** - Match Supabase exactly with supabase-js and curl examples
2. **Editable descriptions** - Store in `_columns` table (extended) and new metadata tables
3. **Realtime section** - Omitted since sblite doesn't support realtime yet
4. **Navigation** - New top-level "API Docs" nav item in dashboard
5. **Styling** - Match existing sblite dashboard style for consistency

## Files to Modify

1. **`internal/db/migrations.go`**
   - Add `description` column to `_columns` table
   - Create `_table_descriptions` table
   - Create `_function_descriptions` table

2. **`internal/dashboard/handler.go`**
   - Add new API endpoints for apidocs (tables, functions, descriptions)

3. **`internal/dashboard/static/app.js`**
   - Add `apiDocs` state object
   - Add `renderApiDocs()` and sub-render functions
   - Add API methods for fetching/updating docs data
   - Add "API Docs" to navigation

4. **`internal/dashboard/static/style.css`**
   - Add styles for code blocks, language tabs, editable descriptions
   - Sidebar styles for the nested API docs navigation

## Implementation Order

1. Database migrations (add description columns/tables)
2. Backend API endpoints
3. Frontend state and navigation
4. Static pages (Introduction, Authentication, User Management)
5. Tables Introduction + dynamic table pages
6. RPC Introduction + dynamic function pages
7. Code generation utilities (JS and Bash examples)

## Notes

- No new dependencies required - uses existing patterns in the codebase
- All code examples are auto-generated from actual schema metadata
- Descriptions are persisted and survive database restarts
