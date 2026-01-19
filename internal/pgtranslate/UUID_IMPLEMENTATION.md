# UUID v4 Generation Implementation

## Overview

PostgreSQL's `gen_random_uuid()` generates RFC 4122 compliant UUID version 4 (random UUIDs). This document explains how we replicate this behavior in SQLite.

## RFC 4122 UUID v4 Format

```
xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
```

Where:
- `x` = any hexadecimal digit (0-9, a-f)
- `4` = version number (always 4 for UUID v4)
- `y` = variant bits (one of: 8, 9, a, b)

**Structure breakdown:**
- 32 hexadecimal digits total
- 4 hyphens at positions 8, 13, 18, 23
- Total length: 36 characters

**Version field:** Character at position 14 (0-indexed) must be '4'
**Variant field:** Character at position 19 (0-indexed) must be one of '8', '9', 'a', 'b'

These specific values indicate:
- Version 4: randomly generated UUID
- Variant: RFC 4122 compliant (variant bits are '10' in binary)

## SQLite Implementation

### Generated SQL

```sql
(SELECT lower(
  substr(h, 1, 8) || '-' ||
  substr(h, 9, 4) || '-' ||
  '4' || substr(h, 14, 3) || '-' ||
  substr('89ab', (abs(random()) % 4) + 1, 1) || substr(h, 18, 3) || '-' ||
  substr(h, 21, 12)
) FROM (SELECT hex(randomblob(16)) as h))
```

### Step-by-Step Explanation

1. **Generate random bytes:**
   ```sql
   randomblob(16)  -- 16 random bytes (128 bits)
   ```

2. **Convert to hexadecimal:**
   ```sql
   hex(randomblob(16))  -- 32 hex characters
   ```

3. **Store in subquery:**
   ```sql
   SELECT hex(randomblob(16)) as h
   ```
   This ensures we use the same random value throughout the expression.

4. **Extract and format sections:**

   | Section | Source | Length | Result |
   |---------|--------|--------|--------|
   | 1st | `substr(h, 1, 8)` | 8 chars | `xxxxxxxx` |
   | Hyphen | `'-'` | 1 char | `-` |
   | 2nd | `substr(h, 9, 4)` | 4 chars | `xxxx` |
   | Hyphen | `'-'` | 1 char | `-` |
   | **Version** | `'4'` | 1 char | `4` |
   | 3rd rest | `substr(h, 14, 3)` | 3 chars | `xxx` |
   | Hyphen | `'-'` | 1 char | `-` |
   | **Variant** | `substr('89ab', ...)` | 1 char | `y` |
   | 4th rest | `substr(h, 18, 3)` | 3 chars | `xxx` |
   | Hyphen | `'-'` | 1 char | `-` |
   | 5th | `substr(h, 21, 12)` | 12 chars | `xxxxxxxxxxxx` |

5. **Variant selection:**
   ```sql
   substr('89ab', (abs(random()) % 4) + 1, 1)
   ```
   - Generates a random number
   - Takes modulo 4 → result in [0, 3]
   - Adds 1 → result in [1, 4] (SQLite substr is 1-indexed)
   - Selects one of '8', '9', 'a', or 'b'

6. **Lowercase conversion:**
   ```sql
   lower(...)
   ```
   Ensures consistent lowercase output (UUIDs are case-insensitive but lowercase is conventional).

## Validation

### Format Regex

```regex
^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$
```

### Tests

The implementation is validated by:

1. **Format correctness:** UUID matches RFC 4122 v4 pattern
2. **Version field:** Character at position 14 is '4'
3. **Variant field:** Character at position 19 is one of 8, 9, a, b
4. **Length:** Always 36 characters
5. **Hyphens:** At positions 8, 13, 18, 23
6. **Uniqueness:** No duplicates in 1000+ generated UUIDs
7. **Integration:** Works in CREATE TABLE DEFAULT, INSERT VALUES, etc.

See `uuid_test.go` for comprehensive test suite.

## Performance

**Benchmark results:**
- Generation time: ~microseconds per UUID
- Memory: Minimal (one randomblob call per UUID)
- No external dependencies

**Comparison to PostgreSQL:**
- PostgreSQL: Uses native C function from pgcrypto extension
- SQLite: Pure SQL expression, slightly slower but adequate for most use cases
- Both: Cryptographically random (use system PRNG)

## Examples

### Direct generation:
```sql
SELECT gen_random_uuid();
-- Result: '550e8400-e29b-41d4-a716-446655440000'
```

### CREATE TABLE with DEFAULT:
```sql
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT
);
```

### INSERT with explicit UUID:
```sql
INSERT INTO users (id, name) VALUES (gen_random_uuid(), 'Alice');
```

## Limitations

1. **Performance:** Slightly slower than native PostgreSQL implementation
2. **Entropy:** Depends on SQLite's random() function quality
3. **Version:** Only generates UUID v4 (random); does not support v1, v3, v5, etc.

## Compatibility

**PostgreSQL gen_random_uuid():**
- ✅ Generates RFC 4122 compliant UUIDs
- ✅ Version 4 (random)
- ✅ Proper variant bits
- ✅ Case-insensitive output
- ✅ Can be used in DEFAULT constraints
- ✅ Can be used in INSERT/UPDATE statements

**Differences from PostgreSQL:**
- PostgreSQL returns `uuid` type; SQLite returns `TEXT`
- PostgreSQL may use different randomness source (pgcrypto)
- Performance characteristics differ

## References

- [RFC 4122: A Universally Unique IDentifier (UUID) URN Namespace](https://www.rfc-editor.org/rfc/rfc4122)
- [PostgreSQL gen_random_uuid() documentation](https://www.postgresql.org/docs/current/functions-uuid.html)
- [UUID Version 4 Wikipedia](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random))
