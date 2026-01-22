# Vector-Feld Demo App Design

## Overview

A client-side React/Vite app demonstrating sblite's pgvector-compatible vector search using Seinfeld scripts. Users enter a topic and find semantically similar dialogue lines.

## Architecture

```
test_apps/vector-feld/
├── src/                    # React/Vite app with shadcn UI
│   ├── App.tsx            # Main search interface
│   ├── components/        # UI components
│   │   ├── search-input.tsx
│   │   ├── result-card.tsx
│   │   └── results-list.tsx
│   └── lib/
│       ├── supabase.ts    # Supabase client setup
│       └── gemini.ts      # Gemini embedding client
├── scripts/               # Node.js data processing scripts
│   ├── import-scripts.ts  # Import CSV → scripts table
│   └── generate-embeddings.ts  # Generate vectors via Gemini
├── migrations/            # sblite migrations
│   └── 20260122000000_create_scripts.sql
├── package.json
├── README.md
└── .env.local             # VITE_SUPABASE_URL, VITE_SUPABASE_ANON_KEY, GOOGLE_API_KEY
```

## Data Flow

1. Migration creates `scripts` table with `embedding vector(768)` column
2. Import script fetches CSV, batch inserts ~1,100 rows
3. Embedding script processes unembedded rows, updates with vectors
4. App UI calls `supabase.rpc('vector_search', {...})` on user input

## Database Schema

### Table: `scripts`

| Column | Type | Description |
|--------|------|-------------|
| `id` | `integer` | Primary key (from CSV ID column) |
| `character` | `text` | Speaker name (JERRY, GEORGE, etc.) |
| `dialogue` | `text` | The spoken line |
| `episode_no` | `integer` | Episode number |
| `seid` | `text` | Season-episode ID (S01E01) |
| `season` | `integer` | Season number |
| `embedding` | `vector(768)` | Gemini text-embedding-004 vector (nullable until generated) |

### Migration: `20260122000000_create_scripts.sql`

```sql
CREATE TABLE scripts (
  id INTEGER PRIMARY KEY,
  character TEXT NOT NULL,
  dialogue TEXT NOT NULL,
  episode_no INTEGER,
  seid TEXT,
  season INTEGER,
  embedding TEXT  -- vector(768) tracked in _columns metadata
);

CREATE INDEX idx_scripts_character ON scripts(character);
CREATE INDEX idx_scripts_season ON scripts(season);
```

## Data Import Script

**File:** `scripts/import-scripts.ts`

**Process:**
1. Fetch CSV from `https://media.githubusercontent.com/media/burggraf/datasets/refs/heads/main/seinfeld/scripts.csv`
2. Parse CSV rows using `csv-parse` (handles quotes, commas in dialogue)
3. Batch insert in chunks of 100 rows using `supabase.from('scripts').insert()`
4. Report progress: "Imported 100/1100 rows..."

**Key considerations:**
- Skip header row
- Handle floating point episode/season values from CSV (convert to integers)
- Validate dialogue is non-empty before inserting
- Use service role key for bulk insert

## Embedding Generation Script

**File:** `scripts/generate-embeddings.ts`

**Process:**
1. Query rows where `embedding IS NULL` (resumable)
2. Process rows one at a time (Gemini embedContent is single-input)
3. For each row:
   - Call Gemini `text-embedding-004` with dialogue text
   - Update row with embedding vector
   - Log progress: "Embedded 200/1100 (18%)"
4. Handle rate limits with exponential backoff (15 req/min free tier)
5. Continue until no unembedded rows remain

**API call:**
```typescript
const response = await genAI.getGenerativeModel({ model: 'text-embedding-004' })
  .embedContent(dialogue);
// Returns 768-dimension vector
```

**Environment:** `GOOGLE_API_KEY` in `.env.local`

**Resumability:** Script queries for `embedding IS NULL`, so it can be stopped and restarted.

## Search UI Design

```
┌─────────────────────────────────────────────────────────┐
│  Vector-Feld                                            │
│  Search Seinfeld scripts by topic                       │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────┐  ┌──────────────┐  │
│  │ Enter a topic...                │  │   Search     │  │
│  └─────────────────────────────────┘  └──────────────┘  │
│                                                         │
│  Results for "social anxiety"               10 matches  │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐│
│  │ GEORGE                           S03E12 • 94% match ││
│  │ "I'm much more comfortable criticizing people       ││
│  │  behind their backs."                               ││
│  └─────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────┐│
│  │ JERRY                            S05E04 • 91% match ││
│  │ "People on dates shouldn't even be allowed out      ││
│  │  in public."                                        ││
│  └─────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────┘
```

**Components:**
- `SearchInput` - Input field with search button
- `ResultCard` - Card showing character, dialogue, episode, similarity score
- `ResultsList` - Scrollable list of result cards

**Search flow:**
1. User enters topic, clicks Search (or presses Enter)
2. App calls Gemini API to embed the query (client-side)
3. App calls `supabase.rpc('vector_search', {...})` with query embedding
4. Results displayed as cards sorted by similarity descending

## Package Configuration

### Dependencies

**Runtime:**
- `react`, `react-dom` - UI framework
- `@supabase/supabase-js` - Database client
- `@google/generative-ai` - Gemini embeddings
- `tailwindcss`, `@tailwindcss/vite` - Styling
- shadcn: `card`, `input`, `button`, `badge`, `scroll-area`

**Scripts:**
- `csv-parse` - CSV parsing for import
- `tsx` - TypeScript execution

### Scripts

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "import": "tsx scripts/import-scripts.ts",
    "embed": "tsx scripts/generate-embeddings.ts"
  }
}
```

### Environment Variables

`.env.local`:
```
VITE_SUPABASE_URL=http://localhost:8080
VITE_SUPABASE_ANON_KEY=<anon-key>
GOOGLE_API_KEY=<gemini-api-key>
```

## Usage Instructions

1. Start sblite server:
   ```bash
   ./sblite serve --db test_apps/vector-feld/data.db
   ```

2. Apply migration:
   ```bash
   ./sblite db push --db test_apps/vector-feld/data.db --migrations-dir test_apps/vector-feld/migrations
   ```

3. Install dependencies:
   ```bash
   cd test_apps/vector-feld && pnpm install
   ```

4. Configure environment:
   ```bash
   cp .env.example .env.local
   # Edit .env.local with your API keys
   ```

5. Import Seinfeld scripts:
   ```bash
   pnpm import
   ```

6. Generate embeddings (takes ~10 min with rate limiting):
   ```bash
   pnpm embed
   ```

7. Run the app:
   ```bash
   pnpm dev
   ```

## Security Notes

- The Gemini API key is exposed client-side for this demo app
- Production apps should proxy embedding requests through a backend
- The app uses the anon key which respects RLS policies

## Vector Search RPC Call

```typescript
const { data, error } = await supabase.rpc('vector_search', {
  table_name: 'scripts',
  embedding_column: 'embedding',
  query_embedding: queryVector,  // 768-dimension array
  match_count: 10,
  match_threshold: 0.5,
  metric: 'cosine',
  select_columns: ['id', 'character', 'dialogue', 'seid', 'season']
});
```

Returns array of results with `similarity` score appended to each row.
