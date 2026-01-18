# Vector Search Design Document

## Overview

Add vector similarity search capabilities to sblite that are API-compatible with Supabase's pgvector usage patterns. This enables AI/ML applications (RAG, semantic search, recommendations) while maintaining the migration path to full Supabase.

## Goals

1. **API Compatibility**: Client code using `.rpc()` should work identically on sblite and Supabase
2. **Data Portability**: Vector embeddings export cleanly to pgvector format
3. **Simple Implementation**: Built-in functions in Go, no custom SQL parsing
4. **Reasonable Performance**: Handle 10k-100k vectors for development/small production workloads

## Non-Goals

- Native SQLite extension support (requires CGO)
- Production-scale performance (millions of vectors)
- Training or generating embeddings (out of scope)

## Design

### Storage Format

Vectors stored as JSON arrays in TEXT columns:

```sql
-- sblite table
CREATE TABLE documents (
  id TEXT PRIMARY KEY,
  content TEXT,
  embedding TEXT  -- JSON array: [0.1, 0.2, -0.3, ...]
);

-- Column metadata
INSERT INTO _columns (table_name, column_name, column_type)
VALUES ('documents', 'embedding', 'vector(1536)');
```

The `vector(n)` type in `_columns` tracks the expected dimension for validation and export.

### New Type: `vector(n)`

Add to the type system (`internal/types/`):

| sblite Type | SQLite Storage | PostgreSQL Type | Validation |
|-------------|----------------|-----------------|------------|
| `vector(n)` | TEXT | vector(n) | JSON array, length = n, all numbers |

**Validation rules:**
- Must be valid JSON array
- Array length must equal declared dimension
- All elements must be numbers (float64)
- Elements should be in reasonable range (warn if |x| > 100)

### RPC Endpoint

New endpoint for calling built-in functions:

```
POST /rest/v1/rpc/{function_name}
Content-Type: application/json
Authorization: Bearer <jwt>

{
  "param1": value1,
  "param2": value2
}
```

Returns JSON array of results (matching PostgREST behavior).

### Built-in Vector Functions

#### `vector_search`

Primary similarity search function.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `table_name` | string | yes | Table containing vectors |
| `embedding_column` | string | yes | Column with vector data |
| `query_embedding` | number[] | yes | Vector to search for |
| `match_count` | integer | no | Max results (default: 10) |
| `match_threshold` | number | no | Min similarity 0-1 (default: 0) |
| `select_columns` | string[] | no | Columns to return (default: all) |
| `metric` | string | no | Distance metric (default: "cosine") |
| `filter` | object | no | Additional WHERE conditions |

**Supported metrics:**
- `cosine` - Cosine similarity (most common for embeddings)
- `l2` - Euclidean distance (L2)
- `dot` - Dot product (inner product)

**Example request:**

```javascript
const { data, error } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: [0.1, 0.2, -0.3, ...],  // 1536 dimensions
  match_count: 10,
  match_threshold: 0.7,
  select_columns: ['id', 'content', 'metadata'],
  metric: 'cosine',
  filter: { category: 'tech' }
});
```

**Response:**

```json
[
  {
    "id": "doc-123",
    "content": "Introduction to machine learning...",
    "metadata": {"category": "tech"},
    "similarity": 0.92
  },
  {
    "id": "doc-456",
    "content": "Neural network architectures...",
    "metadata": {"category": "tech"},
    "similarity": 0.87
  }
]
```

#### `vector_match` (Alias)

Simplified interface matching common Supabase patterns:

```javascript
const { data } = await supabase.rpc('vector_match', {
  query_embedding: [...],
  match_threshold: 0.78,
  match_count: 10
});
```

This requires a default table/column configuration (set via dashboard or config).

### Implementation Architecture

```
internal/
├── vector/
│   ├── vector.go       # Core types, parsing, validation
│   ├── distance.go     # Distance/similarity functions (SIMD-optimized)
│   ├── search.go       # Search algorithm (brute force + optional HNSW)
│   └── handler.go      # RPC endpoint handlers
├── types/
│   └── types.go        # Add vector(n) type
└── rest/
    └── rpc.go          # New file: RPC endpoint routing
```

### Distance Calculations

```go
// internal/vector/distance.go

// CosineSimilarity returns similarity between 0 and 1
func CosineSimilarity(a, b []float64) float64 {
    var dot, normA, normB float64
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    if normA == 0 || normB == 0 {
        return 0
    }
    return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// L2Distance returns Euclidean distance (lower = more similar)
func L2Distance(a, b []float64) float64 {
    var sum float64
    for i := range a {
        d := a[i] - b[i]
        sum += d * d
    }
    return math.Sqrt(sum)
}

// DotProduct returns inner product (higher = more similar for normalized vectors)
func DotProduct(a, b []float64) float64 {
    var sum float64
    for i := range a {
        sum += a[i] * b[i]
    }
    return sum
}
```

For better performance, use SIMD-optimized library like `github.com/viterin/vek`.

### Search Algorithm

**Phase 1: Brute Force**

Simple O(n) scan, sufficient for <50k vectors:

```go
func (s *Searcher) Search(ctx context.Context, params SearchParams) ([]Result, error) {
    // 1. Query all rows with embeddings
    rows, err := s.db.Query(ctx,
        "SELECT "+strings.Join(params.SelectColumns, ",")+", "+params.EmbeddingColumn+
        " FROM "+params.TableName+
        " WHERE "+buildFilter(params.Filter))

    // 2. Parse embeddings and compute similarities
    var results []Result
    for rows.Next() {
        embedding := parseEmbedding(row)
        similarity := computeSimilarity(params.QueryEmbedding, embedding, params.Metric)
        if similarity >= params.MatchThreshold {
            results = append(results, Result{...})
        }
    }

    // 3. Sort by similarity descending
    sort.Slice(results, func(i, j int) bool {
        return results[i].Similarity > results[j].Similarity
    })

    // 4. Return top N
    if len(results) > params.MatchCount {
        results = results[:params.MatchCount]
    }
    return results, nil
}
```

**Phase 2: HNSW Index (Optional Enhancement)**

For better performance with larger datasets, add optional in-memory HNSW index:

```go
// Build index on startup or first query
index := hnsw.New(dimensions, hnsw.Config{
    M:              16,
    EfConstruction: 200,
})

// Add vectors
for _, doc := range documents {
    index.Add(doc.ID, doc.Embedding)
}

// Search
results := index.Search(queryEmbedding, matchCount)
```

Index would need to be rebuilt on data changes (INSERT/UPDATE/DELETE).

### RLS Integration

Vector search respects Row Level Security:

```go
func (s *Searcher) Search(ctx context.Context, params SearchParams, userID string) ([]Result, error) {
    // Get RLS policies for table
    policies := s.rls.GetPolicies(params.TableName, "SELECT")

    // Build WHERE clause including RLS conditions
    whereClause := buildFilter(params.Filter)
    if len(policies) > 0 {
        rlsCondition := s.rls.BuildCondition(policies, userID)
        whereClause = whereClause + " AND " + rlsCondition
    }

    // Query with RLS applied
    rows, err := s.db.Query(ctx,
        "SELECT ... FROM "+params.TableName+" WHERE "+whereClause)
    // ...
}
```

### PostgreSQL Export

The `sblite migrate export` command generates pgvector-compatible DDL:

```sql
-- Enable extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Table with vector column
CREATE TABLE documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  content text,
  embedding vector(1536)
);

-- Create HNSW index for fast search
CREATE INDEX ON documents USING hnsw (embedding vector_cosine_ops);

-- Equivalent RPC function
CREATE OR REPLACE FUNCTION vector_search(
  p_table_name text,
  p_embedding_column text,
  p_query_embedding vector,
  p_match_count integer DEFAULT 10,
  p_match_threshold float DEFAULT 0,
  p_select_columns text[] DEFAULT NULL,
  p_metric text DEFAULT 'cosine',
  p_filter jsonb DEFAULT NULL
) RETURNS jsonb AS $$
DECLARE
  result jsonb;
  distance_op text;
BEGIN
  -- Map metric to pgvector operator
  distance_op := CASE p_metric
    WHEN 'cosine' THEN '<=>'
    WHEN 'l2' THEN '<->'
    WHEN 'dot' THEN '<#>'
    ELSE '<=>'
  END;

  -- Dynamic query (simplified, actual impl would be more robust)
  EXECUTE format(
    'SELECT jsonb_agg(row_to_json(t)) FROM (
      SELECT *, 1 - (embedding %s $1) as similarity
      FROM %I
      WHERE 1 - (embedding %s $1) >= $2
      ORDER BY embedding %s $1
      LIMIT $3
    ) t',
    distance_op, p_table_name, distance_op, distance_op
  ) INTO result USING p_query_embedding, p_match_threshold, p_match_count;

  RETURN COALESCE(result, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql;
```

**Data export:**

```sql
-- Convert JSON arrays to pgvector literals
INSERT INTO documents (id, content, embedding)
SELECT
  id,
  content,
  embedding::vector(1536)
FROM json_populate_recordset(null::documents, $json_data$);
```

### Dashboard UI

Add vector management to the dashboard:

**Table Editor:**
- New column type option: `vector(n)` with dimension input
- Show dimension in schema view

**Vector Search Tab (new):**
- Configure default search table/column
- Test vector search with sample embeddings
- View search performance stats

**Settings:**
- Enable/disable HNSW indexing
- Configure index parameters (M, efConstruction)

### Client Usage Examples

**RAG Application:**

```javascript
// 1. Store document with embedding
const embedding = await openai.embeddings.create({
  model: 'text-embedding-3-small',
  input: documentContent
});

await supabase.from('documents').insert({
  content: documentContent,
  embedding: embedding.data[0].embedding  // Stored as JSON array
});

// 2. Semantic search
const queryEmbedding = await openai.embeddings.create({
  model: 'text-embedding-3-small',
  input: userQuestion
});

const { data: relevantDocs } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryEmbedding.data[0].embedding,
  match_count: 5,
  match_threshold: 0.7
});

// 3. Generate response with context
const response = await openai.chat.completions.create({
  model: 'gpt-4',
  messages: [
    { role: 'system', content: 'Answer based on the provided context.' },
    { role: 'user', content: `Context: ${relevantDocs.map(d => d.content).join('\n')}\n\nQuestion: ${userQuestion}` }
  ]
});
```

**Recommendation System:**

```javascript
// Find similar products
const { data: similar } = await supabase.rpc('vector_search', {
  table_name: 'products',
  embedding_column: 'feature_embedding',
  query_embedding: currentProduct.feature_embedding,
  match_count: 10,
  filter: {
    category: currentProduct.category,
    id: { neq: currentProduct.id }  // Exclude current product
  }
});
```

### Migration Path

| Step | Action |
|------|--------|
| 1 | Export schema: `sblite migrate export` generates pgvector DDL |
| 2 | Export data: JSON embeddings converted to pgvector literals |
| 3 | Create indexes: HNSW indexes for production performance |
| 4 | Deploy function: `vector_search` PostgreSQL function |
| 5 | Update connection: Point client to Supabase |

**Client code requires no changes** - same `.rpc('vector_search', {...})` calls work on both platforms.

### Performance Considerations

**Brute Force (Phase 1):**
- 10k vectors, 1536 dimensions: ~50ms per search
- 50k vectors: ~250ms per search
- Acceptable for development and small production workloads

**With HNSW Index (Phase 2):**
- 100k vectors: ~5ms per search
- Memory overhead: ~1.5x vector size
- Index build time: ~30s for 100k vectors

**Optimizations:**
- SIMD-accelerated distance calculations
- Batch embedding loading
- Connection pooling for concurrent searches
- Optional dimension reduction (PCA) for large vectors

### Security Considerations

- RLS policies apply to vector searches
- Input validation: dimension limits, numeric ranges
- Rate limiting for compute-intensive searches
- No arbitrary SQL execution (built-in functions only)

### Testing Strategy

**Unit Tests:**
- Distance function accuracy
- Embedding validation
- Search result ordering

**Integration Tests:**
- RPC endpoint functionality
- RLS policy enforcement
- Concurrent search handling

**E2E Tests:**
- Full RAG workflow with supabase-js
- Migration export/import roundtrip

### Implementation Phases

**Phase 1: Core Vector Search**
- [ ] Add `vector(n)` type to type system
- [ ] Implement distance functions with tests
- [ ] Create `/rest/v1/rpc` endpoint infrastructure
- [ ] Implement `vector_search` function
- [ ] Add embedding validation
- [ ] Update PostgreSQL export for vector columns

**Phase 2: Performance & Polish**
- [ ] SIMD-optimized distance calculations
- [ ] Optional HNSW indexing
- [ ] Dashboard vector management UI
- [ ] Performance benchmarks and tuning

**Phase 3: Advanced Features**
- [ ] Hybrid search (vector + full-text)
- [ ] Multi-vector queries
- [ ] Dimension reduction utilities

## Alternatives Considered

### SQLite Extension (sqlite-vec)

**Pros:** Native performance, standard SQLite patterns
**Cons:** Requires CGO, breaks single-binary promise
**Decision:** Could offer as optional CGO build variant later

### External Vector Database

**Pros:** Production-ready performance (Pinecone, Qdrant, etc.)
**Cons:** External dependency, defeats single-binary purpose
**Decision:** Out of scope for sblite's design goals

### Custom SQL Function Parsing

**Pros:** More flexible, user-defined functions
**Cons:** Complex to implement correctly, security risks
**Decision:** Built-in functions simpler and safer

## References

- [pgvector documentation](https://github.com/pgvector/pgvector)
- [Supabase Vector documentation](https://supabase.com/docs/guides/ai)
- [HNSW algorithm paper](https://arxiv.org/abs/1603.09320)
- [sqlite-vec](https://github.com/asg017/sqlite-vec)
