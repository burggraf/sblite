# Full-Text Search Design

**Date:** 2026-01-18
**Status:** Draft
**Goal:** Implement full-text search using SQLite FTS5 with Supabase API compatibility and clean migration path to PostgreSQL.

## Overview

sblite will support full-text search through SQLite's FTS5 extension while maintaining API compatibility with Supabase's `textSearch` filter. Applications using `@supabase/supabase-js` will work unchanged, and the migration export tool will generate equivalent PostgreSQL FTS setup.

Key design principles:

1. **API Compatibility** - Support Supabase's `.textSearch(column, query, options)` API
2. **Transparent Indexing** - FTS5 virtual tables managed automatically
3. **Migration Support** - Export generates PostgreSQL tsvector columns and GIN indexes
4. **Minimal Overhead** - External content tables to avoid data duplication

## Background: FTS Technologies

### SQLite FTS5

```sql
-- Create FTS index (external content table)
CREATE VIRTUAL TABLE posts_fts USING fts5(
    title, body,
    content='posts',
    content_rowid='id'
);

-- Query
SELECT * FROM posts WHERE id IN (
    SELECT rowid FROM posts_fts WHERE posts_fts MATCH 'search terms'
);
```

Key characteristics:
- Virtual tables with MATCH operator
- Tokenizers: unicode61 (default), porter (stemming), trigram
- Built-in BM25 ranking via `rank` column
- External content tables avoid data duplication

### PostgreSQL/Supabase FTS

```sql
-- Add tsvector column
ALTER TABLE posts ADD COLUMN fts tsvector
    GENERATED ALWAYS AS (to_tsvector('english', coalesce(title,'') || ' ' || coalesce(body,''))) STORED;

-- Create index
CREATE INDEX posts_fts_idx ON posts USING GIN(fts);

-- Query
SELECT * FROM posts WHERE fts @@ websearch_to_tsquery('english', 'search terms');
```

Key characteristics:
- tsvector/tsquery types with @@ operator
- Language-aware stemming via text search configurations
- GIN indexes for performance
- Multiple query parsers: plainto_tsquery, phraseto_tsquery, websearch_to_tsquery

### Supabase Client API

```javascript
const { data } = await supabase
    .from('posts')
    .select()
    .textSearch('fts', "'fat' & 'cat'", {
        type: 'websearch',  // 'plain' | 'phrase' | 'websearch'
        config: 'english'
    })
```

## Design

### FTS Metadata Schema

Extend `_columns` table to track FTS configuration:

```sql
-- Add FTS columns to _columns table
ALTER TABLE _columns ADD COLUMN fts_enabled INTEGER DEFAULT 0;
ALTER TABLE _columns ADD COLUMN fts_weight TEXT;  -- 'A', 'B', 'C', 'D' for ranking

-- New table for FTS index tracking
CREATE TABLE IF NOT EXISTS _fts_indexes (
    table_name    TEXT NOT NULL,
    index_name    TEXT NOT NULL,
    columns       TEXT NOT NULL,  -- JSON array of column names
    tokenizer     TEXT DEFAULT 'unicode61',
    created_at    TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (table_name, index_name)
);
```

### FTS Index Management

#### Creating an FTS Index

`POST /admin/v1/tables/{name}/fts`

```json
{
    "name": "search_idx",
    "columns": ["title", "body"],
    "tokenizer": "porter"
}
```

Internal flow:

1. Validate table and columns exist
2. Create FTS5 virtual table:
   ```sql
   CREATE VIRTUAL TABLE {table}_fts_{name} USING fts5(
       {columns...},
       content='{table}',
       content_rowid='id',
       tokenize='{tokenizer}'
   );
   ```
3. Create sync triggers (see below)
4. Populate initial index:
   ```sql
   INSERT INTO {table}_fts_{name}(rowid, {columns...})
   SELECT id, {columns...} FROM {table};
   ```
5. Record in `_fts_indexes`

#### Sync Triggers

Keep FTS index synchronized with source table:

```sql
-- After INSERT
CREATE TRIGGER {table}_fts_{name}_ai AFTER INSERT ON {table} BEGIN
    INSERT INTO {table}_fts_{name}(rowid, {columns...})
    VALUES (NEW.id, NEW.{col1}, NEW.{col2}, ...);
END;

-- After UPDATE
CREATE TRIGGER {table}_fts_{name}_au AFTER UPDATE ON {table} BEGIN
    INSERT INTO {table}_fts_{name}({table}_fts_{name}, rowid, {columns...})
    VALUES ('delete', OLD.id, OLD.{col1}, OLD.{col2}, ...);
    INSERT INTO {table}_fts_{name}(rowid, {columns...})
    VALUES (NEW.id, NEW.{col1}, NEW.{col2}, ...);
END;

-- After DELETE
CREATE TRIGGER {table}_fts_{name}_ad AFTER DELETE ON {table} BEGIN
    INSERT INTO {table}_fts_{name}({table}_fts_{name}, rowid, {columns...})
    VALUES ('delete', OLD.id, OLD.{col1}, OLD.{col2}, ...);
END;
```

#### Dropping an FTS Index

`DELETE /admin/v1/tables/{name}/fts/{index_name}`

1. Drop triggers
2. Drop FTS virtual table
3. Remove from `_fts_indexes`

### REST API: textSearch Filter

#### Query Syntax

```
GET /rest/v1/posts?fts=fts.search%20terms
GET /rest/v1/posts?title=fts.search%20terms
GET /rest/v1/posts?fts=plfts.fat%20cat          (plain)
GET /rest/v1/posts?fts=phfts.fat%20cat          (phrase)
GET /rest/v1/posts?fts=wfts.fat%20or%20cat      (websearch)
```

PostgREST operators:
- `fts` - Full-text search (default, uses to_tsquery)
- `plfts` - Plain text search (uses plainto_tsquery)
- `phfts` - Phrase search (uses phraseto_tsquery)
- `wfts` - Websearch (uses websearch_to_tsquery)

#### Query Translation

| Supabase Type | PostgreSQL | SQLite FTS5 |
|---------------|------------|-------------|
| `fts` (default) | `to_tsquery()` | Direct MATCH with operators |
| `plfts` (plain) | `plainto_tsquery()` | MATCH with implicit AND |
| `phfts` (phrase) | `phraseto_tsquery()` | MATCH with quoted phrase |
| `wfts` (websearch) | `websearch_to_tsquery()` | Parse and convert to FTS5 syntax |

#### Websearch Syntax Conversion

Supabase websearch syntax → FTS5:

| Websearch | Meaning | FTS5 |
|-----------|---------|------|
| `cat dog` | cat AND dog | `cat AND dog` |
| `cat or dog` | cat OR dog | `cat OR dog` |
| `-cat` | NOT cat | `NOT cat` |
| `"fat cat"` | exact phrase | `"fat cat"` |

```go
func websearchToFTS5(query string) string {
    // Parse websearch syntax and convert to FTS5
    // "fat cat" or dog -mouse → "fat cat" OR dog NOT mouse
}
```

#### SQL Generation

```go
func (b *Builder) buildFTSCondition(table, column, op, query string) string {
    // Find FTS index for this column
    ftsTable := b.findFTSIndex(table, column)
    if ftsTable == "" {
        return "" // No FTS index, skip
    }

    // Convert query based on operator type
    ftsQuery := b.convertQuery(op, query)

    // Return subquery condition
    return fmt.Sprintf(
        "id IN (SELECT rowid FROM %s WHERE %s MATCH %s ORDER BY rank)",
        ftsTable, ftsTable, sqlQuote(ftsQuery),
    )
}
```

### Dashboard Integration

#### FTS Index Management UI

Add to Table Schema view:
- List existing FTS indexes
- Create new FTS index (select columns, tokenizer)
- Drop FTS index
- Rebuild index (for maintenance)

#### Search Preview

- Test search queries against FTS indexes
- Show ranked results with snippets
- Display highlight() output

### Migration Export

#### PostgreSQL DDL Generation

When exporting schema, generate PostgreSQL FTS setup:

```sql
-- For table 'posts' with FTS index on columns 'title', 'body'

-- Option 1: Generated column (simpler, automatic)
ALTER TABLE "posts" ADD COLUMN "search_vector" tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce("title", '')), 'A') ||
        setweight(to_tsvector('english', coalesce("body", '')), 'B')
    ) STORED;

CREATE INDEX "posts_search_idx" ON "posts" USING GIN("search_vector");

-- Option 2: Trigger-based (more control, for complex cases)
-- Generated if user requests via export flag
```

#### Export Command

```bash
# Include FTS setup in export
./sblite migrate export --db data.db --output schema.sql

# Skip FTS (data-only migration)
./sblite migrate export --db data.db --output schema.sql --no-fts
```

#### Metadata Export

The export includes comments documenting FTS configuration:

```sql
-- FTS Index: posts_search_idx
-- Columns: title (weight A), body (weight B)
-- Tokenizer: porter (mapped to 'english' text search config)
```

### Tokenizer Mapping

| SQLite FTS5 | PostgreSQL Equivalent | Notes |
|-------------|----------------------|-------|
| unicode61 | 'simple' | Basic tokenization |
| porter | 'english' | English stemming |
| trigram | pg_trgm extension | For fuzzy/partial matching |

## API Reference

### Admin Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/v1/tables/{name}/fts` | GET | List FTS indexes for table |
| `/admin/v1/tables/{name}/fts` | POST | Create FTS index |
| `/admin/v1/tables/{name}/fts/{idx}` | DELETE | Drop FTS index |
| `/admin/v1/tables/{name}/fts/{idx}/rebuild` | POST | Rebuild FTS index |

### Dashboard Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_/api/tables/{name}/fts` | GET | List FTS indexes |
| `/_/api/tables/{name}/fts` | POST | Create FTS index |
| `/_/api/tables/{name}/fts/{idx}` | DELETE | Drop FTS index |
| `/_/api/tables/{name}/fts/test` | POST | Test search query |

### REST Filter Examples

```javascript
// Using @supabase/supabase-js

// Basic text search
const { data } = await supabase
    .from('posts')
    .select()
    .textSearch('search_vector', 'fat & cat')

// Websearch (natural language)
const { data } = await supabase
    .from('posts')
    .select()
    .textSearch('search_vector', 'fat or cat', { type: 'websearch' })

// Phrase search
const { data } = await supabase
    .from('posts')
    .select()
    .textSearch('search_vector', 'fat cat', { type: 'phrase' })

// With ranking (order by relevance)
const { data } = await supabase
    .from('posts')
    .select()
    .textSearch('search_vector', 'search terms')
    .order('search_vector', { ascending: false })  // Note: ranking behavior differs
```

## Implementation Plan

### Phase 1: Core FTS5 Integration

1. Add `_fts_indexes` schema table
2. Implement FTS index creation with sync triggers
3. Basic MATCH query support in REST handler
4. Unit tests for FTS operations

### Phase 2: Query Type Support

1. Implement `fts`, `plfts`, `phfts`, `wfts` operators
2. Websearch syntax parser
3. Ranking/ordering support
4. E2E tests with Supabase client

### Phase 3: Admin & Dashboard

1. Admin API endpoints for FTS management
2. Dashboard UI for index management
3. Search testing/preview UI

### Phase 4: Migration Export

1. PostgreSQL tsvector DDL generation
2. GIN index generation
3. Weight/configuration mapping
4. E2E migration tests

## Package Structure

```
internal/
├── fts/
│   ├── fts.go          # FTS index management (create, drop, rebuild)
│   ├── query.go        # Query parsing and conversion
│   ├── websearch.go    # Websearch syntax parser
│   └── migrate.go      # PostgreSQL DDL generation
├── rest/
│   ├── handler.go      # (modify) Add FTS filter support
│   └── builder.go      # (modify) FTS SQL generation
├── admin/
│   └── handler.go      # (modify) FTS admin endpoints
└── dashboard/
    └── handler.go      # (modify) FTS dashboard endpoints
```

## Compatibility Matrix

| Feature | sblite | Supabase | Migration |
|---------|--------|----------|-----------|
| textSearch filter | ✅ | ✅ | ✅ API compatible |
| plain type | ✅ | ✅ | ✅ |
| phrase type | ✅ | ✅ | ✅ |
| websearch type | ✅ | ✅ | ✅ |
| Language config | 🔸 Tokenizer | ✅ Full configs | ⚠️ Mapped |
| Ranking/ordering | ✅ BM25 | ✅ ts_rank | ⚠️ Different scores |
| highlight() | ❌ Not in API | ✅ | N/A |
| Prefix search | ✅ `word*` | ✅ `:*` | ⚠️ Syntax differs |

## Behavioral Differences

Applications should be aware of these differences:

### Tokenization

- SQLite FTS5 porter stemmer may tokenize differently than PostgreSQL english config
- Stop words may differ between implementations
- Result: Same query may return slightly different result sets

### Ranking

- SQLite FTS5 uses BM25 algorithm
- PostgreSQL uses ts_rank or ts_rank_cd
- Result: Same results but potentially different ordering

### Recommendations

1. **For migration**: Test critical search queries after migration
2. **For exact parity**: Use simple tokenization on both sides
3. **For production Supabase**: Consider tuning ts_rank weights post-migration

## Testing Strategy

### Unit Tests

- FTS index creation/deletion
- Sync trigger correctness
- Query parsing (all types)
- Websearch syntax conversion

### Integration Tests

- REST API with FTS filters
- Admin API endpoints
- Dashboard FTS operations

### E2E Tests

```javascript
// e2e/tests/filters/text-search.test.ts

describe('textSearch', () => {
    beforeAll(async () => {
        // Create table with FTS index
        await adminClient.post('/admin/v1/tables', {
            name: 'articles',
            columns: [
                { name: 'id', type: 'uuid', primary: true },
                { name: 'title', type: 'text' },
                { name: 'body', type: 'text' }
            ]
        });

        await adminClient.post('/admin/v1/tables/articles/fts', {
            name: 'search_idx',
            columns: ['title', 'body']
        });

        // Insert test data
        await supabase.from('articles').insert([
            { title: 'Fat Cat', body: 'A story about a fat cat' },
            { title: 'Thin Dog', body: 'A story about a thin dog' }
        ]);
    });

    test('basic text search', async () => {
        const { data } = await supabase
            .from('articles')
            .select()
            .textSearch('search_idx', 'fat cat');

        expect(data).toHaveLength(1);
        expect(data[0].title).toBe('Fat Cat');
    });

    test('websearch OR', async () => {
        const { data } = await supabase
            .from('articles')
            .select()
            .textSearch('search_idx', 'cat or dog', { type: 'websearch' });

        expect(data).toHaveLength(2);
    });
});
```

### Migration Tests

- Export FTS-enabled table to PostgreSQL DDL
- Verify generated tsvector columns
- Verify GIN index creation
- Test same queries work on both

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| FTS5 not available | High | FTS5 included in modernc.org/sqlite by default |
| Performance on large tables | Medium | External content tables, proper indexing |
| Query syntax differences | Medium | Thorough query translation layer |
| Ranking differences | Low | Document as expected behavior |
| Trigger overhead | Low | Minimal for typical write patterns |

## Future Enhancements

Not included in initial implementation:

- **snippet()** - Return text snippets with highlighted matches
- **highlight()** - Mark matching terms in results
- **Trigram search** - Fuzzy/partial matching
- **Multiple FTS indexes per table** - Different configurations
- **Custom dictionaries** - Domain-specific tokenization

## References

- [SQLite FTS5 Documentation](https://sqlite.org/fts5.html)
- [Supabase Full Text Search](https://supabase.com/docs/guides/database/full-text-search)
- [Supabase textSearch API](https://supabase.com/docs/reference/javascript/textsearch)
- [PostgreSQL Full Text Search](https://www.postgresql.org/docs/current/textsearch.html)
