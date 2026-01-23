# Vector-Feld

A demo app showcasing sblite's pgvector-compatible vector similarity search using Seinfeld scripts.

Search for topics like "social anxiety", "food obsession", or "relationship problems" to find semantically similar dialogue lines from the show.

## Prerequisites

- Node.js 18+
- sblite binary
- Google Gemini API key (free tier works)

## Setup

### 1. Install dependencies

```bash
cd test_apps/vector-feld
pnpm install
```

### 2. Configure environment

```bash
cp .env.example .env.local
```

Edit `.env.local`:
- Get your anon key from the sblite dashboard at http://localhost:8080/_
- Get a Gemini API key from https://aistudio.google.com/apikey

```
VITE_SUPABASE_URL=http://localhost:8080
VITE_SUPABASE_ANON_KEY=<your-anon-key>
GOOGLE_API_KEY=<your-gemini-key>
```

### 3. Initialize the database

```bash
# From the sblite root directory
./sblite init --db test_apps/vector-feld/data.db
./sblite db push --db test_apps/vector-feld/data.db --migrations-dir test_apps/vector-feld/migrations
```

This creates two tables:
- `scripts` - Dialogue lines with vector embeddings
- `episode_info` - Episode metadata (title, air date, writers, director)

### 4. Set up the embedding edge function

The app uses an edge function to generate embeddings server-side, keeping your API key secure.

```bash
# Set the Google API key as a secret (will prompt for value)
./sblite functions secrets set GOOGLE_API_KEY --db test_apps/vector-feld/data.db

# Disable JWT verification for the embed function (allows anonymous search)
./sblite functions config set-jwt embed disabled --db test_apps/vector-feld/data.db
```

### 5. Start sblite with edge functions enabled

```bash
./sblite serve --db test_apps/vector-feld/data.db --functions --functions-dir test_apps/vector-feld/functions
```

### 6. Import data

In a new terminal:

```bash
cd test_apps/vector-feld

# Import ~54,600 dialogue lines
pnpm import

# Import 174 episode info records
pnpm import:episodes
```

### 7. Generate embeddings

```bash
pnpm embed
```

This generates vector embeddings for each dialogue line using Gemini text-embedding-004.

The script uses batch embedding (100 texts per API call) and processes ~5,400 rows/minute. For ~54,600 rows, expect completion in ~10 minutes.

The script is resumable - if interrupted, just run it again to continue where it left off.

### 8. Run the app

```bash
pnpm dev
```

Open http://localhost:5173 to use the app.

## Features

- **Semantic Search**: Search by topic/concept rather than exact keywords
- **Episode Info**: Each result shows the episode title and air date
- **Context Modal**: Click any result to see surrounding dialogue lines from the same episode, with the matched line highlighted
- **Episode Details**: The context modal shows episode title, air date, director, and writers

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Browser                                    │
│  ┌─────────────┐    ┌──────────────┐    ┌────────────────────────┐  │
│  │ Search Input │───▶│ embedText()  │───▶│ supabase.functions     │  │
│  └─────────────┘    └──────────────┘    │   .invoke("embed")     │  │
│                                          └───────────┬────────────┘  │
└──────────────────────────────────────────────────────┼───────────────┘
                                                       │
                                                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           sblite                                     │
│  ┌─────────────────────┐    ┌────────────────────────────────────┐  │
│  │ /functions/v1/embed │───▶│ Edge Function (Deno)               │  │
│  │                     │    │ - Reads GOOGLE_API_KEY secret      │  │
│  │                     │    │ - Calls Gemini embedding API       │  │
│  │                     │◀───│ - Returns embedding vector         │  │
│  └─────────────────────┘    └────────────────────────────────────┘  │
│                                                                      │
│  ┌─────────────────────┐    ┌────────────────────────────────────┐  │
│  │ /rest/v1/rpc/       │───▶│ vector_search()                    │  │
│  │   vector_search     │    │ - Computes cosine similarity       │  │
│  │                     │◀───│ - Returns top N matches            │  │
│  └─────────────────────┘    └────────────────────────────────────┘  │
│                                                                      │
│  ┌─────────────────────┐    ┌────────────────────────────────────┐  │
│  │ /rest/v1/           │───▶│ episode_info table                 │  │
│  │   episode_info      │    │ - Title, air date, writers, etc.   │  │
│  │                     │◀───│ - Indexed by seid for fast lookup  │  │
│  └─────────────────────┘    └────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                                                       │
                                                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Gemini API                                      │
│  text-embedding-004 model (768 dimensions)                          │
└─────────────────────────────────────────────────────────────────────┘
```

## How It Works

1. User enters a search topic in the browser
2. Frontend calls the `embed` edge function via `supabase.functions.invoke()`
3. Edge function securely calls Gemini API with server-side API key
4. Embedding vector is returned to the browser
5. Frontend calls `supabase.rpc('vector_search', {...})` with the embedding
6. sblite computes cosine similarity against all stored vectors
7. Top matches are returned and displayed as cards with episode info
8. Clicking a result fetches surrounding dialogue and episode details

## Database Schema

### scripts
| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| character | TEXT | Character name |
| dialogue | TEXT | The dialogue line |
| episode_no | INTEGER | Episode number within season |
| seid | TEXT | Season/episode ID (e.g., S02E11) |
| season | INTEGER | Season number |
| embedding | VECTOR(768) | Gemini embedding vector |

### episode_info
| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| season | INTEGER | Season number |
| episode_no | INTEGER | Episode number within season |
| title | TEXT | Episode title |
| air_date | TEXT | Original air date |
| writers | TEXT | Episode writers |
| director | TEXT | Episode director |
| seid | TEXT | Season/episode ID (indexed) |

## Edge Function

The `embed` function (`functions/embed/index.ts`) handles embedding generation server-side:

- Accepts `{ text: string }` in the request body
- Uses `GOOGLE_API_KEY` secret (never exposed to browser)
- Calls Gemini `text-embedding-004` with `taskType: "RETRIEVAL_QUERY"`
- Returns `{ embedding: number[] }` (768 dimensions)

## Scripts

| Script | Command | Description |
|--------|---------|-------------|
| import | `pnpm import` | Import dialogue lines from scripts.csv |
| import:episodes | `pnpm import:episodes` | Import episode info from episode_info.csv |
| embed | `pnpm embed` | Generate embeddings for all dialogue lines |
| dev | `pnpm dev` | Start development server |
| build | `pnpm build` | Build for production |

## Tech Stack

- React 19 + Vite
- Tailwind CSS 4 + shadcn/ui
- @supabase/supabase-js
- sblite with vector search and edge functions
- Gemini text-embedding-004 (via edge function)

## RAM & Performance

- **Disk storage**: ~475 MB for 54k vectors (768 dims as JSON)
- **RAM during search**: ~50-100 MB peak (vectors are streamed, not loaded all at once)
- **Search method**: Brute-force cosine similarity (no index)
- **Search latency**: ~100-500ms for 54k vectors (depends on hardware)
