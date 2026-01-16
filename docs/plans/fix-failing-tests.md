# Plan: Fix Failing E2E Tests

## Test Results Summary

- **Passing**: 75 tests
- **Failing**: 44 tests (should be working)
- **Skipped**: 71 tests (features not implemented yet)

## Root Cause Analysis

All 44 failing tests trace back to these **6 core issues** in the codebase:

### Issue 1: Missing Filter Operators (in, like, ilike)

**File**: `internal/rest/query.go`

**Problem**: The `validOperators` map only includes basic comparison operators (eq, neq, gt, gte, lt, lte, is). Missing:
- `in` - for matching values in an array
- `like` - for pattern matching with wildcards
- `ilike` - for case-insensitive pattern matching

**Tests Affected**: 8 tests
- `in()` filter tests (3)
- `like()` pattern tests (2)
- `ilike()` tests (1)
- DELETE with `in()` (2)

**Fix**:
1. Add `in`, `like`, `ilike` to `validOperators` map
2. Update `ToSQL()` method to handle each operator's special syntax:
   - `in`: Generate `column IN (?, ?, ?)` with values parsed from `(val1,val2,val3)`
   - `like`: Generate `column LIKE ?` with `*` → `%` wildcard conversion
   - `ilike`: Generate `column LIKE ? COLLATE NOCASE` for SQLite

---

### Issue 2: Prefer: return=representation Not Implemented for UPDATE/DELETE

**File**: `internal/rest/handler.go`

**Problem**: INSERT handler has partial implementation of `Prefer: return=representation` but UPDATE and DELETE don't return affected rows at all.

**Tests Affected**: 12+ tests
- INSERT with `.select()` (4)
- UPDATE with `.select()` (4)
- DELETE with `.select()` (4+)
- UPSERT with `.select()` (4)

**Fix**:
1. **UPDATE**: Before executing UPDATE, SELECT the matching rows to capture them, then execute UPDATE, then return the captured rows
2. **DELETE**: Before executing DELETE, SELECT the matching rows, then execute DELETE, return the captured rows
3. **INSERT**: Fix current implementation to return array format, respect `select` param for column filtering

---

### Issue 3: Bulk INSERT Not Supported

**File**: `internal/rest/handler.go`

**Problem**: `HandleInsert` only decodes into `map[string]any`, rejecting arrays. PostgREST and Supabase support bulk inserts via JSON arrays.

**Tests Affected**: 2 tests
- "should insert multiple records in a single operation"
- "should insert multiple records and return them"

**Fix**:
1. First try to decode as `[]map[string]any`
2. If that fails, try `map[string]any` (single record)
3. Execute multiple INSERTs in a transaction for bulk

---

### Issue 4: single() Modifier Not Implemented Properly

**File**: `internal/rest/handler.go`

**Problem**: The `single()` modifier should:
- Return a single object instead of array
- Error if 0 rows returned
- Error if >1 rows returned

Currently returns array regardless.

**Tests Affected**: 3 tests
- "should return data as single object instead of array"
- "should error when multiple rows returned"
- "should error when no rows returned"

**Fix**:
1. Check for `Accept: application/vnd.pgrst.object+json` header
2. Check for `Prefer: return=representation,count=exact` with limit=1
3. If single mode:
   - Return 406 error if row count != 1
   - Return single object (not array) if exactly 1 row

---

### Issue 5: Auth Session Issues (Client-Side Supabase-js)

**Problem**: Many auth tests fail because the Supabase-js client isn't persisting sessions between operations. This may be a test configuration issue rather than a server issue.

**Tests Affected**: 15+ auth tests
- `getSession()` returns null after signin
- `refreshSession()` fails
- Auth state change events not firing

**Investigation Needed**:
1. Check if server is setting correct response headers
2. Verify session storage configuration in tests
3. May need to manually pass tokens between operations

---

### Issue 6: User Metadata Not Being Stored

**File**: `internal/server/auth_handlers.go` (likely)

**Problem**: User metadata passed during signup isn't being stored in the database.

**Tests Affected**: 2 tests
- "should store custom user metadata during signup"
- "should update user metadata" / "should merge metadata"

**Fix**:
1. Check signup handler for `user_metadata` field handling
2. Ensure JSON is properly serialized and stored in auth_users table

---

## Recommended Fix Order

### Phase 1: REST API Filters (High Impact)
1. Add `in`, `like`, `ilike` operators to query.go
2. Implement special SQL generation for each

### Phase 2: Return Representation (High Impact)
3. Implement `Prefer: return=representation` for UPDATE
4. Implement `Prefer: return=representation` for DELETE
5. Fix INSERT to return proper array format

### Phase 3: Bulk Operations
6. Support array input for bulk INSERT

### Phase 4: Modifiers
7. Implement `single()` modifier properly

### Phase 5: Auth Improvements
8. Investigate and fix session persistence
9. Fix user metadata storage

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/rest/query.go` | Add `in`, `like`, `ilike` operators; update `ToSQL()` |
| `internal/rest/handler.go` | Add return=representation for UPDATE/DELETE; bulk INSERT; single() modifier |
| `internal/server/auth_handlers.go` | Fix user metadata storage |

## Estimated Test Impact

After fixes:
- Phase 1: +8 passing tests (in, like, ilike filters)
- Phase 2: +12 passing tests (return=representation)
- Phase 3: +2 passing tests (bulk insert)
- Phase 4: +3 passing tests (single modifier)
- Phase 5: +15 passing tests (auth session)

**Total**: ~40 additional passing tests, bringing total from 75 to ~115
