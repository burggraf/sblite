# PostgreSQL Syntax Translation - Implementation Summary

## Quick Answer to Your Questions

### 1. Could we implement something like postlite for the SQL editor?

**Yes, but with a different approach:**

- **postlite** is a PostgreSQL wire protocol proxy (allows psql/pgAdmin to connect to SQLite)
- **What sblite needs** is SQL syntax translation (convert PostgreSQL queries to SQLite)
- **Better reference:** [pgsqlite](https://github.com/erans/pgsqlite) - does SQL translation but is written in Rust

**Recommendation:** Implement a lightweight Go-based SQL translator specifically for sblite's needs

### 2. How could this make sblite more directly compatible with Postgres?

**Major improvements:**

1. **Write-once SQL:** Developers can write PostgreSQL syntax that works in both sblite and Supabase
2. **Migration testing:** Test actual PostgreSQL migrations locally before deploying to Supabase
3. **Reduced friction:** No mental context switching between SQLite and PostgreSQL syntax
4. **Better learning curve:** Learn PostgreSQL once, use everywhere

### 3. Could it help reduce data type translation or other issues?

**Yes, significantly:**

**Current limitations:**
- Type system is metadata-only (stored in `_columns` table)
- No runtime type enforcement
- Developers must manually handle type differences

**With translation:**
- Automatic type mapping (UUID ↔ TEXT, BOOLEAN ↔ INTEGER, JSONB ↔ TEXT)
- Remove PostgreSQL-specific casts (`::uuid`, `::timestamptz`)
- Translate type-specific functions (gen_random_uuid, NOW, etc.)
- Enable PostgreSQL-style CREATE TABLE statements

## What Has Been Implemented (POC)

Created in this session:

### 1. Core Translation Engine
**File:** `internal/pgtranslate/translator.go`

**Handles:**
- 40+ function translations (NOW, LEFT, RIGHT, POSITION, etc.)
- Type cast removal (::uuid, ::timestamptz, etc.)
- Data type mapping (UUID→TEXT, BOOLEAN→INTEGER, etc.)
- Boolean literals (TRUE→1, FALSE→0)
- Special functions (gen_random_uuid)

**Architecture:**
- Rule-based system (easy to extend)
- Regex-based translations (lightweight, no heavy dependencies)
- Fallback mechanism (returns original query if translation fails)
- Safety checks (detects untranslatable features like WINDOW functions)

### 2. Comprehensive Test Suite
**File:** `internal/pgtranslate/translator_test.go`

**Coverage:**
- Date/time functions
- String functions
- Type casts
- Data types
- Boolean literals
- Complex queries
- Edge cases

### 3. Integration Plan
**File:** `internal/pgtranslate/integration_example.go`

Shows exactly how to integrate into:
- Dashboard SQL browser backend (handler.go)
- Frontend UI (app.js)
- Response format changes
- User preferences (toggle on/off)

### 4. Documentation
**Files:**
- `docs/plans/postgres-syntax-translation.md` - Full analysis and implementation plan
- `docs/plans/postgres-translation-examples.md` - Real-world usage examples

## Example Transformations

### Simple Query
```sql
-- Input (PostgreSQL)
SELECT NOW(), LEFT(name, 10), active = TRUE
FROM users
WHERE created_at > NOW() - INTERVAL '7 days';

-- Output (SQLite)
SELECT datetime('now'), SUBSTR(name, 1, 10), active = 1
FROM users
WHERE created_at > datetime('now', '+7 day');
```

### CREATE TABLE
```sql
-- Input (PostgreSQL)
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  active BOOLEAN DEFAULT TRUE,
  metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Output (SQLite)
CREATE TABLE users (
  id TEXT PRIMARY KEY DEFAULT (SELECT lower(hex(randomblob(16)))),
  active INTEGER DEFAULT 1,
  metadata TEXT,
  created_at TEXT DEFAULT datetime('now')
);
```

## Integration Roadmap

### Phase 1: SQL Browser (Recommended First Step)
**Effort:** 1 week
**Impact:** High

- Add translation to `handleExecuteSQL` in dashboard
- Add UI toggle for PostgreSQL mode
- Show translated query in results
- Update error messages to be helpful

### Phase 2: Migration Validation
**Effort:** 1 week
**Impact:** Medium

- Add `sblite migration validate` command
- Check migration files for PostgreSQL compatibility
- Auto-translate migrations (optional)

### Phase 3: Expand Translation Rules
**Effort:** 2-3 weeks (ongoing)
**Impact:** Medium

- Add JSON operators (`->`, `->>`)
- Add more date/time functions
- Add array-to-JSON translations
- Community contributions

### Phase 4: REST API Translation (Optional)
**Effort:** 2 weeks
**Impact:** Low

- Accept PostgreSQL functions in REST filters
- May not be worth the complexity

## Technical Considerations

### Pros
✅ Lightweight (no external dependencies beyond regex)
✅ Easy to extend (add new rules incrementally)
✅ Safe (falls back to original query if translation fails)
✅ No CGO (pure Go, maintains sblite's portability)
✅ Incremental adoption (opt-in feature)

### Cons
⚠️ Regex-based (not as robust as AST parsing)
⚠️ Won't handle all PostgreSQL syntax (document limitations)
⚠️ Maintenance burden (keep up with PostgreSQL changes)
⚠️ Performance overhead (small, but measurable)

### Risk Mitigation
- Make translation **opt-in** via toggle
- Document supported vs. unsupported syntax
- Provide clear error messages
- Add extensive test coverage
- Allow disabling for problematic queries

## Next Steps

### Immediate (This PR)
1. ✅ Create translation engine (`internal/pgtranslate/`)
2. ✅ Write comprehensive tests
3. ✅ Document implementation plan
4. ✅ Provide integration examples
5. ⬜ Get feedback on approach

### Short-term (Next PR)
1. Integrate into SQL browser
2. Add UI toggle
3. Update frontend to show translation info
4. Add E2E tests
5. Update user documentation

### Long-term
1. Gather user feedback
2. Expand translation rules based on demand
3. Add migration validation
4. Consider AST-based approach if needed

## Resources

### Similar Projects
- [pgsqlite](https://github.com/erans/pgsqlite) - Rust-based PostgreSQL to SQLite translator (comprehensive but not Go)
- [postlite](https://github.com/benbjohnson/postlite) - PostgreSQL protocol proxy (archived, different use case)

### Go SQL Parsers (if we need AST-based approach later)
- [pg_query_go](https://github.com/pganalyze/pg_query_go) - PostgreSQL query parser (uses CGO, libpg_query)
- [rqlite/sql](https://github.com/rqlite/sql) - Pure Go SQL parser (SQLite dialect)
- [xwb1989/sqlparser](https://github.com/xwb1989/sqlparser) - MySQL-based SQL parser

## Conclusion

**Yes, this is worth implementing!**

The PostgreSQL syntax translation layer would:
- Significantly improve developer experience
- Reduce migration friction to Supabase
- Better align with sblite's "Supabase-compatible" positioning
- Be relatively straightforward to implement (1-2 weeks for MVP)

**Recommended approach:**
Start with Phase 1 (SQL Browser integration) as an **opt-in experimental feature**, gather feedback, and iterate.

The proof-of-concept code in this commit demonstrates feasibility and provides a solid foundation for implementation.
