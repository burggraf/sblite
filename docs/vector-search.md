# Vector Search

sblite provides pgvector-compatible vector similarity search for AI/ML applications including RAG (Retrieval-Augmented Generation), semantic search, and recommendation systems.

## Overview

- **Storage**: Vectors are stored as JSON arrays in TEXT columns
- **Search**: Built-in `vector_search` RPC function with RLS support
- **Metrics**: Cosine similarity, L2 (Euclidean) distance, dot product
- **Migration**: Exports to native pgvector types for seamless Supabase migration

## Quick Start

### 1. Create a Table with Vector Column

Using the Admin API or dashboard:

```javascript
// Create table with vector column (1536 dimensions for OpenAI embeddings)
await fetch('http://localhost:8080/admin/v1/tables', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'apikey': 'your-service-role-key'
  },
  body: JSON.stringify({
    name: 'documents',
    columns: [
      { name: 'id', type: 'uuid', primary: true },
      { name: 'title', type: 'text' },
      { name: 'content', type: 'text' },
      { name: 'embedding', type: 'vector(1536)' }
    ]
  })
})
```

Or via SQL in the dashboard:

```sql
-- Using sblite's PostgreSQL translation
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT,
    content TEXT,
    embedding vector(1536)
);
```

### 2. Insert Documents with Embeddings

```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

// Insert document with embedding from OpenAI
const embedding = await getEmbeddingFromOpenAI(content)

await supabase.from('documents').insert({
  title: 'My Document',
  content: 'This is the document content...',
  embedding: embedding // Array of floats, e.g., [0.1, -0.2, 0.3, ...]
})
```

### 3. Search for Similar Documents

```javascript
// Get embedding for search query
const queryEmbedding = await getEmbeddingFromOpenAI('search query')

// Find similar documents
const { data, error } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryEmbedding,
  match_count: 10,
  match_threshold: 0.7
})

// Results include similarity score
data.forEach(doc => {
  console.log(`${doc.title} - Similarity: ${doc.similarity}`)
})
```

## API Reference

### `vector_search` RPC Function

```typescript
supabase.rpc('vector_search', {
  table_name: string,        // Required: table containing vectors
  embedding_column: string,  // Required: name of vector column
  query_embedding: number[], // Required: query vector (same dimension as column)
  match_count?: number,      // Optional: max results (default: 10)
  match_threshold?: number,  // Optional: minimum similarity (default: 0)
  metric?: string,           // Optional: 'cosine' | 'l2' | 'dot' (default: 'cosine')
  select_columns?: string[], // Optional: columns to return (default: all)
  filter?: object            // Optional: additional WHERE conditions
})
```

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `table_name` | string | Yes | - | Name of the table containing vectors |
| `embedding_column` | string | Yes | - | Name of the vector column to search |
| `query_embedding` | number[] | Yes | - | Query vector (must match column dimension) |
| `match_count` | integer | No | 10 | Maximum number of results to return |
| `match_threshold` | number | No | 0 | Minimum similarity score (0-1 for cosine) |
| `metric` | string | No | 'cosine' | Distance metric: 'cosine', 'l2', or 'dot' |
| `select_columns` | string[] | No | all | Specific columns to return |
| `filter` | object | No | null | Additional filter conditions |

### Response

Returns an array of objects, each containing:
- All requested columns from the row
- `similarity`: Similarity score (higher = more similar)

```javascript
[
  {
    id: '123e4567-e89b-12d3-a456-426614174000',
    title: 'Similar Document',
    content: '...',
    similarity: 0.95
  },
  // ...
]
```

## Distance Metrics

### Cosine Similarity (default)

Best for: Text embeddings, semantic search

- Range: -1 to 1 (1 = identical direction)
- Measures angle between vectors, ignores magnitude
- Most common for embeddings from OpenAI, Cohere, etc.

```javascript
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector,
  metric: 'cosine'
})
```

### L2 Distance (Euclidean)

Best for: Spatial data, image features

- Range: 0 to ∞ (0 = identical)
- Note: Returns negative distance so higher = more similar

```javascript
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector,
  metric: 'l2',
  match_threshold: 0.5  // Maximum distance of 0.5
})
```

### Dot Product

Best for: Pre-normalized vectors, ranking

- Range: -∞ to ∞ (higher = more similar for normalized vectors)
- Fastest computation, but assumes normalized input

```javascript
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: normalizedQueryVector,
  metric: 'dot'
})
```

## Filtering Results

### Match Threshold

Filter results by minimum similarity:

```javascript
// Only return documents with >= 70% similarity
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector,
  match_threshold: 0.7
})
```

### Additional Filters

Combine vector search with standard filters:

```javascript
// Search only within a specific category
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector,
  filter: { category: 'technology' }
})

// Filter by multiple conditions
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector,
  filter: {
    status: 'published',
    author_id: userId
  }
})
```

## Row Level Security

Vector search respects RLS policies. Users only see results they have access to.

### Example: User-Owned Documents

```javascript
// Create RLS policy
await supabase.rpc('create_policy', {
  table_name: 'documents',
  policy_name: 'user_documents',
  command: 'SELECT',
  using_expr: "user_id = auth.uid()"
})

// Enable RLS
await fetch('http://localhost:8080/_/api/rls/documents', {
  method: 'PUT',
  body: JSON.stringify({ enabled: true })
})

// Now vector_search only returns documents owned by the authenticated user
const { data } = await supabase.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector
})
// Results filtered to user's documents only
```

### Service Role Bypass

The `service_role` API key bypasses RLS for administrative operations:

```javascript
const adminClient = createClient(url, serviceRoleKey)

// Returns ALL matching documents regardless of RLS
const { data } = await adminClient.rpc('vector_search', {
  table_name: 'documents',
  embedding_column: 'embedding',
  query_embedding: queryVector
})
```

## Supported Vector Dimensions

Common embedding dimensions:

| Provider | Model | Dimensions |
|----------|-------|------------|
| OpenAI | text-embedding-3-small | 1536 |
| OpenAI | text-embedding-3-large | 3072 |
| OpenAI | text-embedding-ada-002 | 1536 |
| Cohere | embed-english-v3.0 | 1024 |
| Google | textembedding-gecko | 768 |
| Hugging Face | all-MiniLM-L6-v2 | 384 |

Define the dimension when creating the column:

```javascript
{ name: 'embedding', type: 'vector(1536)' }  // OpenAI
{ name: 'embedding', type: 'vector(384)' }   // MiniLM
{ name: 'embedding', type: 'vector(768)' }   // Gecko
```

## Performance Considerations

### Current Implementation

sblite uses **brute-force** similarity search (scanning all rows). This is:

- **Fast for small datasets**: < 10,000 vectors: typically < 50ms
- **Adequate for medium datasets**: 10,000-100,000 vectors: 50-500ms
- **Slow for large datasets**: > 100,000 vectors: consider migration to Supabase

### Optimization Tips

1. **Use filters** to reduce the search space:
   ```javascript
   filter: { category: 'active' }  // Only search active documents
   ```

2. **Limit match_count** to what you need:
   ```javascript
   match_count: 5  // Only get top 5 results
   ```

3. **Use match_threshold** to skip dissimilar rows early:
   ```javascript
   match_threshold: 0.5  // Skip low-similarity results
   ```

### Migration to Supabase

When you outgrow sblite's brute-force search, export to Supabase:

```bash
./sblite migrate export --db data.db -o schema.sql
```

The export includes:
- `CREATE EXTENSION vector` for pgvector
- Vector columns as native `vector(N)` type
- HNSW indexes for fast approximate nearest neighbor search
- Compatible `vector_search` function using pgvector operators

## Example: RAG Application

Complete example for a Retrieval-Augmented Generation (RAG) chatbot:

```javascript
import { createClient } from '@supabase/supabase-js'
import OpenAI from 'openai'

const supabase = createClient(SBLITE_URL, SBLITE_KEY)
const openai = new OpenAI({ apiKey: OPENAI_KEY })

// 1. Index documents
async function indexDocument(title, content) {
  // Generate embedding
  const response = await openai.embeddings.create({
    model: 'text-embedding-3-small',
    input: content
  })
  const embedding = response.data[0].embedding

  // Store in database
  await supabase.from('documents').insert({
    title,
    content,
    embedding
  })
}

// 2. Search for relevant documents
async function searchDocuments(query, limit = 5) {
  // Generate query embedding
  const response = await openai.embeddings.create({
    model: 'text-embedding-3-small',
    input: query
  })
  const queryEmbedding = response.data[0].embedding

  // Vector search
  const { data } = await supabase.rpc('vector_search', {
    table_name: 'documents',
    embedding_column: 'embedding',
    query_embedding: queryEmbedding,
    match_count: limit,
    match_threshold: 0.7
  })

  return data
}

// 3. Generate response with context
async function chat(userMessage) {
  // Find relevant documents
  const relevantDocs = await searchDocuments(userMessage)

  // Build context
  const context = relevantDocs
    .map(doc => `Title: ${doc.title}\nContent: ${doc.content}`)
    .join('\n\n---\n\n')

  // Generate response
  const response = await openai.chat.completions.create({
    model: 'gpt-4',
    messages: [
      {
        role: 'system',
        content: `You are a helpful assistant. Use the following context to answer questions:\n\n${context}`
      },
      { role: 'user', content: userMessage }
    ]
  })

  return response.choices[0].message.content
}

// Usage
await indexDocument('About AI', 'Artificial intelligence is...')
await indexDocument('Machine Learning', 'ML is a subset of AI...')

const answer = await chat('What is the relationship between AI and ML?')
console.log(answer)
```

## Comparison: sblite vs Supabase

| Feature | sblite | Supabase |
|---------|--------|----------|
| Vector type | `vector(N)` | `vector(N)` |
| Storage | JSON in TEXT | Native binary |
| Search API | `supabase.rpc('vector_search', {...})` | Same |
| RLS support | Yes | Yes |
| Index type | None (brute-force) | HNSW, IVFFlat |
| Performance | ~50ms / 10k vectors | <5ms with HNSW |
| Max dimensions | 65,536 | 16,000 (HNSW) |

**When to migrate**: If you need sub-5ms queries on >100k vectors, export to Supabase with:
```bash
./sblite migrate export --db data.db
```

## Troubleshooting

### Dimension Mismatch Error

```
Error: vector dimension mismatch: expected 1536, got 384
```

**Cause**: Query embedding dimension doesn't match column definition.

**Solution**: Ensure you're using the same embedding model for indexing and querying.

### Table Not Found

```
Error: table "documents" not found
```

**Cause**: Table doesn't exist or column metadata not registered.

**Solution**: Create table via Admin API or ensure `_columns` metadata exists.

### Column Not a Vector Type

```
Error: column "content" is not a vector type
```

**Cause**: Trying to search on a non-vector column.

**Solution**: Specify the correct vector column name.

### Empty Results

**Possible causes**:
1. No documents in table
2. All embeddings are NULL
3. `match_threshold` too high
4. RLS filtering all results

**Debug**: Use `service_role` key to bypass RLS and verify data exists.

## See Also

- [Type System](type-system.md) - Supported PostgreSQL types including vector
- [PostgreSQL Functions (RPC)](rpc-functions.md) - Creating custom RPC functions
- [Row Level Security](../CLAUDE.md#row-level-security) - Configuring RLS policies
