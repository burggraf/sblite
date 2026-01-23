# Vector-Feld

A demo app showcasing sblite's pgvector-compatible vector similarity search using Seinfeld scripts.

Search for topics like "social anxiety", "food obsession", or "relationship problems" to find semantically similar dialogue lines from the show.

## Prerequisites

- Node.js 18+
- sblite binary
- Google Gemini API key (free tier works)

## Setup

### 1. Start sblite

```bash
# From the sblite root directory
./sblite init --db test_apps/vector-feld/data.db
./sblite db push --db test_apps/vector-feld/data.db --migrations-dir test_apps/vector-feld/migrations
./sblite serve --db test_apps/vector-feld/data.db
```

### 2. Install dependencies

```bash
cd test_apps/vector-feld
pnpm install
```

### 3. Configure environment

```bash
cp .env.example .env.local
```

Edit `.env.local`:
- Get your anon key from the sblite dashboard at http://localhost:8080/_
- Get a Gemini API key from https://aistudio.google.com/apikey

```
VITE_SUPABASE_URL=http://localhost:8080
VITE_SUPABASE_ANON_KEY=<your-anon-key>
VITE_GOOGLE_API_KEY=<your-gemini-key>
GOOGLE_API_KEY=<your-gemini-key>
```

### 4. Import Seinfeld scripts

```bash
pnpm import
```

This downloads ~1,100 dialogue lines from the Seinfeld scripts dataset.

### 5. Generate embeddings

```bash
pnpm embed
```

This generates vector embeddings for each dialogue line using Gemini text-embedding-004.

**Note:** With the free tier rate limit (15 req/min), this takes ~80 minutes for ~1,100 rows. The script is resumable - if interrupted, just run it again.

### 6. Run the app

```bash
pnpm dev
```

Open http://localhost:5173 to use the app.

## How It Works

1. User enters a search topic
2. App generates a vector embedding of the query using Gemini
3. App calls `supabase.rpc('vector_search', {...})` to find similar dialogue
4. Results are displayed as cards sorted by similarity score

## Tech Stack

- React 19 + Vite
- Tailwind CSS 4 + shadcn/ui
- @supabase/supabase-js
- @google/generative-ai (text-embedding-004)
- sblite with vector search
