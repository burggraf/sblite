# PostgreSQL Parser Integration Design

**Date:** 2025-01-24
**Status:** Planned (not yet implemented)

## Overview

Integrate [auxten/postgresql-parser](https://github.com/auxten/postgresql-parser) to provide fuller PostgreSQL syntax coverage in sblite's SQL translation layer.

## Motivation

The current translation approach uses:
1. **Regex rules** - Fast but fragile, can match inside string literals
2. **Custom AST parser** - More accurate but limited coverage

Users want support for:
- **Window functions**: `ROW_NUMBER() OVER (PARTITION BY ... ORDER BY ...)`
- **Arrays**: `ARRAY[]`, `unnest()`, `array_agg()`, `ANY/ALL`
- **CTEs**: `WITH` clauses and `WITH RECURSIVE`

## Analysis

### postgresql-parser Capabilities

Tested successfully parsing:
```sql
-- Window functions
SELECT name, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) FROM employees

-- Arrays
SELECT * FROM users WHERE id = ANY(ARRAY[1, 2, 3])

-- CTEs
WITH recent AS (SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '7 days')
SELECT * FROM recent

-- Recursive CTEs
WITH RECURSIVE tree AS (...) SELECT * FROM tree
```

### Cost Assessment

| Metric | Value |
|--------|-------|
| Transitive dependencies | ~558 |
| Binary size impact | +23MB (~doubles sblite binary) |
| CGO required | No (pure Go) |
| Maintenance burden | Medium (CockroachDB-derived, well-maintained) |

### Recommended Approach: Hybrid Integration

Use postgresql-parser selectively for complex queries while keeping the fast regex path for simple ones.

```go
func TranslateQuery(sql string) (string, error) {
    // Fast path: simple queries without complex features
    if !hasComplexFeatures(sql) {
        return regexTranslate(sql), nil
    }

    // Slow path: use full parser for complex queries
    stmts, err := parser.Parse(sql)
    if err != nil {
        // Fallback to regex if parsing fails
        return regexTranslate(sql), nil
    }

    return translateAST(stmts[0].AST)
}

func hasComplexFeatures(sql string) bool {
    upper := strings.ToUpper(sql)
    return strings.Contains(upper, "OVER") ||
           strings.Contains(upper, "ARRAY[") ||
           strings.Contains(upper, "WITH ")
}
```

## Implementation Plan

### Phase 1: Add Dependency
- Add `github.com/auxten/postgresql-parser` to go.mod
- Create `internal/pgtranslate/pgparser/` wrapper package
- Isolate dependency to minimize import surface

### Phase 2: AST Walker for Translation
- Implement visitor pattern to walk CockroachDB AST
- Map PostgreSQL AST nodes to SQLite equivalents
- Handle unsupported features gracefully (return error or fallback)

### Phase 3: Feature Implementation

#### Window Functions → Subqueries
```sql
-- PostgreSQL
SELECT name, ROW_NUMBER() OVER (ORDER BY created_at) as rn FROM users

-- SQLite (approximate)
SELECT name, (SELECT COUNT(*) FROM users u2 WHERE u2.created_at <= users.created_at) as rn FROM users
```

#### Arrays → JSON Arrays
```sql
-- PostgreSQL
SELECT * FROM users WHERE id = ANY(ARRAY[1, 2, 3])

-- SQLite
SELECT * FROM users WHERE id IN (1, 2, 3)
```

#### CTEs → Unchanged (SQLite supports WITH)
```sql
-- Both PostgreSQL and SQLite
WITH recent AS (...) SELECT * FROM recent
```

### Phase 4: Integration
- Wire into dashboard SQL browser
- Add feature flag to enable/disable
- Update tests

## Alternatives Considered

1. **Extend custom parser** - More work, but no dependency cost
2. **Full replacement** - Simpler code but larger binary
3. **Do nothing** - Current coverage may be sufficient for most Supabase use cases

## Decision

**Defer implementation.** The current translation layer handles common Supabase patterns. Will revisit when users report specific unsupported query patterns.

## References

- [postgresql-parser GitHub](https://github.com/auxten/postgresql-parser)
- [SQLite Window Functions](https://www.sqlite.org/windowfunctions.html) (supported since 3.25)
- [SQLite JSON1](https://www.sqlite.org/json1.html)
