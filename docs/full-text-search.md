# Full-Text Search

sblite provides full-text search capabilities using SQLite FTS5, with an API designed to be compatible with the Supabase JavaScript client's `textSearch` method.

## Overview

Full-text search allows you to search for text content across one or more columns in your tables. sblite uses SQLite FTS5 (Full-Text Search 5) virtual tables under the hood, which provides fast and efficient text searching with support for:

- Multiple search types (plain, phrase, websearch)
- Tokenizer options (unicode61, porter stemming, trigram)
- Boolean operators (AND, OR, NOT)
- Prefix matching
- Automatic synchronization via triggers

## Creating an FTS Index

Before you can perform full-text searches, you need to create an FTS index on the columns you want to search.

### Using the Admin API

```bash
curl -X POST http://localhost:8080/admin/v1/tables/articles/fts \
  -H "Authorization: Bearer $SERVICE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "search",
    "columns": ["title", "body"],
    "tokenizer": "unicode61"
  }'
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Unique name for the FTS index |
| `columns` | array | Yes | List of text columns to include in the index |
| `tokenizer` | string | No | Tokenizer to use (default: `unicode61`) |

### Available Tokenizers

| Tokenizer | Description |
|-----------|-------------|
| `unicode61` | Default. Unicode-aware tokenizer supporting multiple languages |
| `porter` | Porter stemming for English. Matches word stems (e.g., "running" matches "run") |
| `ascii` | Simple ASCII tokenizer for basic use cases |
| `trigram` | Character-level trigrams for fuzzy matching and substring search |

## Performing Full-Text Search

### Using the Supabase JS Client

```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

// Basic text search
const { data, error } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'programming')
```

### Query Types

sblite supports four query types, matching Supabase/PostgreSQL's textSearch API:

#### Plain Text (plfts) - Default
All terms must match (implicit AND):

```typescript
// Finds articles containing both "fat" AND "cat"
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'fat cat', { type: 'plain' })
```

#### Phrase Search (phfts)
Matches exact phrase in order:

```typescript
// Finds articles containing the exact phrase "fat cat"
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'fat cat', { type: 'phrase' })
```

#### Websearch (wfts)
Google-like search syntax with OR, negation, and quoted phrases:

```typescript
// Finds articles with "cat" OR "dog" but NOT "mouse"
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'cat or dog -mouse', { type: 'websearch' })

// With quoted phrases
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', '"fat cat" or dog', { type: 'websearch' })
```

#### FTS Query (fts)
PostgreSQL tsquery-style operators:

```typescript
// AND: 'term1' & 'term2'
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', "'Go' & 'programming'")

// OR: 'term1' | 'term2'
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', "'Python' | 'JavaScript'")

// Prefix match: 'term':*
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', "'program':*")  // matches "programming", "programmer", etc.
```

### Combining with Other Filters

Full-text search can be combined with other query methods:

```typescript
// Search with author filter
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'programming')
  .eq('author', 'Alice')

// Search with ordering
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'programming')
  .order('created_at', { ascending: false })

// Search with limit
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'programming')
  .limit(10)
```

### Ordering by Relevance

To order search results by relevance (most relevant first), use `.order()` with the FTS column name. This matches Supabase behavior where ordering by the tsvector column sorts by relevance:

```typescript
// Order by relevance (most relevant first)
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'programming')
  .order('body')  // Order by FTS column = order by relevance

// Order by relevance descending (least relevant first)
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'programming')
  .order('body', { ascending: false })

// Combine relevance ordering with other filters
const { data } = await supabase
  .from('articles')
  .select('*')
  .textSearch('body', 'cat OR dog')
  .eq('author', 'Alice')
  .order('body')
  .limit(10)
```

Internally, sblite uses FTS5's BM25 ranking algorithm to determine relevance scores. The `rank` column from FTS5 is used for ordering but is not exposed in the results.

## Managing FTS Indexes

### List Indexes

```bash
curl http://localhost:8080/admin/v1/tables/articles/fts \
  -H "Authorization: Bearer $SERVICE_KEY"
```

### Get Index Details

```bash
curl http://localhost:8080/admin/v1/tables/articles/fts/search \
  -H "Authorization: Bearer $SERVICE_KEY"
```

### Rebuild Index

Useful after bulk data changes or to refresh the index:

```bash
curl -X POST http://localhost:8080/admin/v1/tables/articles/fts/search/rebuild \
  -H "Authorization: Bearer $SERVICE_KEY"
```

### Delete Index

```bash
curl -X DELETE http://localhost:8080/admin/v1/tables/articles/fts/search \
  -H "Authorization: Bearer $SERVICE_KEY"
```

## Dashboard UI

FTS indexes can also be managed through the web dashboard at `http://localhost:8080/_`.

### Accessing FTS Management

1. Navigate to a table in the Tables view
2. Click the **Schema** button
3. Scroll down to the **Full-Text Search Indexes** section

### Creating an Index

1. Click **+ Create FTS Index**
2. Enter an index name (e.g., "search")
3. Select one or more text columns to index
4. Choose a tokenizer:
   - **Unicode61** (default) - Multi-language support
   - **Porter Stemming** - English word stems
   - **ASCII** - Simple ASCII tokenizer
   - **Trigram** - Fuzzy/substring matching
5. Click **Create Index**

### Testing Search

1. Click **Test** next to an index
2. Enter a search query
3. Select a query type (Plain, Phrase, Websearch, or FTS Query)
4. Click **Search** to see results
5. The modal shows:
   - The FTS5 query translation
   - Matching results with relevance scores

### Other Actions

- **Rebuild**: Re-index all data (useful after bulk imports)
- **Delete**: Remove the FTS index

## How It Works

### External Content Tables

sblite uses FTS5 external content tables, which means:

1. **No data duplication**: The FTS index references the original table data
2. **Automatic sync**: Triggers keep the index synchronized with source data
3. **Efficient storage**: Only the search index is stored, not a copy of the data

### Sync Triggers

When you create an FTS index, sblite automatically creates three triggers:

- **INSERT trigger**: Adds new rows to the FTS index
- **UPDATE trigger**: Updates FTS index when source rows change
- **DELETE trigger**: Removes rows from FTS index when deleted

These triggers ensure your FTS index is always up-to-date with your data.

## Migration to Supabase

When migrating to Supabase/PostgreSQL:

1. **Query syntax**: The textSearch API is compatible, so your client code will continue to work
2. **Index recreation**: You'll need to create PostgreSQL tsvector indexes:

```sql
-- Create GIN index for full-text search
CREATE INDEX articles_search_idx ON articles
USING GIN (to_tsvector('english', title || ' ' || body));
```

3. **Tokenizers**: Map sblite tokenizers to PostgreSQL text search configurations:
   - `unicode61` → `simple` or language-specific (e.g., `english`)
   - `porter` → `english` (includes stemming)

## Limitations

### Current Limitations

1. **Highlighting**: `ts_headline` equivalent is not implemented
2. **Configuration parameter**: The `config` option in textSearch is ignored (tokenizer is set at index creation)
3. **Rank score not exposed**: While ordering by relevance is supported, the actual BM25 score is not included in results

### FTS5 Limitations

1. **Standalone NOT**: Queries with only negation (e.g., `!'cat'`) are not supported - you need at least one positive term
2. **Hyphenated terms**: Words with hyphens are tokenized separately (e.g., "full-text" becomes "full" AND "text")

## Best Practices

1. **Choose the right tokenizer**:
   - Use `unicode61` for multilingual content
   - Use `porter` for English content where stemming is beneficial
   - Use `trigram` when you need substring matching

2. **Index relevant columns only**: Include only columns that users will search

3. **Rebuild after bulk imports**: Call the rebuild endpoint after importing large amounts of data

4. **Use appropriate query types**:
   - `plain` for simple searches
   - `phrase` for exact phrase matching
   - `websearch` for user-facing search boxes
   - `fts` for advanced programmatic queries
