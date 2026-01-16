# sblite E2E Test Suite

End-to-end tests for sblite, verifying Supabase JavaScript client compatibility.

## Overview

This test suite validates sblite's compatibility with the official Supabase JavaScript client (`@supabase/supabase-js`). Tests are based on examples from the [Supabase JavaScript documentation](https://supabase.com/docs/reference/javascript/introduction).

## Test Coverage

### REST API Tests

| Category | Tests | Status |
|----------|-------|--------|
| **SELECT** | 13 examples | ✅ Basic implemented, 🔸 Advanced (relationships) pending |
| **INSERT** | 3 examples | ✅ Fully implemented |
| **UPDATE** | 3 examples | ✅ Fully implemented |
| **UPSERT** | 3 examples | ✅ Fully implemented |
| **DELETE** | 3 examples | ✅ Fully implemented |

### Filter Tests

| Filter | Tests | Status |
|--------|-------|--------|
| `eq()` | Equals | ✅ Implemented |
| `neq()` | Not equals | ✅ Implemented |
| `gt()` | Greater than | ✅ Implemented |
| `gte()` | Greater than or equal | ✅ Implemented |
| `lt()` | Less than | ✅ Implemented |
| `lte()` | Less than or equal | ✅ Implemented |
| `like()` | Pattern match (case-sensitive) | ✅ Implemented |
| `ilike()` | Pattern match (case-insensitive) | ✅ Implemented |
| `is()` | Null/boolean check | ✅ Implemented |
| `in()` | Match any in array | ✅ Implemented |
| `contains()` | Array/JSONB containment | ❌ Not implemented |
| `containedBy()` | Contained by check | ❌ Not implemented |
| `rangeGt/Gte/Lt/Lte()` | Range comparisons | ❌ Not implemented |
| `overlaps()` | Array/range overlap | ❌ Not implemented |
| `textSearch()` | Full-text search | ❌ Not implemented |
| `match()` | Multi-column match | ❌ Not implemented |
| `not()` | Negate filter | ❌ Not implemented |
| `or()` | OR logic | ❌ Not implemented |
| `filter()` | Raw PostgREST filter | ❌ Not implemented |

### Modifier Tests

| Modifier | Tests | Status |
|----------|-------|--------|
| `order()` | Sort results | ✅ Implemented |
| `limit()` | Limit rows | ✅ Implemented |
| `range()` | Pagination | ✅ Implemented |
| `single()` | Return single object | ✅ Implemented |
| `maybeSingle()` | Return object or null | ✅ Implemented |
| `csv()` | Return as CSV | ❌ Not implemented |
| `explain()` | Query plan | ❌ Not implemented |

### Auth Tests

| Feature | Tests | Status |
|---------|-------|--------|
| `signUp()` | 5 examples | ✅ Email/password implemented |
| `signInWithPassword()` | 2 examples | ✅ Implemented |
| `signOut()` | 3 examples | ✅ Basic implemented |
| `getSession()` | 1 example | ✅ Implemented |
| `refreshSession()` | 2 examples | ✅ Implemented |
| `setSession()` | 1 example | 🔸 Partially implemented |
| `getUser()` | 2 examples | ✅ Implemented |
| `updateUser()` | 5 examples | ✅ Password/metadata implemented |
| `onAuthStateChange()` | 8 examples | ✅ Core events implemented |
| `resetPasswordForEmail()` | 2 examples | ❌ Not implemented |
| `getClaims()` | 1 example | ❌ Not implemented |

## Quick Start

### Prerequisites

- Node.js 18+
- sblite binary built
- SQLite3 (for test database setup)

### Installation

```bash
cd e2e
npm install
```

### Setup Test Database

```bash
npm run setup
```

This creates a test database with sample data tables.

### Start sblite Server

In a separate terminal:

```bash
cd ..
./sblite serve --db test.db
```

Or use the npm script:

```bash
npm run server:start
```

### Run Tests

```bash
# Run all tests
npm test

# Run specific test categories
npm run test:rest      # REST API tests
npm run test:auth      # Auth tests
npm run test:filters   # Filter tests
npm run test:modifiers # Modifier tests

# Watch mode (re-run on changes)
npm run test:watch

# With UI
npm run test:ui
```

## Test Structure

```
e2e/
├── package.json
├── vitest.config.ts
├── tsconfig.json
├── README.md
├── setup/
│   ├── global-setup.ts     # Test configuration
│   └── test-helpers.ts     # Utility functions
├── scripts/
│   └── setup-test-db.ts    # Database seeding
└── tests/
    ├── rest/
    │   ├── select.test.ts
    │   ├── insert.test.ts
    │   ├── update.test.ts
    │   ├── upsert.test.ts
    │   └── delete.test.ts
    ├── filters/
    │   ├── basic-filters.test.ts
    │   ├── advanced-filters.test.ts
    │   └── logical-filters.test.ts
    ├── modifiers/
    │   └── modifiers.test.ts
    └── auth/
        ├── signup.test.ts
        ├── signin-signout.test.ts
        ├── session.test.ts
        ├── user.test.ts
        ├── auth-state-change.test.ts
        └── password-reset.test.ts
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_URL` | `http://localhost:8080` | sblite server URL |
| `SBLITE_ANON_KEY` | `test-anon-key` | Anonymous API key |
| `SBLITE_JWT_SECRET` | (default) | JWT signing secret |
| `SBLITE_DB_PATH` | `./test.db` | Test database path |

## Writing New Tests

### Test File Template

```typescript
import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Feature Name', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  describe('Example from docs', () => {
    it('should behave as documented', async () => {
      // Test implementation
    })

    it.skip('not implemented yet', async () => {
      // Skipped tests document planned features
    })
  })
})
```

### Test Naming Convention

- Test files: `feature-name.test.ts`
- Describe blocks: Match documentation section names
- Test names: Describe expected behavior

### Compatibility Status

Each test file includes a compatibility summary comment:

```typescript
/**
 * Compatibility Summary:
 *
 * IMPLEMENTED:
 * - Feature A
 * - Feature B
 *
 * NOT IMPLEMENTED:
 * - Feature C (requires PostgreSQL-specific feature)
 */
```

## Documentation References

Tests are based on these Supabase documentation pages:

- [JavaScript Client Reference](https://supabase.com/docs/reference/javascript/introduction)
- [Select](https://supabase.com/docs/reference/javascript/select)
- [Insert](https://supabase.com/docs/reference/javascript/insert)
- [Update](https://supabase.com/docs/reference/javascript/update)
- [Upsert](https://supabase.com/docs/reference/javascript/upsert)
- [Delete](https://supabase.com/docs/reference/javascript/delete)
- [Using Filters](https://supabase.com/docs/reference/javascript/using-filters)
- [Using Modifiers](https://supabase.com/docs/reference/javascript/using-modifiers)
- [Auth - Sign Up](https://supabase.com/docs/reference/javascript/auth-signup)
- [Auth - Sign In](https://supabase.com/docs/reference/javascript/auth-signinwithpassword)
- [Auth - Sign Out](https://supabase.com/docs/reference/javascript/auth-signout)

## Contributing

1. Add new tests for each Supabase feature
2. Use `it.skip()` for unimplemented features
3. Include documentation URL references
4. Add compatibility summary comments
5. Update this README when adding new test categories

## License

MIT
