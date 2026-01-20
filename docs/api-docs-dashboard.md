# API Docs Dashboard

The API Docs dashboard provides auto-generated API documentation based on your database schema. It displays documentation for tables, columns, and RPC functions with code examples in JavaScript (supabase-js) and Bash (curl).

## Overview

The API Docs feature is accessible from the dashboard sidebar under "Documentation > API Docs". It provides:

- **Auto-generated documentation** for all your database tables and RPC functions
- **Code examples** in both JavaScript (using @supabase/supabase-js) and Bash (using curl)
- **Editable descriptions** for tables, columns, and functions
- **Supabase-compatible** documentation format

## Navigation Structure

```
API Docs
├── Introduction
├── Authentication
├── User Management
├── Tables and Views
│   ├── Introduction
│   └── [dynamic list of your tables]
└── Stored Procedures
    ├── Introduction
    └── [dynamic list of your RPC functions]
```

## Pages

### Introduction

Explains how to connect to your sblite project using the Supabase client library. Shows:
- Project URL
- API key usage
- Getting started code example

### Authentication

Documents the API authentication system:
- Client API keys (anon key) for client-side code
- Service keys (service_role) for server-side code
- Security warnings about key usage

### User Management

Comprehensive documentation of auth operations:
- Sign Up (email/password)
- Sign In with Password
- Sign In with Magic Link
- Get User
- Update User
- Sign Out
- Password Recovery
- Invite User (admin only)

### Tables Introduction

Overview of the REST API conventions:
- List of available tables
- REST endpoint patterns (GET, POST, PATCH, DELETE)

### Table Documentation (Dynamic)

For each table in your database, shows:
- Table name and editable description
- Column details with:
  - Required/Optional badge
  - Type (string, number, boolean)
  - Format (uuid, text, integer, etc.)
  - Editable description
  - SELECT code example
- CRUD operations:
  - Read All Rows
  - Read Specific Columns
  - With Pagination
  - With Filtering
  - Insert a Row
  - Insert Many Rows
  - Upsert
  - Update Rows
  - Delete Rows

### Stored Procedures Introduction

Overview of RPC function calls via the API.

### Function Documentation (Dynamic)

For each RPC function, shows:
- Function name and editable description
- Invoke function code example
- Return type information
- Arguments with:
  - Required/Optional badge
  - Type and format

## Language Toggle

All code examples can be viewed in two languages:
- **JavaScript**: Uses `@supabase/supabase-js` syntax
- **Bash**: Uses `curl` with proper headers

Click the language tabs at the top right of any documentation page to switch.

## Editable Descriptions

Tables, columns, and functions can have descriptions that help document your API:

1. Navigate to the table or function page
2. Find the "Description" section
3. Enter your description in the text field
4. Click "Save"

Descriptions are stored in the database and persist across restarts:
- Table descriptions: `_table_descriptions` table
- Column descriptions: `description` column in `_columns` table
- Function descriptions: `_function_descriptions` table

## API Endpoints

The API Docs dashboard uses these backend endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_/api/apidocs/tables` | GET | List all tables with column metadata |
| `/_/api/apidocs/tables/{name}` | GET | Get detailed schema for a table |
| `/_/api/apidocs/tables/{name}/description` | PATCH | Update table description |
| `/_/api/apidocs/tables/{name}/columns/{col}/description` | PATCH | Update column description |
| `/_/api/apidocs/functions` | GET | List all RPC functions |
| `/_/api/apidocs/functions/{name}` | GET | Get function details |
| `/_/api/apidocs/functions/{name}/description` | PATCH | Update function description |

## Database Schema

The API Docs feature adds these tables to store descriptions:

```sql
-- Table descriptions
CREATE TABLE _table_descriptions (
    table_name    TEXT PRIMARY KEY,
    description   TEXT DEFAULT '',
    updated_at    TEXT DEFAULT (datetime('now'))
);

-- Function descriptions
CREATE TABLE _function_descriptions (
    function_name TEXT PRIMARY KEY,
    description   TEXT DEFAULT '',
    updated_at    TEXT DEFAULT (datetime('now'))
);

-- Column descriptions (added to existing _columns table)
ALTER TABLE _columns ADD COLUMN description TEXT DEFAULT '';
```

## Example Usage

### Accessing API Docs

1. Log into the sblite dashboard at `/_/`
2. Click "API Docs" in the sidebar under "Documentation"
3. Browse the documentation sections

### Adding a Table Description

1. Navigate to API Docs > Tables and Views > [your table]
2. In the "Description" section, enter your description
3. Click "Save"

### Viewing Code Examples

1. Navigate to any table or function page
2. Use the JavaScript/Bash tabs to switch languages
3. Copy the code examples for use in your application

## Supabase Compatibility

The API Docs format matches Supabase's API documentation style, making it familiar for developers who have used Supabase before. The generated code examples work directly with the `@supabase/supabase-js` client library.
