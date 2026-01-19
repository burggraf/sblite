# PostgreSQL Syntax Translation for sblite

## Overview

This document analyzes the feasibility and benefits of adding PostgreSQL SQL syntax translation to sblite, inspired by projects like [postlite](https://github.com/benbjohnson/postlite) and [pgsqlite](https://github.com/erans/pgsqlite).

## Current State

**sblite's SQL Browser** (internal/dashboard/handler.go:2969)
- Directly executes SQL queries against SQLite
- No syntax translation layer
- Users must write SQLite-compatible SQL
- Type system provides PostgreSQL type mapping but only for schema metadata

## Relevant Projects Analysis

### postlite (Go, Archived)
**What it does:**
- Network proxy translating PostgreSQL wire protocol to SQLite
- Allows GUI tools (pgAdmin, psql) to connect to SQLite databases
- Virtual `pg_catalog` for system metadata queries

**What it DOESN'T do:**
- SQL syntax translation (focuses on protocol, not query rewriting)
- Limited to enabling remote access, not query compatibility

**Key Insight:** Postlite is about **protocol translation**, not SQL dialect translation.

### pgsqlite (Rust, Experimental)
**What it does:**
- Full PostgreSQL wire protocol implementation
- **Extensive SQL syntax translation** from PostgreSQL to SQLite
- 40+ PostgreSQL type mappings
- Function translations (string, math, array operations)
- Advanced feature support (CTEs, RETURNING, GENERATED columns)

**Architecture:**
1. Protocol Handler (PostgreSQL v3 wire protocol)
2. Query Parser (AST-based, PostgreSQL dialect)
3. **SQL Translator** (converts AST to SQLite syntax)
4. Type System (bidirectional mapping)
5. Security layer (injection protection)

**Key Insight:** This is a comprehensive solution but written in Rust, not Go.

## Benefits for sblite

### 1. SQL Browser Enhancement
**Current limitation:** Users must know SQLite syntax
```sql
-- Current (SQLite syntax required)
SELECT datetime('now', 'localtime')
SELECT substr(name, 1, 5)
```

**With translation:** Write familiar PostgreSQL syntax
```sql
-- PostgreSQL syntax (translated automatically)
SELECT NOW()
SELECT LEFT(name, 5)
```

### 2. Migration Testing
**Current:** Users write migrations in SQLite, must manually rewrite for PostgreSQL
**With translation:** Test PostgreSQL migrations directly against sblite
```sql
-- This would work in sblite AND PostgreSQL
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 3. Improved Type Compatibility
**Current:** Type system is metadata-only (stored in `_columns` table)
**With translation:** Enforce PostgreSQL type semantics at query time
- Validate UUID formats in WHERE clauses
- Enforce NUMERIC precision
- Handle TIMESTAMPTZ timezone conversions

### 4. Reduced Migration Friction
**Current workflow:**
1. Develop with sblite (SQLite syntax)
2. Export schema (`sblite migrate export`)
3. Discover incompatibilities when deploying to Supabase
4. Debug and fix PostgreSQL-specific issues

**Improved workflow:**
1. Develop with sblite (PostgreSQL syntax)
2. Export schema (already PostgreSQL-compatible)
3. Deploy to Supabase (seamless transition)

## Implementation Approaches

### Option 1: Minimal Translation Layer (Recommended)
**Scope:** Translate most common PostgreSQL functions/syntax
**Implementation:** Query rewriting before execution

```go
// internal/sql/translator.go
type Translator struct {
    rules []TranslationRule
}

type TranslationRule interface {
    Match(query string) bool
    Translate(query string) (string, error)
}

// Example rules
var commonRules = []TranslationRule{
    FunctionTranslationRule{
        "NOW()": "datetime('now')",
        "CURRENT_TIMESTAMP": "datetime('now')",
        "LEFT(": "substr(",
        "RIGHT(": "substr(",
    },
    TypeCastRule{
        "::uuid": "",  // Remove UUID casts (SQLite doesn't support)
        "::timestamptz": "",  // Handle as datetime
    },
}
```

**Pros:**
- Lightweight, easy to implement
- Covers 80% of common cases
- No external dependencies
- Can be incrementally improved

**Cons:**
- Regex-based, not AST-based (less robust)
- Won't handle complex syntax
- Edge cases may fail

### Option 2: AST-Based Translation
**Scope:** Full SQL parsing and rewriting
**Implementation:** Use or build a SQL parser

**Option 2a: Use existing Go SQL parser**
```go
import "github.com/pganalyze/pg_query_go/v5"

func translateQuery(pgQuery string) (string, error) {
    // Parse PostgreSQL query into AST
    tree, err := pg_query.Parse(pgQuery)
    if err != nil {
        return "", err
    }

    // Walk AST and rewrite for SQLite
    sqliteAST := rewriteAST(tree)

    // Generate SQLite query
    return generateSQLite(sqliteAST), nil
}
```

**Option 2b: Build custom translator**
- More control, but significant effort
- Need to maintain parser as PostgreSQL evolves

**Pros:**
- Robust, handles complex queries
- Can provide detailed error messages
- Extensible architecture

**Cons:**
- More complex implementation
- Performance overhead from parsing
- May require external C dependencies (pg_query_go uses libpg_query)

### Option 3: Embedded pgsqlite (Not Feasible)
**Scope:** Use pgsqlite as a library
**Problem:** pgsqlite is written in Rust, not Go
**Options:**
- CGO bindings (negates sblite's "no CGO" advantage)
- IPC/network proxy (adds complexity, latency)

**Verdict:** Not recommended

## Recommended Implementation Plan

### Phase 1: SQL Browser Translation (Low-Hanging Fruit)
**Scope:** Add translation to dashboard SQL editor only
**Impact:** Improves UX without affecting core functionality

```go
// internal/dashboard/handler.go
func (h *Handler) handleExecuteSQL(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    // NEW: Translate PostgreSQL syntax to SQLite
    translatedQuery, err := sql.TranslatePostgresToSQLite(req.Query)
    if err != nil {
        // Fall back to original query if translation fails
        translatedQuery = req.Query
    }

    // Execute translated query
    if queryType == "SELECT" || queryType == "PRAGMA" {
        rows, err := h.db.Query(translatedQuery)
        // ... rest of existing code ...
    }
}
```

**Translation rules to prioritize:**
1. Function name mappings (NOW, CURRENT_TIMESTAMP, LEFT, RIGHT, etc.)
2. Type cast removal (::uuid, ::timestamptz)
3. RETURNING clause support (already supported by SQLite 3.35+)
4. Common PostgreSQL string functions

### Phase 2: Migration Validation
**Scope:** Validate migration files for PostgreSQL compatibility
**Implementation:**

```go
// cmd/migration.go - new validation command
sblite migration validate --dialect postgres
```

**Features:**
- Parse migration files
- Check for SQLite-specific syntax
- Suggest PostgreSQL equivalents
- Optional: Auto-translate migrations

### Phase 3: Core REST API Translation (Optional)
**Scope:** Accept PostgreSQL syntax in REST API filters
**Impact:** Allows `@supabase/supabase-js` clients to use PostgreSQL functions

**Example:**
```javascript
// This would work with translation
const { data } = await supabase
  .from('users')
  .select('*')
  .filter('created_at', 'gte', 'NOW() - INTERVAL 7 days')
```

**Caution:** This is complex and may not be worth the effort for the REST API.

## Risks and Considerations

### 1. Incomplete Translation
**Risk:** Users write PostgreSQL queries that can't be translated
**Mitigation:**
- Clear documentation of supported syntax
- Helpful error messages when translation fails
- Option to disable translation and use raw SQLite

### 2. Performance Overhead
**Risk:** Parsing/translation adds latency
**Mitigation:**
- Cache translated queries
- Make translation optional (opt-in flag)
- Profile and optimize hot paths

### 3. Maintenance Burden
**Risk:** PostgreSQL syntax evolves, translation rules need updates
**Mitigation:**
- Start with stable, common syntax
- Incremental improvements based on user feedback
- Automated tests for translation rules

### 4. False Sense of Compatibility
**Risk:** Users think sblite is fully PostgreSQL-compatible
**Mitigation:**
- Clear documentation: "PostgreSQL-friendly, SQLite-powered"
- Highlight differences that can't be bridged
- Test migrations against real Supabase before production

## Alternative: PostgreSQL Mode

Instead of automatic translation, offer an explicit "PostgreSQL mode":

```go
// CLI flag
sblite serve --postgres-mode

// SQL Browser toggle
SELECT * FROM users;  -- Toggle: PostgreSQL syntax / SQLite syntax
```

**Benefits:**
- User controls when translation happens
- Clearer expectations
- Can be more aggressive with translations (fail loudly vs. fall back)

## Recommended Next Steps

1. **Spike: Prototype minimal translator** (1-2 days)
   - Implement 10-15 common function translations
   - Test in SQL browser
   - Measure performance impact

2. **User feedback** (1 week)
   - Add to dashboard as experimental feature
   - Gather feedback on usefulness
   - Identify most-needed translations

3. **Decision point: Proceed or abandon**
   - If valuable: Expand translation rules, add tests
   - If not: Document workarounds, focus elsewhere

## Conclusion

**PostgreSQL syntax translation would significantly improve sblite's developer experience** by:
- Making the SQL browser more intuitive for PostgreSQL developers
- Reducing migration friction to Supabase
- Better aligning sblite's "API-compatible" promise

**Recommended approach:**
- Start with **Phase 1: SQL Browser translation**
- Use **Option 1: Minimal translation layer**
- Make it **opt-in/experimental** initially
- Gather feedback before expanding scope

**Effort estimate:** 2-4 weeks for Phase 1 (high-quality implementation with tests)

**Priority:** Medium-High (improves UX, aligns with Supabase compatibility goals)
