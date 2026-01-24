# Comprehensive PostgreSQL Compatibility Plan

## Overview

This plan documents all PostgreSQL features that should be translated to SQLite equivalents for maximum compatibility with PostgreSQL client tools, migrations, and applications.

**Reference:** [pgsqlite](https://github.com/erans/pgsqlite) - Rust-based PostgreSQL to SQLite translator

## Current State (Already Implemented)

sblite's `internal/pgtranslate/` already supports:

| Category | Features |
|----------|----------|
| **Type Casts** | `::uuid`, `::text`, `::integer`, `::timestamp`, `::timestamptz` |
| **Types** | UUID, BOOLEAN, TIMESTAMPTZ, JSONB, SERIAL, BIGSERIAL, BYTEA |
| **Date/Time** | `NOW()`, `CURRENT_TIMESTAMP`, `CURRENT_DATE`, `CURRENT_TIME`, `INTERVAL`, `EXTRACT()`, `AGE()` |
| **String** | `LENGTH`, `LOWER`, `UPPER`, `TRIM`, `LTRIM`, `RTRIM`, `LEFT`, `RIGHT`, `POSITION`, `CONCAT`, `CONCAT_WS`, `STRING_AGG` |
| **JSON** | `->`, `->>` operators, `json_extract()` |
| **Arrays** | `ARRAY[...]` literals, array subscripts `arr[n]`, `= ANY()`, `= ALL()` |
| **Window** | `OVER()`, `PARTITION BY`, `ROWS`, `RANGE`, `GROUPS`, frame specifications |
| **Regex** | `~`, `~*`, `!~`, `!~*` operators |
| **Comparison** | `GREATEST`, `LEAST`, `BETWEEN`, `IN`, `IS NULL/TRUE/FALSE` |
| **Queries** | CTEs (`WITH`), `RETURNING`, `ON CONFLICT DO NOTHING`, `ILIKE` |

---

## Phase 1: String Functions (Priority: High)

Missing string functions commonly used in PostgreSQL applications.

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `SPLIT_PART(str, delim, n)` | Custom using `INSTR` + `SUBSTR` | Returns nth part of string |
| `TRANSLATE(str, from, to)` | Custom function | Character-by-character replacement |
| `ASCII(char)` | `UNICODE(char)` | Get character code |
| `CHR(code)` | `CHAR(code)` | Character from code |
| `REPEAT(str, n)` | Custom with recursive CTE or custom function | Repeat string n times |
| `REVERSE(str)` | Custom function | Reverse string |
| `LPAD(str, len, fill)` | Custom using `SUBSTR` + `||` | Left-pad string |
| `RPAD(str, len, fill)` | Custom using `SUBSTR` + `||` | Right-pad string |
| `INITCAP(str)` | Custom function | Title case |
| `QUOTE_LITERAL(str)` | `'''' \|\| REPLACE(str, '''', '''''') \|\| ''''` | SQL-escape string |
| `QUOTE_IDENT(str)` | `'"' \|\| str \|\| '"'` | Quote identifier |
| `FORMAT(fmt, ...)` | Custom function | Printf-style formatting |
| `REGEXP_REPLACE(str, pat, rep)` | Custom function | Regex replacement |
| `REGEXP_MATCHES(str, pat)` | Custom function | Extract regex matches |
| `SUBSTRING(str FROM pat)` | Custom function | Regex substring |

### Implementation

**File:** `internal/pgtranslate/funcs.go`

```go
// Add to pgToSQLite map:
m.pgToSQLite["SPLIT_PART"] = transformSplitPart
m.pgToSQLite["TRANSLATE"] = transformTranslate
m.pgToSQLite["ASCII"] = func(call *FunctionCall, _ *Generator) (string, bool) {
    return transformFuncRename(call, "UNICODE")
}
m.pgToSQLite["CHR"] = func(call *FunctionCall, _ *Generator) (string, bool) {
    return transformFuncRename(call, "CHAR")
}
m.pgToSQLite["REPEAT"] = transformRepeat
m.pgToSQLite["REVERSE"] = transformReverse
m.pgToSQLite["LPAD"] = transformLpad
m.pgToSQLite["RPAD"] = transformRpad
```

**File:** `internal/db/db.go` - Register custom SQLite functions:

```go
// Register string helper functions
sqlite.RegisterScalarFunction("translate", 3, translateFunc)
sqlite.RegisterScalarFunction("repeat", 2, repeatFunc)
sqlite.RegisterScalarFunction("reverse", 1, reverseFunc)
sqlite.RegisterScalarFunction("initcap", 1, initcapFunc)
sqlite.RegisterScalarFunction("format", -1, formatFunc) // variadic
sqlite.RegisterScalarFunction("regexp_replace", 3, regexpReplaceFunc)
```

### Verification

```sql
SELECT SPLIT_PART('a,b,c', ',', 2);  -- 'b'
SELECT REPEAT('ab', 3);              -- 'ababab'
SELECT LPAD('42', 5, '0');           -- '00042'
SELECT REVERSE('hello');             -- 'olleh'
```

---

## Phase 2: Math Functions (Priority: Medium)

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `TRUNC(n)` / `TRUNC(n, d)` | `CAST(n AS INTEGER)` / custom | Truncate decimal |
| `CEIL(n)` / `CEILING(n)` | Custom function | Ceiling |
| `SIGN(n)` | Custom function | Returns -1, 0, or 1 |
| `POWER(x, y)` | Custom function | x^y |
| `SQRT(n)` | Custom function | Square root |
| `EXP(n)` | Custom function | e^n |
| `LN(n)` | Custom function | Natural log |
| `LOG(base, n)` / `LOG10(n)` | Custom function | Logarithm |
| `MOD(x, y)` | `x % y` | Modulo |
| `DIV(x, y)` | `x / y` | Integer division |
| `RANDOM()` | `(ABS(RANDOM()) / 9223372036854775807.0)` | 0.0-1.0 range |
| `SETSEED(n)` | No-op | Not supported |
| `PI()` | `3.141592653589793` | Constant |
| `DEGREES(radians)` | `radians * 180.0 / PI()` | Convert |
| `RADIANS(degrees)` | `degrees * PI() / 180.0` | Convert |
| Trig functions | Custom functions | `SIN`, `COS`, `TAN`, `ASIN`, `ACOS`, `ATAN`, `ATAN2` |

### Implementation

**File:** `internal/db/db.go`

```go
// Math functions using Go's math package
sqlite.RegisterScalarFunction("ceil", 1, ceilFunc)
sqlite.RegisterScalarFunction("ceiling", 1, ceilFunc)
sqlite.RegisterScalarFunction("floor", 1, floorFunc)
sqlite.RegisterScalarFunction("sign", 1, signFunc)
sqlite.RegisterScalarFunction("power", 2, powerFunc)
sqlite.RegisterScalarFunction("sqrt", 1, sqrtFunc)
sqlite.RegisterScalarFunction("exp", 1, expFunc)
sqlite.RegisterScalarFunction("ln", 1, lnFunc)
sqlite.RegisterScalarFunction("log", 2, logFunc)
sqlite.RegisterScalarFunction("log10", 1, log10Func)
sqlite.RegisterScalarFunction("pi", 0, piFunc)
sqlite.RegisterScalarFunction("sin", 1, sinFunc)
sqlite.RegisterScalarFunction("cos", 1, cosFunc)
sqlite.RegisterScalarFunction("tan", 1, tanFunc)
sqlite.RegisterScalarFunction("asin", 1, asinFunc)
sqlite.RegisterScalarFunction("acos", 1, acosFunc)
sqlite.RegisterScalarFunction("atan", 1, atanFunc)
sqlite.RegisterScalarFunction("atan2", 2, atan2Func)
sqlite.RegisterScalarFunction("degrees", 1, degreesFunc)
sqlite.RegisterScalarFunction("radians", 1, radiansFunc)
```

### Verification

```sql
SELECT CEIL(4.2);         -- 5
SELECT SQRT(16);          -- 4.0
SELECT POWER(2, 10);      -- 1024
SELECT LOG(10, 100);      -- 2.0
SELECT SIN(PI() / 2);     -- 1.0
```

---

## Phase 3: JSON Functions & Operators (Priority: High)

### Operators to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `@>` (contains) | `json_contains(a, b)` | Custom function |
| `<@` (contained by) | `json_contains(b, a)` | Reverse of @> |
| `#>` (path extract) | `json_extract(doc, path)` | Already have `->` |
| `#>>` (path extract text) | `json_extract(doc, path)` | Return as text |
| `?` (key exists) | `json_type(doc, '$.key') IS NOT NULL` | Check key |
| `?|` (any key exists) | Custom function | Check any of keys |
| `?&` (all keys exist) | Custom function | Check all keys |
| `||` (concatenate) | `json_patch(a, b)` | Merge JSON objects |
| `-` (delete key) | `json_remove(doc, path)` | Remove key |
| `- int` (delete index) | `json_remove(doc, '$[n]')` | Remove array element |
| `#-` (delete path) | `json_remove(doc, path)` | Remove at path |

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `JSON_AGG(expr)` | Custom aggregate | Collect into JSON array |
| `JSONB_AGG(expr)` | Custom aggregate | Same as JSON_AGG |
| `JSON_OBJECT_AGG(k, v)` | Custom aggregate | Collect into JSON object |
| `JSONB_OBJECT_AGG(k, v)` | Custom aggregate | Same |
| `ROW_TO_JSON(record)` | Custom function | Convert row to JSON |
| `JSON_BUILD_OBJECT(k1,v1,...)` | `json_object(k1,v1,...)` | Direct mapping |
| `JSON_BUILD_ARRAY(...)` | `json_array(...)` | Direct mapping |
| `JSON_ARRAY_LENGTH(arr)` | `json_array_length(arr)` | Direct mapping |
| `JSONB_PRETTY(doc)` | Custom function | Format JSON |
| `JSONB_SET(doc, path, val)` | `json_set(doc, path, val)` | Set value at path |
| `JSONB_INSERT(doc, path, val)` | `json_insert(doc, path, val)` | Insert at path |
| `JSONB_TYPEOF(doc)` | `json_type(doc)` | Get JSON type |
| `JSON_EACH(doc)` | `json_each(doc)` | Table function |
| `JSON_EACH_TEXT(doc)` | `json_each(doc)` | Same, text values |
| `JSON_ARRAY_ELEMENTS(arr)` | `json_each(arr)` | Array to rows |
| `JSONB_ARRAY_ELEMENTS_TEXT(arr)` | `json_each(arr)` | Array to text rows |
| `JSON_OBJECT_KEYS(doc)` | Custom function | Get object keys |
| `JSONB_STRIP_NULLS(doc)` | Custom function | Remove null values |
| `JSON_POPULATE_RECORD(...)` | Custom function | JSON to record |
| `JSON_TO_RECORD(doc)` | Not directly supported | Complex |

### Implementation

**File:** `internal/pgtranslate/parser.go` - Add parsing for JSON operators:

```go
// In lexer, recognize:
// @>  TokenJsonContains
// <@  TokenJsonContainedBy
// ?   TokenJsonExists
// ?|  TokenJsonExistsAny
// ?&  TokenJsonExistsAll
// #>  TokenJsonPath
// #>> TokenJsonPathText
// #-  TokenJsonDeletePath
```

**File:** `internal/pgtranslate/generator.go`:

```go
func (g *Generator) generateJsonContains(left, right string) string {
    // Custom json_contains function
    return fmt.Sprintf("json_contains(%s, %s)", left, right)
}
```

**File:** `internal/db/db.go`:

```go
// Register JSON helper functions
sqlite.RegisterScalarFunction("json_contains", 2, jsonContainsFunc)
sqlite.RegisterScalarFunction("json_exists", 2, jsonExistsFunc)
sqlite.RegisterScalarFunction("jsonb_pretty", 1, jsonPrettyFunc)
sqlite.RegisterScalarFunction("json_object_keys", 1, jsonObjectKeysFunc)
sqlite.RegisterScalarFunction("jsonb_strip_nulls", 1, jsonStripNullsFunc)

// Aggregate functions
sqlite.RegisterAggregateFunction("json_agg", jsonAggStep, jsonAggFinal)
sqlite.RegisterAggregateFunction("json_object_agg", jsonObjectAggStep, jsonObjectAggFinal)
```

### Verification

```sql
SELECT '{"a":1}'::jsonb @> '{"a":1}'::jsonb;  -- true
SELECT '{"a":1, "b":2}'::jsonb ? 'a';         -- true
SELECT JSON_AGG(name) FROM users;             -- ["Alice", "Bob"]
SELECT JSON_BUILD_OBJECT('id', 1, 'name', 'test');
```

---

## Phase 4: Array Functions (Priority: Medium)

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `UNNEST(array)` | `json_each(arr)` with value extraction | Table function |
| `ARRAY_AGG(expr)` | `JSON_GROUP_ARRAY(expr)` | Aggregate to array |
| `ARRAY_LENGTH(arr, dim)` | `json_array_length(arr)` | Get length |
| `ARRAY_APPEND(arr, elem)` | Custom | Add to end |
| `ARRAY_PREPEND(elem, arr)` | Custom | Add to start |
| `ARRAY_CAT(arr1, arr2)` | Custom | Concatenate arrays |
| `ARRAY_REMOVE(arr, elem)` | Custom | Remove element |
| `ARRAY_REPLACE(arr, old, new)` | Custom | Replace element |
| `ARRAY_POSITION(arr, elem)` | Custom | Find index |
| `ARRAY_POSITIONS(arr, elem)` | Custom | Find all indexes |
| `ARRAY_TO_STRING(arr, delim)` | Custom | Join array |
| `STRING_TO_ARRAY(str, delim)` | Custom | Split string |
| `CARDINALITY(arr)` | `json_array_length(arr)` | Array length |
| `ARRAY_DIMS(arr)` | Custom | Dimension info |
| `ARRAY_UPPER(arr, dim)` | Custom | Upper bound |
| `ARRAY_LOWER(arr, dim)` | Custom | Lower bound |

### Implementation

**File:** `internal/pgtranslate/funcs.go`:

```go
m.pgToSQLite["UNNEST"] = transformUnnest
m.pgToSQLite["ARRAY_AGG"] = func(call *FunctionCall, _ *Generator) (string, bool) {
    return transformFuncRename(call, "JSON_GROUP_ARRAY")
}
m.pgToSQLite["ARRAY_LENGTH"] = transformArrayLength
m.pgToSQLite["ARRAY_APPEND"] = transformArrayAppend
m.pgToSQLite["ARRAY_TO_STRING"] = transformArrayToString
m.pgToSQLite["STRING_TO_ARRAY"] = transformStringToArray
m.pgToSQLite["CARDINALITY"] = func(call *FunctionCall, _ *Generator) (string, bool) {
    return transformFuncRename(call, "json_array_length")
}
```

**File:** `internal/db/db.go`:

```go
sqlite.RegisterScalarFunction("array_append", 2, arrayAppendFunc)
sqlite.RegisterScalarFunction("array_prepend", 2, arrayPrependFunc)
sqlite.RegisterScalarFunction("array_cat", 2, arrayCatFunc)
sqlite.RegisterScalarFunction("array_remove", 2, arrayRemoveFunc)
sqlite.RegisterScalarFunction("array_position", 2, arrayPositionFunc)
sqlite.RegisterScalarFunction("array_to_string", 2, arrayToStringFunc)
sqlite.RegisterScalarFunction("string_to_array", 2, stringToArrayFunc)
```

### UNNEST Special Handling

`UNNEST` is complex because it's a table-returning function. Translation:

```sql
-- PostgreSQL:
SELECT * FROM UNNEST(ARRAY[1,2,3]) AS x;

-- SQLite:
SELECT value AS x FROM json_each('[1,2,3]');

-- PostgreSQL with ordinality:
SELECT * FROM UNNEST(ARRAY['a','b']) WITH ORDINALITY AS t(val, idx);

-- SQLite:
SELECT value AS val, key + 1 AS idx FROM json_each('["a","b"]');
```

### Verification

```sql
SELECT UNNEST(ARRAY[1, 2, 3]);                    -- rows: 1, 2, 3
SELECT ARRAY_AGG(name) FROM users;                -- ["Alice", "Bob"]
SELECT ARRAY_TO_STRING(ARRAY['a','b','c'], ',');  -- "a,b,c"
SELECT STRING_TO_ARRAY('a,b,c', ',');             -- ["a","b","c"]
```

---

## Phase 5: Full-Text Search (Priority: Medium)

sblite already has FTS5 integration. This phase adds PostgreSQL tsvector/tsquery syntax translation.

### Types and Operators

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `tsvector` | TEXT (FTS5 content) | Type mapping |
| `tsquery` | TEXT (FTS5 query) | Type mapping |
| `@@` (match) | `fts_table MATCH query` | FTS match |
| `||` (tsvector concat) | Concatenate text | Simple |
| `&&` (tsquery AND) | `term1 AND term2` | FTS5 syntax |
| `||` (tsquery OR) | `term1 OR term2` | FTS5 syntax |
| `!!` (tsquery NOT) | `NOT term` | FTS5 syntax |
| `@>` (contains) | Custom | tsvector contains |
| `<->` (followed by) | `term1 term2` | Phrase query |

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `TO_TSVECTOR(config, text)` | `fts_content(text)` | Create tsvector |
| `TO_TSVECTOR(text)` | `fts_content(text)` | Default config |
| `TO_TSQUERY(config, query)` | FTS5 query conversion | Convert syntax |
| `TO_TSQUERY(query)` | FTS5 query conversion | Default config |
| `PLAINTO_TSQUERY(text)` | FTS5 simple query | Plain text search |
| `PHRASETO_TSQUERY(text)` | FTS5 phrase query | Phrase search |
| `WEBSEARCH_TO_TSQUERY(text)` | FTS5 websearch | Web-style search |
| `TS_RANK(vector, query)` | `bm25(fts_table)` | Relevance score |
| `TS_RANK_CD(vector, query)` | `bm25(fts_table)` | Cover density rank |
| `TS_HEADLINE(doc, query)` | `snippet(fts_table)` | Highlight matches |
| `SETWEIGHT(vector, weight)` | Custom (store weight) | Weight terms |
| `STRIP(vector)` | Remove positions | Strip positions |
| `LENGTH(tsvector)` | Custom | Count lexemes |
| `NUMNODE(tsquery)` | Custom | Count query nodes |

### Implementation

The key insight is that PostgreSQL FTS and SQLite FTS5 have different models:
- PostgreSQL: tsvector stored in column, tsquery at query time
- SQLite: FTS5 virtual table with separate storage

Translation approach:
1. Map `TO_TSVECTOR()` to text extraction for FTS indexing
2. Map `@@` queries to FTS5 MATCH
3. Convert tsquery syntax to FTS5 syntax

**File:** `internal/pgtranslate/parser.go`:

```go
// Add TokenTsMatch for @@
// Parse TO_TSVECTOR, TO_TSQUERY as function calls
```

**File:** `internal/pgtranslate/generator.go`:

```go
func (g *Generator) generateTsMatch(col, query string) string {
    // Transform: col @@ TO_TSQUERY('word')
    // To: col_fts MATCH 'word'
    return fmt.Sprintf("%s_fts MATCH %s", col, g.convertTsQuery(query))
}

func (g *Generator) convertTsQuery(query string) string {
    // Convert PostgreSQL tsquery syntax to FTS5
    // 'cat & dog' -> 'cat AND dog'
    // 'cat | dog' -> 'cat OR dog'
    // '!cat' -> 'NOT cat'
    // 'cat <-> dog' -> '"cat dog"'
}
```

### Verification

```sql
-- PostgreSQL:
SELECT * FROM documents WHERE content @@ TO_TSQUERY('cat & dog');

-- Translated to:
SELECT * FROM documents WHERE documents_fts MATCH 'cat AND dog';
```

---

## Phase 6: Date/Time Functions (Priority: Medium)

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `DATE_TRUNC(field, ts)` | Custom using strftime | Truncate to precision |
| `DATE_PART(field, ts)` | `CAST(strftime(...) AS INTEGER)` | Extract field |
| `MAKE_DATE(y, m, d)` | `printf('%04d-%02d-%02d', y, m, d)` | Construct date |
| `MAKE_TIME(h, m, s)` | `printf('%02d:%02d:%02f', h, m, s)` | Construct time |
| `MAKE_TIMESTAMP(...)` | Custom | Construct timestamp |
| `MAKE_TIMESTAMPTZ(...)` | Custom | With timezone |
| `MAKE_INTERVAL(...)` | Custom | Construct interval |
| `TO_TIMESTAMP(epoch)` | `datetime(epoch, 'unixepoch')` | Epoch to timestamp |
| `TO_TIMESTAMP(str, fmt)` | Custom parsing | Parse formatted string |
| `TO_DATE(str, fmt)` | Custom parsing | Parse formatted date |
| `TO_CHAR(ts, fmt)` | Custom using strftime | Format to string |
| `ISFINITE(ts)` | `ts IS NOT NULL` | Check finite |
| `JUSTIFY_DAYS(interval)` | Custom | Normalize days |
| `JUSTIFY_HOURS(interval)` | Custom | Normalize hours |
| `JUSTIFY_INTERVAL(interval)` | Custom | Normalize all |
| `LOCALTIME` | `time('now', 'localtime')` | Current local time |
| `LOCALTIMESTAMP` | `datetime('now', 'localtime')` | Current local timestamp |
| `CLOCK_TIMESTAMP()` | `datetime('now')` | Current timestamp |
| `STATEMENT_TIMESTAMP()` | `datetime('now')` | Statement start |
| `TRANSACTION_TIMESTAMP()` | `datetime('now')` | Transaction start |
| `TIMEOFDAY()` | Custom | Formatted time |

### Format String Translation (TO_CHAR / TO_TIMESTAMP)

PostgreSQL and SQLite use different format codes:

| PostgreSQL | SQLite strftime | Description |
|------------|-----------------|-------------|
| `YYYY` | `%Y` | 4-digit year |
| `YY` | `%y` | 2-digit year |
| `MM` | `%m` | Month (01-12) |
| `DD` | `%d` | Day (01-31) |
| `HH24` | `%H` | Hour (00-23) |
| `HH12` | `%I` | Hour (01-12) |
| `MI` | `%M` | Minute (00-59) |
| `SS` | `%S` | Second (00-59) |
| `MS` | N/A | Milliseconds |
| `US` | N/A | Microseconds |
| `AM`/`PM` | `%p` | AM/PM indicator |
| `Day` | `%A` | Full day name |
| `Mon` | `%b` | Abbreviated month |
| `TZ` | N/A | Timezone name |

### Implementation

**File:** `internal/pgtranslate/funcs.go`:

```go
m.pgToSQLite["DATE_TRUNC"] = transformDateTrunc
m.pgToSQLite["DATE_PART"] = transformDatePart
m.pgToSQLite["TO_TIMESTAMP"] = transformToTimestamp
m.pgToSQLite["TO_DATE"] = transformToDate
m.pgToSQLite["TO_CHAR"] = transformToChar
m.pgToSQLite["MAKE_DATE"] = transformMakeDate
m.pgToSQLite["LOCALTIME"] = func(call *FunctionCall, _ *Generator) (string, bool) {
    return "time('now', 'localtime')", true
}
m.pgToSQLite["LOCALTIMESTAMP"] = func(call *FunctionCall, _ *Generator) (string, bool) {
    return "datetime('now', 'localtime')", true
}
```

### Verification

```sql
SELECT DATE_TRUNC('month', '2024-03-15 12:30:00');  -- '2024-03-01 00:00:00'
SELECT DATE_PART('year', '2024-03-15');             -- 2024
SELECT TO_CHAR(NOW(), 'YYYY-MM-DD');                -- '2024-03-15'
SELECT TO_TIMESTAMP('2024-03-15', 'YYYY-MM-DD');
```

---

## Phase 7: System & Catalog Functions (Priority: Low)

### Functions to Implement

| PostgreSQL | SQLite Translation | Notes |
|------------|-------------------|-------|
| `VERSION()` | `'PostgreSQL 15.0 (sblite)'` | Fixed string |
| `CURRENT_DATABASE()` | Database filename | From connection |
| `CURRENT_SCHEMA()` | `'main'` | SQLite default |
| `CURRENT_USER` | `'sblite'` | Fixed |
| `SESSION_USER` | `'sblite'` | Fixed |
| `USER` | `'sblite'` | Fixed |
| `INET_CLIENT_ADDR()` | Connection IP | From context |
| `INET_CLIENT_PORT()` | Connection port | From context |
| `INET_SERVER_ADDR()` | Server IP | From config |
| `INET_SERVER_PORT()` | Server port | From config |
| `PG_BACKEND_PID()` | Process ID | `getpid()` |
| `PG_TYPEOF(expr)` | Custom | Return type name |
| `PG_COLUMN_SIZE(value)` | `LENGTH(value)` | Approximate |
| `PG_TABLE_SIZE(table)` | Custom query | Sum page sizes |
| `PG_TOTAL_RELATION_SIZE(table)` | Custom query | With indexes |
| `PG_TABLE_IS_VISIBLE(oid)` | `1` | Always visible |
| `HAS_TABLE_PRIVILEGE(...)` | `1` | No real RLS |
| `HAS_COLUMN_PRIVILEGE(...)` | `1` | No real RLS |
| `OBJ_DESCRIPTION(oid, catalog)` | `NULL` | No descriptions |
| `COL_DESCRIPTION(oid, col)` | `NULL` | No descriptions |
| `SHOBJ_DESCRIPTION(oid, catalog)` | `NULL` | No descriptions |
| `TXID_CURRENT()` | Custom | Transaction ID |

### System Catalog Tables

PostgreSQL clients often query system catalogs. Map to SQLite equivalents:

| PostgreSQL | SQLite Translation |
|------------|-------------------|
| `pg_catalog.pg_tables` | `sqlite_master WHERE type='table'` |
| `pg_catalog.pg_views` | `sqlite_master WHERE type='view'` |
| `pg_catalog.pg_indexes` | `sqlite_master WHERE type='index'` |
| `pg_catalog.pg_class` | `sqlite_master` |
| `pg_catalog.pg_namespace` | Synthetic (main schema) |
| `pg_catalog.pg_type` | Synthetic type list |
| `pg_catalog.pg_attribute` | `pragma_table_info(table)` |
| `information_schema.tables` | Already implemented |
| `information_schema.columns` | Already implemented |
| `information_schema.table_constraints` | `pragma_table_info` + `pragma_index_list` |

### psql Commands (via pgwire)

| Command | Implementation |
|---------|---------------|
| `\d` | List tables, views, sequences |
| `\dt` | List tables only |
| `\dv` | List views only |
| `\di` | List indexes |
| `\d tablename` | Describe table structure |
| `\df` | List functions |
| `\dn` | List schemas |
| `\du` | List users/roles |
| `\l` | List databases |
| `\c database` | Connect to database |
| `\timing` | Toggle timing |
| `\x` | Toggle expanded display |

### Implementation

Most of this is already in `internal/pgwire/catalog.go`. Expand to cover more cases.

---

## Phase 8: Advanced Query Features (Priority: Low)

### LATERAL Joins

```sql
-- PostgreSQL:
SELECT * FROM users u, LATERAL (
    SELECT * FROM orders WHERE user_id = u.id LIMIT 3
) o;

-- SQLite workaround (correlated subquery):
SELECT u.*, o.* FROM users u
JOIN (
    SELECT *, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY id) as rn
    FROM orders
) o ON o.user_id = u.id AND o.rn <= 3;
```

**Status:** Complex, may not be fully translatable. Mark as unsupported for now.

### FOR UPDATE / FOR SHARE

```sql
SELECT * FROM users WHERE id = 1 FOR UPDATE;
```

SQLite uses database-level locking, not row-level. Translation:
- `FOR UPDATE` → Begin immediate transaction
- `FOR SHARE` → No-op (SQLite reads don't block)

### DISTINCT ON

```sql
-- PostgreSQL:
SELECT DISTINCT ON (category) * FROM products ORDER BY category, price;

-- SQLite equivalent:
SELECT * FROM products
GROUP BY category
HAVING price = MIN(price)
ORDER BY category;

-- Or with window function:
SELECT * FROM (
    SELECT *, ROW_NUMBER() OVER (PARTITION BY category ORDER BY price) as rn
    FROM products
) WHERE rn = 1;
```

### TABLESAMPLE

```sql
-- PostgreSQL:
SELECT * FROM large_table TABLESAMPLE BERNOULLI(10);

-- SQLite approximation:
SELECT * FROM large_table WHERE ABS(RANDOM()) % 100 < 10;
```

### GROUPING SETS / CUBE / ROLLUP

```sql
-- PostgreSQL:
SELECT category, brand, SUM(sales)
FROM products
GROUP BY GROUPING SETS ((category), (brand), ());

-- SQLite: Use UNION ALL
SELECT category, NULL as brand, SUM(sales) FROM products GROUP BY category
UNION ALL
SELECT NULL, brand, SUM(sales) FROM products GROUP BY brand
UNION ALL
SELECT NULL, NULL, SUM(sales) FROM products;
```

---

## Phase 9: Sequence Functions (Priority: Low)

PostgreSQL sequences for auto-increment values.

| PostgreSQL | SQLite Translation |
|------------|-------------------|
| `NEXTVAL('seq')` | `INSERT INTO _sequences...` + `RETURNING` |
| `CURRVAL('seq')` | Query `_sequences` table |
| `SETVAL('seq', n)` | Update `_sequences` table |
| `LASTVAL()` | Session variable |
| `CREATE SEQUENCE` | Create row in `_sequences` table |
| `DROP SEQUENCE` | Delete from `_sequences` |
| `ALTER SEQUENCE` | Update `_sequences` |

SQLite has `AUTOINCREMENT` but no standalone sequences. Emulate with a table:

```sql
CREATE TABLE _sequences (
    name TEXT PRIMARY KEY,
    current_value INTEGER DEFAULT 0,
    increment_by INTEGER DEFAULT 1,
    min_value INTEGER,
    max_value INTEGER,
    cycle BOOLEAN DEFAULT FALSE
);
```

---

## Phase 10: Type System Enhancements (Priority: Low)

### ENUM Types

```sql
-- PostgreSQL:
CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');

-- SQLite: Store as TEXT with CHECK constraint
-- Metadata in _types table
CREATE TABLE _enums (
    name TEXT PRIMARY KEY,
    values TEXT  -- JSON array of allowed values
);
```

Translation:
- `CREATE TYPE name AS ENUM (...)` → Insert into `_enums`
- Column with enum type → TEXT with generated CHECK
- `DROP TYPE` → Delete from `_enums`

### Composite Types

```sql
-- PostgreSQL:
CREATE TYPE address AS (street TEXT, city TEXT, zip TEXT);

-- SQLite: Store as JSON object
-- Validate structure on insert
```

### Domain Types

```sql
-- PostgreSQL:
CREATE DOMAIN positive_int AS INTEGER CHECK (VALUE > 0);

-- SQLite: Store type + constraint in metadata
-- Apply CHECK during validation
```

---

## Implementation Priority

| Phase | Name | Priority | Effort | Dependencies |
|-------|------|----------|--------|--------------|
| 1 | String Functions | High | Medium | None |
| 2 | Math Functions | Medium | Medium | None |
| 3 | JSON Functions | High | Large | None |
| 4 | Array Functions | Medium | Medium | Phase 3 (json_each) |
| 5 | Full-Text Search | Medium | Medium | Existing FTS5 |
| 6 | Date/Time Functions | Medium | Medium | None |
| 7 | System Catalogs | Low | Medium | pgwire |
| 8 | Advanced Queries | Low | Large | All above |
| 9 | Sequences | Low | Small | None |
| 10 | Type System | Low | Large | None |

**Recommended order:** 1 → 3 → 2 → 4 → 6 → 5 → 7 → 9 → 10 → 8

---

## Testing Strategy

### Unit Tests

For each function added:
1. Test basic translation (input → output SQL)
2. Test edge cases (NULL, empty, special characters)
3. Test error cases (wrong argument count/types)

### Integration Tests

For each category:
1. Execute translated query against SQLite
2. Verify results match expected PostgreSQL behavior
3. Test with real-world query patterns

### E2E Tests

Add to `e2e/tests/dashboard/`:
- `postgres-string-functions.test.ts`
- `postgres-math-functions.test.ts`
- `postgres-json-operators.test.ts`
- `postgres-array-functions.test.ts`
- `postgres-datetime-functions.test.ts`
- `postgres-fts.test.ts`

### Compatibility Matrix

Maintain a compatibility matrix in `docs/POSTGRESQL-COMPATIBILITY.md`:

```markdown
| Feature | PostgreSQL | sblite | Notes |
|---------|------------|--------|-------|
| SPLIT_PART | ✓ | ✓ | Custom function |
| LATERAL JOIN | ✓ | ✗ | Not supported |
| GROUPING SETS | ✓ | ~ | Workaround via UNION |
```

---

## Success Criteria

1. All Phase 1-6 functions translate and execute correctly
2. pg_catalog queries return reasonable results
3. psql basic commands work via pgwire
4. 90%+ of typical PostgreSQL migrations run without modification
5. All new functions have unit tests
6. Compatibility matrix documented

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/pgtranslate/token.go` | Add tokens for new operators |
| `internal/pgtranslate/parser.go` | Parse new operators and functions |
| `internal/pgtranslate/funcs.go` | Add function transformers |
| `internal/pgtranslate/generator.go` | Generate new expressions |
| `internal/db/db.go` | Register custom SQLite functions |
| `internal/pgwire/catalog.go` | Expand catalog emulation |
| `go.mod` | (no new dependencies expected) |

---

## References

- [pgsqlite](https://github.com/erans/pgsqlite) - Rust PostgreSQL-to-SQLite translator
- [PostgreSQL Functions](https://www.postgresql.org/docs/current/functions.html)
- [SQLite Functions](https://www.sqlite.org/lang_corefunc.html)
- [SQLite JSON1](https://www.sqlite.org/json1.html)
- [SQLite FTS5](https://www.sqlite.org/fts5.html)
