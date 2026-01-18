# REST API Gaps Design

## Overview

This document describes the implementation of three REST API features that are currently missing:

1. **Many-to-Many Relationship Queries** - Query through junction tables
2. **Aliased Joins** - Query same table multiple times with different FKs
3. **Quoted Identifiers** - Support table/column names with spaces

## 1. Many-to-Many Relationship Queries

### Goal

Support queries like `users(*, roles(*))` that traverse junction tables.

### Detection (PostgREST-compatible)

A table is a "strict junction" if:
1. It has exactly 2 foreign keys pointing to different tables
2. Both FK columns are part of the table's primary key

Example:
```sql
CREATE TABLE user_roles (
    user_id TEXT REFERENCES users(id),
    role_id TEXT REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)  -- FKs are part of PK
);
```

Reference: [PostgREST PR #2262](https://github.com/PostgREST/postgrest/pull/2262)

### Query Behavior

```javascript
// Client request
supabase.from('users').select('*, roles(*)')

// SQL generation:
SELECT u.*,
       (SELECT json_group_array(json_object('id', r.id, 'name', r.name))
        FROM user_roles ur
        JOIN roles r ON ur.role_id = r.id
        WHERE ur.user_id = u.id) as roles
FROM users u
```

### Implementation

1. Add `isJunctionTable()` to `relations.go`:
   - Check table has exactly 2 FKs to different tables
   - Check both FK columns are in the primary key

2. Add `findM2MRelationship()` to `relations.go`:
   - Given source table and target table name
   - Find junction table connecting them
   - Return junction info for query building

3. Update `relation_query.go`:
   - Detect when requested relation is M2M (not direct FK)
   - Generate subquery with junction table JOIN

## 2. Aliased Joins (Self-Referential Queries)

### Goal

Query the same table multiple times with different FKs.

### Syntax (PostgREST-compatible)

```javascript
// Query messages with sender and receiver from users table
supabase.from('messages').select(`
  id,
  content,
  sender:users!sender_id(id, name),
  receiver:users!receiver_id(id, name)
`)
```

### Hint Syntax

- `alias:table!hint(cols)` - full syntax
- `hint` can be:
  - FK column name: `!sender_id` (most common)
  - FK constraint name: `!messages_sender_fkey`

### Implementation

1. Update `SelectRelation` struct in `select_parser.go`:
```go
type SelectRelation struct {
    Name    string         // Relation/table name
    Alias   string         // Optional alias
    Inner   bool           // !inner join modifier
    Hint    string         // NEW: FK column or constraint name
    Columns []SelectColumn
}
```

2. Update `parseRelationSelect()` to extract hint:
   - Parse `!hint` between table name and `(`
   - Handle both `!inner` and `!fk_hint` modifiers

3. Update `FindRelationship()` in `relations.go`:
   - Accept optional hint parameter
   - When hint provided, match against FK column or constraint name
   - When multiple FKs exist to same table and no hint, return error

4. SQL generation:
```sql
SELECT m.id, m.content,
       (SELECT json_object('id', u.id, 'name', u.name)
        FROM users u WHERE u.id = m.sender_id) as sender,
       (SELECT json_object('id', u.id, 'name', u.name)
        FROM users u WHERE u.id = m.receiver_id) as receiver
FROM messages m
```

## 3. Quoted Identifiers

### Goal

Support table/column names with spaces or special characters.

### Auto-Quoting Rules

An identifier needs quoting if it contains:
- Spaces
- Special characters (anything not `a-z`, `A-Z`, `0-9`, `_`)
- Starts with a digit
- Is a SQL reserved word

### Implementation

Add to `builder.go`:

```go
func quoteIdentifier(name string) string {
    if needsQuoting(name) {
        escaped := strings.ReplaceAll(name, `"`, `""`)
        return `"` + escaped + `"`
    }
    return name
}

func needsQuoting(name string) bool {
    if len(name) == 0 {
        return true
    }
    for i, r := range name {
        if i == 0 && unicode.IsDigit(r) {
            return true
        }
        if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
            return true
        }
    }
    return isReservedWord(name)
}
```

Apply `quoteIdentifier()` to:
- Table names in FROM clauses
- Column names in SELECT, WHERE, ORDER BY
- Table/column names in JOINs

## Testing

### Test Data Setup

Add to `e2e/scripts/setup-test-db.ts`:

```sql
-- Teams for M2M testing (user_teams already exists)
CREATE TABLE teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

-- Messages for alias testing
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    content TEXT,
    sender_id TEXT REFERENCES users(id),
    receiver_id TEXT REFERENCES users(id)
);
```

### E2E Tests to Enable

1. `should query through many-to-many join tables`
2. `should allow aliasing for multiple references to same table`
3. `should handle table names with spaces using quotes`
4. `should handle deeply nested relationships`

### Unit Tests

- `relations_test.go`: Junction table detection
- `select_parser_test.go`: Hint syntax parsing
- `builder_test.go`: Identifier quoting

## Error Handling

| Scenario | Error Message |
|----------|---------------|
| Ambiguous relationship (multiple FKs, no hint) | "Ambiguous relationship to 'users'. Specify FK with hint: users!sender_id or users!receiver_id" |
| Invalid hint | "No foreign key 'invalid_col' found. Available: sender_id, receiver_id" |
| No junction table found | "No many-to-many relationship found between 'users' and 'roles'" |

## Backward Compatibility

- All existing relationship queries work unchanged
- New syntax is additive only
- No changes to existing API behavior

## Files to Modify

| File | Changes |
|------|---------|
| `internal/rest/relations.go` | Add junction detection, hint matching |
| `internal/rest/select_parser.go` | Parse `!hint` syntax |
| `internal/rest/relation_query.go` | M2M query generation |
| `internal/rest/builder.go` | Identifier quoting |
| `internal/rest/query.go` | Apply quoting to filters |
| `e2e/scripts/setup-test-db.ts` | Add test tables |
| `e2e/tests/rest/select.test.ts` | Enable skipped tests |
