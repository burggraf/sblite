# PostgreSQL Translation Examples

This document shows real-world examples of PostgreSQL queries that would now work in sblite's SQL Browser with the translation layer.

## Before & After Comparison

### Example 1: User Registration Query

**Before (SQLite syntax required):**
```sql
INSERT INTO auth_users (id, email, created_at, is_active)
VALUES (
  lower(hex(randomblob(16))),
  'user@example.com',
  datetime('now'),
  1
);
```

**After (PostgreSQL syntax):**
```sql
INSERT INTO auth_users (id, email, created_at, is_active)
VALUES (
  gen_random_uuid(),
  'user@example.com',
  NOW(),
  TRUE
);
```

### Example 2: Table Creation

**Before (SQLite syntax):**
```sql
CREATE TABLE posts (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT,
  published INTEGER DEFAULT 0,
  metadata TEXT,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT
);
```

**After (PostgreSQL syntax):**
```sql
CREATE TABLE posts (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL,
  title TEXT NOT NULL,
  content TEXT,
  published BOOLEAN DEFAULT FALSE,
  metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ
);
```

### Example 3: Date Filtering

**Before:**
```sql
SELECT * FROM events
WHERE timestamp > datetime('now', '-7 days')
  AND timestamp < datetime('now');
```

**After:**
```sql
SELECT * FROM events
WHERE timestamp > NOW() - INTERVAL '7 days'
  AND timestamp < NOW();
```

### Example 4: String Manipulation

**Before:**
```sql
SELECT
  substr(name, 1, 20) as short_name,
  substr(email, -15) as email_end,
  instr(email, '@') as at_position
FROM users;
```

**After:**
```sql
SELECT
  LEFT(name, 20) as short_name,
  RIGHT(email, 15) as email_end,
  POSITION('@' IN email) as at_position
FROM users;
```

### Example 5: Testing Migrations

**Migration file: migrations/20260119_create_products.sql**

Before translation, developers had to write two versions:

**SQLite version (for sblite testing):**
```sql
CREATE TABLE products (
  id TEXT PRIMARY KEY,
  sku TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  price INTEGER NOT NULL,
  in_stock INTEGER DEFAULT 1,
  tags TEXT,
  created_at TEXT DEFAULT (datetime('now'))
);
```

**PostgreSQL version (for Supabase deployment):**
```sql
CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sku TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  price INTEGER NOT NULL,
  in_stock BOOLEAN DEFAULT TRUE,
  tags JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

**With translation, write once (PostgreSQL syntax):**
```sql
CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sku TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  price INTEGER NOT NULL,
  in_stock BOOLEAN DEFAULT TRUE,
  tags JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

Test it in sblite (auto-translates to SQLite), deploy to Supabase (no changes needed).

## Complex Query Examples

### Example 6: Dashboard Analytics Query

**PostgreSQL syntax (works in both sblite and Supabase):**
```sql
SELECT
  date_trunc('day', created_at) as day,
  COUNT(*) as total_orders,
  SUM(total_price) as revenue,
  COUNT(DISTINCT user_id) as unique_customers
FROM orders
WHERE
  created_at >= NOW() - INTERVAL '30 days'
  AND status = 'completed'
GROUP BY date_trunc('day', created_at)
ORDER BY day DESC;
```

**Note:** The `date_trunc` function would need additional translation rules:
```sql
-- Translated to SQLite:
SELECT
  date(created_at) as day,
  COUNT(*) as total_orders,
  SUM(total_price) as revenue,
  COUNT(DISTINCT user_id) as unique_customers
FROM orders
WHERE
  created_at >= datetime('now', '-30 days')
  AND status = 'completed'
GROUP BY date(created_at)
ORDER BY day DESC;
```

### Example 7: User Search with JSON Metadata

**PostgreSQL syntax:**
```sql
SELECT
  id,
  email,
  LEFT(email, POSITION('@' IN email) - 1) as username,
  metadata->>'role' as role,
  created_at
FROM auth_users
WHERE
  is_active = TRUE
  AND metadata->>'department' = 'engineering'
ORDER BY created_at DESC;
```

**Translated to SQLite:**
```sql
SELECT
  id,
  email,
  SUBSTR(email, 1, INSTR(email, '@') - 1) as username,
  json_extract(metadata, '$.role') as role,
  created_at
FROM auth_users
WHERE
  is_active = 1
  AND json_extract(metadata, '$.department') = 'engineering'
ORDER BY created_at DESC;
```

**Note:** JSON operators (`->`, `->>`) would need additional translation rules.

## Developer Workflows Improved

### Workflow 1: Prototyping with sblite, Deploying to Supabase

**Old workflow:**
1. Write SQLite queries in sblite SQL browser
2. Test locally
3. Manually convert to PostgreSQL syntax
4. Create migration files
5. Deploy to Supabase
6. Debug PostgreSQL incompatibilities

**New workflow:**
1. Write PostgreSQL queries in sblite SQL browser (with translation enabled)
2. Test locally (auto-translates to SQLite)
3. Copy working queries to migration files (no conversion needed)
4. Deploy to Supabase (works immediately)

### Workflow 2: Learning Supabase API

**Old approach:** Beginners had to learn both SQLite and PostgreSQL syntax

**New approach:** Learn PostgreSQL syntax once, use everywhere:
- Test queries in sblite SQL browser
- Copy to application code using `@supabase/supabase-js`
- Deploy to Supabase without syntax changes

## Limitations and Workarounds

### What DOESN'T translate (yet)

1. **Window Functions**
   ```sql
   -- Not supported in basic SQLite
   SELECT ROW_NUMBER() OVER (PARTITION BY category ORDER BY price)
   FROM products;
   ```
   **Workaround:** Use subqueries or application-level pagination

2. **Array Operations**
   ```sql
   -- PostgreSQL arrays don't map cleanly to SQLite
   SELECT ARRAY_AGG(tag) FROM tags WHERE post_id = 123;
   ```
   **Workaround:** Store as JSON array, use `json_group_array()`

3. **Advanced Date Functions**
   ```sql
   -- Complex interval arithmetic
   SELECT * FROM events
   WHERE timestamp > NOW() - INTERVAL '1 month 2 weeks 3 days';
   ```
   **Workaround:** Break into simpler intervals or use date() functions

4. **Full-Text Search (PostgreSQL style)**
   ```sql
   -- PostgreSQL full-text search
   SELECT * FROM posts
   WHERE to_tsvector('english', content) @@ to_tsquery('database & query');
   ```
   **Workaround:** Use SQLite FTS5 syntax (not auto-translated)

## Testing Translation Quality

Use these queries to verify translation is working correctly:

```sql
-- Test 1: Basic type translations
CREATE TABLE test_types (
  id UUID PRIMARY KEY,
  flag BOOLEAN,
  data JSONB,
  ts TIMESTAMPTZ
);

-- Test 2: Function translations
SELECT
  NOW(),
  CURRENT_TIMESTAMP,
  LEFT('hello', 2),
  RIGHT('world', 3);

-- Test 3: Cast removal
SELECT id::text, ts::timestamptz FROM test_types;

-- Test 4: Boolean literals
INSERT INTO test_types (id, flag) VALUES (gen_random_uuid(), TRUE);

-- Test 5: Complex query
UPDATE test_types
SET
  flag = FALSE,
  ts = NOW()
WHERE
  LEFT(id::text, 8) = '12345678'
  AND flag = TRUE;
```

All of these should execute successfully in sblite's SQL Browser with translation enabled.

## Future Enhancements

Translation rules that could be added:

1. **JSON operators:** `->`, `->>`, `@>`, `?`
2. **Array functions:** `array_length`, `array_agg`, `unnest`
3. **String functions:** `concat_ws`, `string_agg`, `split_part`
4. **Date functions:** `date_trunc`, `extract`, `age`
5. **Math functions:** `ceil`, `floor`, `round` (with PostgreSQL precision)
6. **Regex functions:** `regexp_match`, `regexp_replace`

## Conclusion

With PostgreSQL syntax translation, sblite becomes a true PostgreSQL-compatible development environment that just happens to use SQLite under the hood. Developers can:

- Write queries once in PostgreSQL syntax
- Test locally with sblite
- Deploy to Supabase without changes
- Share the same SQL knowledge across tools

This bridges the gap between "Supabase-compatible API" and "Supabase-compatible SQL," making sblite an even more powerful rapid prototyping and development tool.
