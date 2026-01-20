# PostgreSQL Syntax Translation

**Status:** Implemented
**Version:** 1.0
**Last Updated:** 2026-01-19

## Overview

sblite includes a PostgreSQL-to-SQLite syntax translation layer that automatically converts PostgreSQL SQL syntax to SQLite-compatible syntax. This feature enables developers to:

- Write SQL queries using familiar PostgreSQL syntax
- Test PostgreSQL migrations locally before deploying to Supabase
- Seamlessly migrate from sblite to Supabase without rewriting SQL
- Use PostgreSQL-specific functions and data types in sblite

## Quick Start

### Dashboard SQL Browser

1. Navigate to the **SQL Browser** tab in the dashboard
2. Enable the **PostgreSQL Mode** toggle in the toolbar
3. Write your queries using PostgreSQL syntax
4. Click **Run Query** - the query will be automatically translated and executed

![PostgreSQL Mode Toggle](../assets/postgres-mode-toggle.png)

When a query is translated, you'll see a translation indicator with the option to view the translated SQLite query.

### Programmatic Usage

```go
import "github.com/markb/sblite/internal/pgtranslate"

// Simple translation
sqliteQuery := pgtranslate.Translate("SELECT NOW()")
// Result: "SELECT datetime('now')"

// Translation with fallback
translated, wasTranslated := pgtranslate.TranslateWithFallback(query)
if wasTranslated {
    fmt.Println("Translated:", translated)
} else {
    fmt.Println("Query contains unsupported features")
}
```

## Supported Translations

### Date/Time Functions

| PostgreSQL | SQLite | Example |
|------------|--------|---------|
| `NOW()` | `datetime('now')` | `SELECT NOW()` → `SELECT datetime('now')` |
| `CURRENT_TIMESTAMP` | `datetime('now')` | `SELECT CURRENT_TIMESTAMP` → `SELECT datetime('now')` |
| `CURRENT_DATE` | `date('now')` | `SELECT CURRENT_DATE` → `SELECT date('now')` |
| `CURRENT_TIME` | `time('now')` | `SELECT CURRENT_TIME` → `SELECT time('now')` |
| `INTERVAL '7 days'` | `'+7 day'` | `NOW() - INTERVAL '7 days'` → `datetime('now', '+7 day')` |
| `INTERVAL '1 hour'` | `'+1 hour'` | Supports days, hours, minutes, seconds, months, years |

#### EXTRACT Function

| PostgreSQL | SQLite |
|------------|--------|
| `EXTRACT(YEAR FROM date)` | `CAST(strftime('%Y', date) AS INTEGER)` |
| `EXTRACT(MONTH FROM date)` | `CAST(strftime('%m', date) AS INTEGER)` |
| `EXTRACT(DAY FROM date)` | `CAST(strftime('%d', date) AS INTEGER)` |
| `EXTRACT(HOUR FROM date)` | `CAST(strftime('%H', date) AS INTEGER)` |

**Example:**
```sql
-- PostgreSQL
SELECT EXTRACT(YEAR FROM created_at) as year FROM users;

-- Translated to SQLite
SELECT CAST(strftime('%Y', created_at) AS INTEGER) as year FROM users;
```

### String Functions

| PostgreSQL | SQLite | Example |
|------------|--------|---------|
| `LEFT(str, n)` | `SUBSTR(str, 1, n)` | `LEFT(name, 5)` → `SUBSTR(name, 1, 5)` |
| `RIGHT(str, n)` | `SUBSTR(str, -n)` | `RIGHT(name, 5)` → `SUBSTR(name, -5)` |
| `POSITION(sub IN str)` | `INSTR(str, sub)` | `POSITION('x' IN 'text')` → `INSTR('text', 'x')` |
| `ILIKE` | `LIKE` | `name ILIKE '%john%'` → `name LIKE '%john%'` |

**Example:**
```sql
-- PostgreSQL
SELECT LEFT(email, 3), RIGHT(email, 10)
FROM users
WHERE name ILIKE '%smith%';

-- Translated to SQLite
SELECT SUBSTR(email, 1, 3), SUBSTR(email, -10)
FROM users
WHERE name LIKE '%smith%';
```

### Type Casts

PostgreSQL `::` cast notation is automatically removed:

| PostgreSQL | SQLite |
|------------|--------|
| `value::uuid` | `value` |
| `value::timestamptz` | `value` |
| `value::timestamp` | `value` |
| `value::text` | `value` |
| `value::integer` | `value` |

**Example:**
```sql
-- PostgreSQL
SELECT '550e8400-e29b-41d4-a716-446655440000'::uuid as id;

-- Translated to SQLite
SELECT '550e8400-e29b-41d4-a716-446655440000' as id;
```

### Data Types (CREATE TABLE)

| PostgreSQL | SQLite | Notes |
|------------|--------|-------|
| `UUID` | `TEXT` | Stores as text in RFC 4122 format |
| `BOOLEAN` | `INTEGER` | `TRUE` → `1`, `FALSE` → `0` |
| `TIMESTAMPTZ` | `TEXT` | Stores as ISO 8601 string |
| `JSONB` | `TEXT` | Compatible with SQLite JSON1 extension |
| `SERIAL` | `INTEGER` | Use with `PRIMARY KEY` for autoincrement |
| `BIGSERIAL` | `INTEGER` | Same as SERIAL in SQLite |

**Example:**
```sql
-- PostgreSQL
CREATE TABLE users (
  id UUID PRIMARY KEY,
  email TEXT NOT NULL,
  active BOOLEAN DEFAULT TRUE,
  metadata JSONB,
  created_at TIMESTAMPTZ
);

-- Translated to SQLite
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL,
  active INTEGER DEFAULT 1,
  metadata TEXT,
  created_at TEXT
);
```

> **Note:** `gen_random_uuid()` translates to a SELECT subquery which SQLite doesn't support in DEFAULT expressions. Use `gen_random_uuid()` in INSERT statements instead:
> ```sql
> INSERT INTO users (id, email) VALUES (gen_random_uuid(), 'user@example.com');
> ```

### Boolean Literals

| PostgreSQL | SQLite |
|------------|--------|
| `TRUE` | `1` |
| `FALSE` | `0` |

**Example:**
```sql
-- PostgreSQL
SELECT * FROM users WHERE active = TRUE;

-- Translated to SQLite
SELECT * FROM users WHERE active = 1;
```

### JSON Operators

| PostgreSQL | SQLite | Example |
|------------|--------|---------|
| `col->'key'` | `json_extract(col, '$.key')` | `data->'name'` → `json_extract(data, '$.name')` |
| `col->>'key'` | `json_extract(col, '$.key')` | `data->>'name'` → `json_extract(data, '$.name')` |

**Example:**
```sql
-- PostgreSQL
SELECT
  metadata->'preferences' as prefs,
  metadata->>'email' as email
FROM users;

-- Translated to SQLite
SELECT
  json_extract(metadata, '$.preferences') as prefs,
  json_extract(metadata, '$.email') as email
FROM users;
```

### Aggregate Functions

| PostgreSQL | SQLite |
|------------|--------|
| `GREATEST(...)` | `MAX(...)` |
| `LEAST(...)` | `MIN(...)` |
| `STRING_AGG(col, sep)` | `GROUP_CONCAT(col, sep)` |

**Example:**
```sql
-- PostgreSQL
SELECT GREATEST(score1, score2, score3) as max_score FROM games;

-- Translated to SQLite
SELECT MAX(score1, score2, score3) as max_score FROM games;
```

### Special Functions

#### gen_random_uuid()

PostgreSQL's `gen_random_uuid()` is translated to a SQLite expression that generates RFC 4122 compliant UUID v4 values:

```sql
-- PostgreSQL
INSERT INTO users (id) VALUES (gen_random_uuid());

-- Translated to SQLite (generates proper UUID v4 format)
INSERT INTO users (id) VALUES ((SELECT lower(
  substr(h, 1, 8) || '-' ||
  substr(h, 9, 4) || '-' ||
  '4' || substr(h, 14, 3) || '-' ||
  substr('89ab', (abs(random()) % 4) + 1, 1) || substr(h, 18, 3) || '-' ||
  substr(h, 21, 12)
) FROM (SELECT hex(randomblob(16)) as h)));
```

This generates UUIDs in the format: `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx` where:
- `4` is the UUID version
- `y` is one of `8`, `9`, `a`, or `b` (variant bits)

### INSERT Conflict Handling

| PostgreSQL | SQLite |
|------------|--------|
| `ON CONFLICT DO NOTHING` | `OR IGNORE` |

**Example:**
```sql
-- PostgreSQL
INSERT INTO users (id, email) VALUES (1, 'test@example.com')
ON CONFLICT DO NOTHING;

-- Translated to SQLite
INSERT OR IGNORE INTO users (id, email) VALUES (1, 'test@example.com');
```

## Unsupported Features

The following PostgreSQL features cannot be reliably translated and will **not** be translated (original query returned):

- **Window Functions** - `OVER()`, `PARTITION BY`
- **Array Literals** - `ARRAY[1,2,3]`
- **Array Functions** - `ARRAY_AGG()`, `UNNEST()`
- **Lateral Joins** - `LATERAL`
- **Row Locking** - `FOR UPDATE`, `FOR SHARE`
- **Advanced CTEs** - Some complex Common Table Expressions
- **ON CONFLICT DO UPDATE** - Requires complex rewriting

For these features, you'll need to:
1. Disable PostgreSQL mode and write SQLite syntax manually
2. Wait for future enhancements to the translator
3. Use workarounds (e.g., rewrite window functions as subqueries)

## Known Limitations

The following are known limitations of the regex-based translation approach:

### DEFAULT Expressions with gen_random_uuid()

SQLite doesn't support SELECT subqueries in DEFAULT expressions. While `gen_random_uuid()` translates correctly for use in INSERT statements, it cannot be used in CREATE TABLE DEFAULT clauses:

```sql
-- This PostgreSQL syntax:
CREATE TABLE users (id UUID PRIMARY KEY DEFAULT gen_random_uuid());

-- Translates to a SELECT subquery that SQLite rejects in DEFAULT:
-- ERROR: near "SELECT": syntax error

-- Workaround: Use gen_random_uuid() in INSERT statements:
INSERT INTO users (id, name) VALUES (gen_random_uuid(), 'Alice');
```

### String Literals May Be Affected

The regex-based translation doesn't distinguish between SQL keywords and string contents. Keywords inside string literals may be incorrectly translated:

```sql
-- This query:
SELECT 'The value is TRUE' as message;

-- May become:
SELECT 'The value is 1' as message;
```

This is a known limitation of the regex-based approach. A future AST-based translation would avoid this issue.

### JSON Operator Semantic Differences

PostgreSQL's `->` returns JSON and `->>` returns text. SQLite's `json_extract()` always returns the value type. Both operators are translated to `json_extract()`, which works for most use cases but may differ in edge cases involving JSON type handling.

## Dashboard Integration

### PostgreSQL Mode Toggle

The SQL Browser includes a **PostgreSQL Mode** toggle in the toolbar:

1. **Enabled** (checked): All queries are translated from PostgreSQL to SQLite before execution
2. **Disabled** (unchecked): Queries are executed as-is in SQLite syntax

The toggle state is **persisted** in localStorage, so your preference is remembered across sessions.

### Translation Indicator

When a query is successfully translated, the results panel displays:

```
✓ Translated from PostgreSQL syntax
▼ View translated query
```

Clicking "View translated query" expands a section showing the actual SQLite query that was executed.

### Error Handling

If a query contains unsupported PostgreSQL features, it will **not** be translated and will be executed as-is. This may result in SQLite errors, which will be displayed in the error panel.

## API Usage

### Dashboard SQL API

The `/_/api/sql` endpoint accepts a `postgres_mode` parameter:

```bash
curl -X POST http://localhost:8080/_/api/sql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT NOW() as current_time",
    "postgres_mode": true
  }'
```

Response includes translation information:

```json
{
  "columns": ["current_time"],
  "rows": [["2026-01-19 14:30:45"]],
  "row_count": 1,
  "execution_time_ms": 2,
  "type": "SELECT",
  "translated_query": "SELECT datetime('now') as current_time",
  "was_translated": true
}
```

## Use Cases

### 1. Testing PostgreSQL Migrations Locally

Before deploying migrations to Supabase, test them locally with sblite:

```sql
-- migration.sql (PostgreSQL syntax)
CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  price NUMERIC NOT NULL,
  active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_products_active ON products(active) WHERE active = TRUE;
```

Run this in sblite with PostgreSQL mode enabled - it will translate and execute correctly. When ready, deploy the same SQL to Supabase without modifications.

### 2. Writing SQL Functions for RPC

When implementing PostgreSQL functions that will be transpiled or migrated:

```sql
-- Write in PostgreSQL syntax
CREATE FUNCTION get_active_users()
RETURNS TABLE(id uuid, email text, created_at timestamptz)
LANGUAGE sql AS $$
  SELECT id, email, created_at
  FROM users
  WHERE active = TRUE
  AND created_at > NOW() - INTERVAL '30 days'
$$;
```

With PostgreSQL mode enabled, the function body will be automatically translated when stored.

### 3. Learning PostgreSQL While Using sblite

Developers new to PostgreSQL can learn PostgreSQL syntax while using sblite, making the transition to full Supabase smoother.

## Implementation Details

### Translation Pipeline

1. **Input**: Original PostgreSQL query + `postgres_mode` flag
2. **Rule Application**: Series of regex-based translation rules applied sequentially
3. **Validation**: Check for unsupported features
4. **Fallback**: If unsupported features detected, return original query (no translation)
5. **Execution**: Translated (or original) query executed against SQLite
6. **Response**: Results + translation metadata

### Performance

- **Overhead**: Minimal (<1ms for typical queries)
- **Caching**: Future enhancement - cache translated queries
- **Optimization**: Translation happens once before execution, not per row

### Architecture

```
internal/pgtranslate/
├── translator.go      # Core translation engine
├── translator_test.go # Comprehensive unit tests
└── uuid_test.go       # UUID generation tests
```

**Key Components:**

- `Translator`: Main translation engine with rule-based system
- `Rule` interface: Extensible rule system for adding new translations
- `RegexRule`: Regex-based translation rules
- `FunctionRule`: Function name translation rules
- `IsTranslatable()`: Checks for unsupported features
- `TranslateWithFallback()`: Safe translation with fallback

### Extending the Translator

To add new translation rules, edit `internal/pgtranslate/translator.go`:

```go
// Add to defaultRules() function
&RegexRule{
    pattern:     regexp.MustCompile(`(?i)YOUR_PATTERN`),
    replacement: "YOUR_REPLACEMENT",
},
```

Example - adding `CONCAT_WS` support:

```go
&RegexRule{
    pattern:     regexp.MustCompile(`(?i)CONCAT_WS\s*\(\s*'([^']+)'\s*,\s*(.+?)\s*\)`),
    replacement: "($2)", // SQLite doesn't have CONCAT_WS, use manual concatenation
},
```

## Troubleshooting

### Query Not Translating

**Symptom**: PostgreSQL mode is enabled but query fails with "no such function"

**Cause**: Query contains unsupported features or syntax not yet implemented

**Solution**:
1. Check the list of unsupported features above
2. Disable PostgreSQL mode and rewrite using SQLite syntax
3. File an issue requesting support for the feature

### Incorrect Translation

**Symptom**: Query translates but produces unexpected results

**Cause**: Edge case in translation rules or semantic difference between PostgreSQL and SQLite

**Solution**:
1. Click "View translated query" to see the SQLite translation
2. Verify the translated query produces correct results
3. If incorrect, file a bug report with the original and translated queries

### Performance Issues

**Symptom**: Queries with PostgreSQL mode are noticeably slower

**Cause**: Complex regex patterns or very large queries

**Solution**:
1. Translation overhead should be <1ms - profile to confirm
2. For performance-critical queries, consider writing directly in SQLite syntax
3. Future releases will include query caching

## Migration Strategy

### From sblite (PostgreSQL Mode) to Supabase

1. **During Development**: Use PostgreSQL mode for all queries and migrations
2. **Before Migration**: Test all queries work correctly in sblite
3. **Export Schema**: Use `sblite migrate export` to generate PostgreSQL DDL
4. **Deploy to Supabase**: Run the exported schema directly on Supabase
5. **Verify**: Test that your app works identically with Supabase backend

### From sblite (SQLite Mode) to Supabase

If you've been using SQLite syntax without PostgreSQL mode:

1. **Enable PostgreSQL Mode**: Turn on the toggle
2. **Rewrite Queries**: Update queries to use PostgreSQL syntax
3. **Test Thoroughly**: Verify behavior hasn't changed
4. **Follow Migration Steps Above**

## Best Practices

1. **Enable PostgreSQL Mode Early**: Use it from the start of your project
2. **Test Both Ways**: Periodically test queries with mode on and off to catch issues
3. **Use Type System**: Leverage sblite's type system (`_columns` metadata) alongside translation
4. **Document Edge Cases**: If you find limitations, document workarounds for your team
5. **Keep Migrations Clean**: Write migrations in pure PostgreSQL syntax
6. **Test Before Deploy**: Always test exported schema in a Supabase dev environment first

## Future Enhancements

Planned improvements to the translation layer:

- **AST-Based Translation**: Replace regex with proper SQL parser for more robust translation
- **Query Caching**: Cache translated queries for better performance
- **More Functions**: Add support for additional PostgreSQL functions
- **Better Error Messages**: Specific errors for unsupported features
- **Translation Statistics**: Dashboard showing translation success rate
- **Custom Rules**: Allow users to define custom translation rules

## Contributing

To contribute new translation rules or improvements:

1. Add translation rule to `internal/pgtranslate/translator.go`
2. Add unit tests to `internal/pgtranslate/translator_test.go`
3. Add E2E tests to `e2e/tests/dashboard/postgres-translation.test.ts`
4. Update this documentation with the new feature
5. Submit a pull request

## Related Documentation

- [PostgreSQL Functions (RPC) Design](./plans/2025-01-19-postgresql-functions-design.md)
- [Type System](./plans/2026-01-17-type-system-design.md)
- [Migration System](./plans/2026-01-17-phase1-migration-system.md)
- [SQL Browser](./plans/2026-01-18-sql-browser-design.md)

## FAQ

**Q: Does PostgreSQL mode make sblite fully PostgreSQL compatible?**
A: No. It provides translation for common PostgreSQL syntax, but sblite is still powered by SQLite. Some PostgreSQL features (like window functions, arrays) are not supported.

**Q: Will translated queries perform the same as in PostgreSQL?**
A: Usually yes, but SQLite and PostgreSQL have different query planners and optimizations. Performance characteristics may differ.

**Q: Can I use PostgreSQL mode with RLS policies?**
A: Yes! RLS policies written in PostgreSQL syntax will be translated just like queries.

**Q: Does translation work with stored functions?**
A: Yes, when you create SQL functions via the RPC system (Phase C), function bodies written in PostgreSQL syntax will be translated.

**Q: What happens if translation fails?**
A: The original query is executed as-is. This will likely result in a SQLite error, which will be displayed to you.

**Q: Is there a performance penalty?**
A: Minimal (<1ms). The translation happens once before execution, not per row.

**Q: Can I disable translation for specific queries?**
A: Yes - either toggle PostgreSQL mode off, or omit the `postgres_mode` parameter when calling the API directly.

---

For questions or issues, please file an issue on GitHub: https://github.com/burggraf/sblite/issues
