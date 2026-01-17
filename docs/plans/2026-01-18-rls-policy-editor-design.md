# RLS Policy Editor Design

## Overview

Add a visual policy editor to the dashboard for managing Row Level Security policies. The RLS backend is already fully implemented - this adds the UI layer.

## UI Structure

### Two-Panel Layout

**Left Panel - Table List**
- Lists all user tables (excluding system tables like `auth_users`, `_rls_policies`)
- Each table shows: name, policy count badge, RLS enabled/disabled indicator
- Click a table to view its policies in the right panel
- Toggle switch next to each table to enable/disable RLS

**Right Panel - Policy List & Actions**
- Header shows selected table name + "New Policy" button
- When RLS is disabled: shows warning banner with "Enable RLS" button
- When RLS is enabled with no policies: shows empty state with "Create your first policy"
- Policy cards display:
  - Policy name
  - Command type (SELECT/INSERT/UPDATE/DELETE/ALL)
  - USING expression preview (truncated)
  - CHECK expression preview if present
  - Enabled/disabled toggle
  - Edit and Delete buttons

## Policy Creation/Edit Modal

### Form Fields
- **Policy Name** - Text input, required (e.g., "Users can view own data")
- **Command** - Dropdown: SELECT, INSERT, UPDATE, DELETE, ALL
- **USING Expression** - Textarea for the WHERE-style condition
  - Shows for SELECT, UPDATE, DELETE, ALL
  - Placeholder: `auth.uid() = user_id`
  - Helper text explaining this filters which rows are visible/affected
- **CHECK Expression** - Textarea for INSERT/UPDATE validation
  - Shows for INSERT, UPDATE, ALL
  - Placeholder: `auth.uid() = user_id`
  - Helper text explaining this validates new/modified data

### SQL Preview Panel
- Read-only code block showing the generated SQL:
  ```sql
  CREATE POLICY "Users can view own data"
  ON public.posts
  FOR SELECT
  USING (auth.uid() = user_id);
  ```
- Updates live as user types in form fields
- Syntax highlighting for readability

### Template Dropdown
- Button: "Use Template" opens dropdown with options:
  - "Users can view their own data" → `auth.uid() = user_id`
  - "Authenticated users can read" → `auth.uid() IS NOT NULL`
  - "Anyone can read" → `true`
  - "Users can modify their own data" → sets both USING and CHECK
- Selecting a template populates the expression fields

### Footer
- Cancel button
- Save/Create button (primary)

## Policy Testing Feature

### Test Panel
- Collapsible section within the modal, below the form
- Toggle/accordion: "Test Policy" - expands to show testing UI

### Test Configuration
- **User Context** - Dropdown populated with users from `auth_users`
  - Shows email as label, stores user ID
  - Selected user's `auth.uid()`, `auth.role()`, `auth.email()` are substituted into expressions
  - Option: "Anonymous (no user)" for testing unauthenticated access

### Test Execution
- "Run Test" button
- For USING expressions: Runs a SELECT with the policy condition, shows count of matching rows
- For CHECK expressions: Shows whether a sample row would pass validation

### Test Results Display
- Success state: Green checkmark + "Policy would allow access to X rows"
- Failure state: Red X + "Policy would deny access" or "Expression error: {message}"
- Shows the actual SQL executed (with substituted values) in a collapsible detail

## API Endpoints

### Policy CRUD
- `GET /_/api/policies` - List all policies (grouped by table)
- `GET /_/api/policies?table={name}` - List policies for specific table
- `POST /_/api/policies` - Create new policy
- `PATCH /_/api/policies/{id}` - Update policy (name, expressions, enabled)
- `DELETE /_/api/policies/{id}` - Delete policy

### RLS Table State
- `GET /_/api/tables/{name}/rls` - Get RLS enabled status for table
- `PATCH /_/api/tables/{name}/rls` - Enable/disable RLS for table

### Policy Testing
- `POST /_/api/policies/test` - Test a policy expression
  - Request: `{ table, using_expr, check_expr, user_id }`
  - Response: `{ success, row_count, error, executed_sql }`

### Request/Response Examples

Create policy:
```json
POST /_/api/policies
{
  "table_name": "posts",
  "policy_name": "Users view own",
  "command": "SELECT",
  "using_expr": "auth.uid() = user_id",
  "check_expr": null,
  "enabled": true
}
```

Test policy:
```json
POST /_/api/policies/test
{
  "table": "posts",
  "using_expr": "auth.uid() = user_id",
  "user_id": "abc-123-def"
}
// Response: { "success": true, "row_count": 12 }
```

## Error Handling

### Expression Validation
- Invalid SQL syntax → Show error inline below the expression field: "Syntax error: {details}"
- Unknown column reference → "Column 'xyz' does not exist in table 'posts'"
- Invalid auth function → "Unknown function: auth.foo()"

### RLS State Warnings
- Table with RLS disabled + no policies: Info banner "RLS is disabled. All rows are accessible."
- Table with RLS enabled + no policies: Warning banner "RLS is enabled but no policies exist. All access is denied."
- Disabling RLS on a table with policies: Confirmation dialog "Disabling RLS will make all rows accessible regardless of policies. Continue?"

### Delete Confirmation
- Deleting a policy: "Delete policy '{name}'? This cannot be undone."
- If it's the last policy on a table with RLS enabled: Additional warning "This is the only policy on this table. Deleting it will deny all access."

### Test Failures
- Expression error during test → Show the SQL error message, highlight problematic expression
- No matching rows → Neutral state (not an error): "0 rows match this policy for selected user"
- User has no ID (anonymous test) → `auth.uid()` substitutes to `NULL`

### Concurrent Editing
- Keep it simple: Last write wins, no locking
- If policy was deleted while editing: "This policy no longer exists" on save attempt

## E2E Testing Plan

### Policy List Tests
- Displays table list in left panel
- Shows policy count badge per table
- Clicking table shows its policies
- RLS toggle enables/disables RLS for table
- Empty state shown when no policies exist

### Policy CRUD Tests
- Opens create policy modal when clicking "New Policy"
- Form fields populate correctly for each command type
- SQL preview updates as user types
- Creating policy adds it to the list
- Editing policy opens modal with existing values
- Deleting policy removes it from list (with confirmation)
- Policy enabled toggle works inline

### Template Tests
- Template dropdown populates expression fields
- Different templates set appropriate USING/CHECK values

### Policy Test Feature Tests
- User dropdown populates with users from database
- Running test shows row count for USING expressions
- Invalid expressions show error message
- Anonymous user option works (NULL uid)

### Error Handling Tests
- Invalid expression shows validation error
- Disabling RLS shows confirmation when policies exist
- Deleting last policy shows additional warning

### Test File Location
- `e2e/tests/dashboard/policies.test.ts`

## Implementation Notes

### Existing RLS Infrastructure
The following backend components already exist and should be reused:
- `internal/rls/policy.go` - Policy CRUD service
- `internal/rls/enforcer.go` - Query condition generation
- `internal/rls/rewriter.go` - Auth function substitution (`auth.uid()`, etc.)
- `_rls_policies` table in database schema

### New Dashboard Endpoints
Add handlers in `internal/dashboard/handler.go` that wrap the existing `internal/rls` package.

### Frontend Components
Add to `internal/dashboard/static/app.js`:
- `renderPoliciesView()` - Main two-panel layout
- `renderPolicyModal()` - Create/edit modal with SQL preview
- `renderPolicyTestPanel()` - Testing UI within modal
- Policy CRUD methods and API calls
