# Vector-Feld Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a React/Vite demo app that searches Seinfeld scripts using sblite's vector similarity search.

**Architecture:** Client-side React app with shadcn UI. Two Node.js scripts for data pipeline (import CSV, generate embeddings). Gemini text-embedding-004 for vectors. Search via `supabase.rpc('vector_search', {...})`.

**Tech Stack:** React 19, Vite, Tailwind CSS 4, shadcn/ui, @supabase/supabase-js, @google/generative-ai, csv-parse, tsx

---

## Task 1: Project Scaffolding

**Files:**
- Create: `test_apps/vector-feld/package.json`
- Create: `test_apps/vector-feld/vite.config.ts`
- Create: `test_apps/vector-feld/tsconfig.json`
- Create: `test_apps/vector-feld/tsconfig.app.json`
- Create: `test_apps/vector-feld/tsconfig.node.json`
- Create: `test_apps/vector-feld/index.html`
- Create: `test_apps/vector-feld/components.json`
- Create: `test_apps/vector-feld/.gitignore`
- Create: `test_apps/vector-feld/.env.example`

**Step 1: Create package.json**

```json
{
  "name": "vector-feld",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "import": "tsx scripts/import-scripts.ts",
    "embed": "tsx scripts/generate-embeddings.ts"
  },
  "dependencies": {
    "@google/generative-ai": "^0.24.0",
    "@radix-ui/react-slot": "^1.2.4",
    "@supabase/supabase-js": "^2.91.0",
    "@tailwindcss/vite": "^4.1.17",
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "csv-parse": "^5.6.0",
    "lucide-react": "^0.562.0",
    "react": "^19.2.0",
    "react-dom": "^19.2.0",
    "tailwind-merge": "^3.4.0",
    "tailwindcss": "^4.1.17"
  },
  "devDependencies": {
    "@types/node": "^24.10.1",
    "@types/react": "^19.2.5",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^5.1.1",
    "tsx": "^4.19.0",
    "typescript": "~5.9.3",
    "vite": "^7.2.4"
  }
}
```

**Step 2: Create vite.config.ts**

```typescript
import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
})
```

**Step 3: Create tsconfig.json**

```json
{
  "files": [],
  "references": [
    { "path": "./tsconfig.app.json" },
    { "path": "./tsconfig.node.json" }
  ],
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  }
}
```

**Step 4: Create tsconfig.app.json**

```json
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "types": ["vite/client"],
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "verbatimModuleSyntax": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "erasableSyntaxOnly": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedSideEffectImports": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"]
}
```

**Step 5: Create tsconfig.node.json**

```json
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.node.tsbuildinfo",
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "verbatimModuleSyntax": true,
    "moduleDetection": "force",
    "noEmit": true,
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "erasableSyntaxOnly": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedSideEffectImports": true
  },
  "include": ["vite.config.ts", "scripts/**/*.ts"]
}
```

**Step 6: Create index.html**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Vector-Feld</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

**Step 7: Create components.json**

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "new-york",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/index.css",
    "baseColor": "neutral",
    "cssVariables": true,
    "prefix": ""
  },
  "iconLibrary": "lucide",
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  }
}
```

**Step 8: Create .gitignore**

```
node_modules
dist
*.local
.env.local
data.db
data.db-shm
data.db-wal
```

**Step 9: Create .env.example**

```
VITE_SUPABASE_URL=http://localhost:8080
VITE_SUPABASE_ANON_KEY=your-anon-key
GOOGLE_API_KEY=your-gemini-api-key
```

**Step 10: Commit**

```bash
git add test_apps/vector-feld/
git commit -m "feat(vector-feld): scaffold project structure"
```

---

## Task 2: Core Source Files

**Files:**
- Create: `test_apps/vector-feld/src/main.tsx`
- Create: `test_apps/vector-feld/src/index.css`
- Create: `test_apps/vector-feld/src/lib/utils.ts`
- Create: `test_apps/vector-feld/src/lib/supabase.ts`
- Create: `test_apps/vector-feld/src/lib/gemini.ts`
- Create: `test_apps/vector-feld/src/vite-env.d.ts`

**Step 1: Create src/main.tsx**

```tsx
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import App from "./App.tsx"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
)
```

**Step 2: Create src/index.css**

Copy exact CSS from `test_apps/chat/src/index.css` (lines 1-123, excluding the float-up animation).

**Step 3: Create src/lib/utils.ts**

```typescript
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

**Step 4: Create src/lib/supabase.ts**

```typescript
import { createClient } from "@supabase/supabase-js"

export const supabase = createClient(
  import.meta.env.VITE_SUPABASE_URL!,
  import.meta.env.VITE_SUPABASE_ANON_KEY!
)
```

**Step 5: Create src/lib/gemini.ts**

```typescript
import { GoogleGenerativeAI } from "@google/generative-ai"

const genAI = new GoogleGenerativeAI(import.meta.env.VITE_GOOGLE_API_KEY || "")

export async function embedText(text: string): Promise<number[]> {
  const model = genAI.getGenerativeModel({ model: "text-embedding-004" })
  const result = await model.embedContent(text)
  return result.embedding.values
}
```

**Step 6: Create src/vite-env.d.ts**

```typescript
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_SUPABASE_URL: string
  readonly VITE_SUPABASE_ANON_KEY: string
  readonly VITE_GOOGLE_API_KEY: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
```

**Step 7: Commit**

```bash
git add test_apps/vector-feld/src/
git commit -m "feat(vector-feld): add core source files and lib setup"
```

---

## Task 3: UI Components

**Files:**
- Create: `test_apps/vector-feld/src/components/ui/button.tsx`
- Create: `test_apps/vector-feld/src/components/ui/input.tsx`
- Create: `test_apps/vector-feld/src/components/ui/card.tsx`
- Create: `test_apps/vector-feld/src/components/ui/badge.tsx`
- Create: `test_apps/vector-feld/src/components/ui/scroll-area.tsx`

**Step 1: Copy UI components from chat app**

Copy these files exactly from `test_apps/chat/src/components/ui/`:
- `button.tsx`
- `input.tsx`
- `card.tsx`
- `badge.tsx`
- `scroll-area.tsx`

**Step 2: Commit**

```bash
git add test_apps/vector-feld/src/components/ui/
git commit -m "feat(vector-feld): add shadcn UI components"
```

---

## Task 4: Search Components

**Files:**
- Create: `test_apps/vector-feld/src/components/search-input.tsx`
- Create: `test_apps/vector-feld/src/components/result-card.tsx`
- Create: `test_apps/vector-feld/src/components/results-list.tsx`

**Step 1: Create search-input.tsx**

```tsx
import { useState } from "react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Search, Loader2 } from "lucide-react"

interface SearchInputProps {
  onSearch: (query: string) => void
  isLoading: boolean
}

export function SearchInput({ onSearch, isLoading }: SearchInputProps) {
  const [query, setQuery] = useState("")

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      onSearch(query.trim())
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex gap-2">
      <Input
        type="text"
        placeholder="Enter a topic to search Seinfeld scripts..."
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        className="flex-1"
        disabled={isLoading}
      />
      <Button type="submit" disabled={isLoading || !query.trim()}>
        {isLoading ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : (
          <Search className="h-4 w-4" />
        )}
        Search
      </Button>
    </form>
  )
}
```

**Step 2: Create result-card.tsx**

```tsx
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

export interface ScriptResult {
  id: number
  character: string
  dialogue: string
  seid: string
  season: number
  similarity: number
}

interface ResultCardProps {
  result: ScriptResult
}

export function ResultCard({ result }: ResultCardProps) {
  const similarityPercent = Math.round(result.similarity * 100)

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-2">
              <span className="font-semibold text-sm uppercase tracking-wide">
                {result.character}
              </span>
              <Badge variant="outline" className="text-xs">
                {result.seid}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground leading-relaxed">
              "{result.dialogue}"
            </p>
          </div>
          <Badge
            variant={similarityPercent >= 80 ? "default" : "secondary"}
            className="shrink-0"
          >
            {similarityPercent}%
          </Badge>
        </div>
      </CardContent>
    </Card>
  )
}
```

**Step 3: Create results-list.tsx**

```tsx
import { ScrollArea } from "@/components/ui/scroll-area"
import { ResultCard, type ScriptResult } from "./result-card"

interface ResultsListProps {
  results: ScriptResult[]
  query: string
}

export function ResultsList({ results, query }: ResultsListProps) {
  if (results.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        No results found for "{query}"
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <div className="text-sm text-muted-foreground">
        {results.length} results for "{query}"
      </div>
      <ScrollArea className="h-[calc(100vh-280px)]">
        <div className="space-y-3 pr-4">
          {results.map((result) => (
            <ResultCard key={result.id} result={result} />
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}
```

**Step 4: Commit**

```bash
git add test_apps/vector-feld/src/components/
git commit -m "feat(vector-feld): add search UI components"
```

---

## Task 5: Main App Component

**Files:**
- Create: `test_apps/vector-feld/src/App.tsx`

**Step 1: Create App.tsx**

```tsx
import { useState } from "react"
import { SearchInput } from "@/components/search-input"
import { ResultsList, type ScriptResult } from "@/components/results-list"
import { supabase } from "@/lib/supabase"
import { embedText } from "@/lib/gemini"
import { Film } from "lucide-react"

type SearchState = "idle" | "embedding" | "searching"

export function App() {
  const [results, setResults] = useState<ScriptResult[]>([])
  const [searchState, setSearchState] = useState<SearchState>("idle")
  const [lastQuery, setLastQuery] = useState("")
  const [error, setError] = useState<string | null>(null)

  const handleSearch = async (query: string) => {
    setError(null)
    setLastQuery(query)

    try {
      // Step 1: Generate embedding for the query
      setSearchState("embedding")
      const queryEmbedding = await embedText(query)

      // Step 2: Search for similar scripts
      setSearchState("searching")
      const { data, error: searchError } = await supabase.rpc("vector_search", {
        table_name: "scripts",
        embedding_column: "embedding",
        query_embedding: queryEmbedding,
        match_count: 20,
        match_threshold: 0.3,
        metric: "cosine",
        select_columns: ["id", "character", "dialogue", "seid", "season"],
      })

      if (searchError) {
        throw new Error(searchError.message)
      }

      setResults(data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : "Search failed")
      setResults([])
    } finally {
      setSearchState("idle")
    }
  }

  const isLoading = searchState !== "idle"
  const statusText =
    searchState === "embedding"
      ? "Generating embedding..."
      : searchState === "searching"
        ? "Searching scripts..."
        : null

  return (
    <div className="min-h-screen bg-background">
      <div className="container max-w-3xl mx-auto py-8 px-4">
        {/* Header */}
        <div className="text-center mb-8">
          <div className="flex items-center justify-center gap-2 mb-2">
            <Film className="h-8 w-8" />
            <h1 className="text-3xl font-bold">Vector-Feld</h1>
          </div>
          <p className="text-muted-foreground">
            Search Seinfeld scripts by topic using vector similarity
          </p>
        </div>

        {/* Search */}
        <div className="mb-6">
          <SearchInput onSearch={handleSearch} isLoading={isLoading} />
          {statusText && (
            <p className="text-sm text-muted-foreground mt-2">{statusText}</p>
          )}
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 p-4 bg-destructive/10 text-destructive rounded-lg text-sm">
            {error}
          </div>
        )}

        {/* Results */}
        {lastQuery && !error && (
          <ResultsList results={results} query={lastQuery} />
        )}

        {/* Empty state */}
        {!lastQuery && !error && (
          <div className="text-center py-12 text-muted-foreground">
            <p>Try searching for topics like:</p>
            <p className="mt-2 text-sm">
              "social anxiety" • "food obsession" • "relationship problems" •
              "workplace complaints"
            </p>
          </div>
        )}
      </div>
    </div>
  )
}

export default App
```

**Step 2: Commit**

```bash
git add test_apps/vector-feld/src/App.tsx
git commit -m "feat(vector-feld): add main App component with search logic"
```

---

## Task 6: Database Migration

**Files:**
- Create: `test_apps/vector-feld/migrations/20260122000000_create_scripts.sql`

**Step 1: Create migration file**

```sql
-- Create scripts table for Seinfeld dialogue
CREATE TABLE scripts (
  id INTEGER PRIMARY KEY,
  character TEXT NOT NULL,
  dialogue TEXT NOT NULL,
  episode_no INTEGER,
  seid TEXT,
  season INTEGER,
  embedding TEXT
);

-- Indexes for common queries
CREATE INDEX idx_scripts_character ON scripts(character);
CREATE INDEX idx_scripts_season ON scripts(season);
CREATE INDEX idx_scripts_seid ON scripts(seid);

-- Register embedding column as vector(768) for Gemini text-embedding-004
INSERT INTO _columns (table_name, column_name, column_type)
VALUES ('scripts', 'embedding', 'vector(768)');
```

**Step 2: Commit**

```bash
git add test_apps/vector-feld/migrations/
git commit -m "feat(vector-feld): add scripts table migration"
```

---

## Task 7: Import Script

**Files:**
- Create: `test_apps/vector-feld/scripts/import-scripts.ts`

**Step 1: Create import script**

```typescript
import { createClient } from "@supabase/supabase-js"
import { parse } from "csv-parse/sync"

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || "http://localhost:8080"
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || ""
const CSV_URL =
  "https://media.githubusercontent.com/media/burggraf/datasets/refs/heads/main/seinfeld/scripts.csv"

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)

interface CsvRow {
  ID: string
  Character: string
  Dialogue: string
  EpisodeNo: string
  SEID: string
  Season: string
}

async function main() {
  console.log("Fetching Seinfeld scripts CSV...")
  const response = await fetch(CSV_URL)
  if (!response.ok) {
    throw new Error(`Failed to fetch CSV: ${response.statusText}`)
  }
  const csvText = await response.text()

  console.log("Parsing CSV...")
  const records: CsvRow[] = parse(csvText, {
    columns: true,
    skip_empty_lines: true,
    trim: true,
  })

  console.log(`Found ${records.length} dialogue lines`)

  // Filter out empty dialogue
  const validRecords = records.filter((r) => r.Dialogue && r.Dialogue.trim())
  console.log(`${validRecords.length} valid records after filtering`)

  // Transform to database format
  const rows = validRecords.map((r) => ({
    id: parseInt(r.ID, 10),
    character: r.Character,
    dialogue: r.Dialogue,
    episode_no: r.EpisodeNo ? Math.round(parseFloat(r.EpisodeNo)) : null,
    seid: r.SEID || null,
    season: r.Season ? Math.round(parseFloat(r.Season)) : null,
  }))

  // Batch insert in chunks of 100
  const BATCH_SIZE = 100
  let inserted = 0

  for (let i = 0; i < rows.length; i += BATCH_SIZE) {
    const batch = rows.slice(i, i + BATCH_SIZE)
    const { error } = await supabase.from("scripts").insert(batch)

    if (error) {
      console.error(`Error inserting batch at ${i}:`, error.message)
      throw error
    }

    inserted += batch.length
    console.log(`Imported ${inserted}/${rows.length} rows`)
  }

  console.log("Import complete!")
}

main().catch((err) => {
  console.error("Import failed:", err)
  process.exit(1)
})
```

**Step 2: Commit**

```bash
git add test_apps/vector-feld/scripts/import-scripts.ts
git commit -m "feat(vector-feld): add CSV import script"
```

---

## Task 8: Embedding Generation Script

**Files:**
- Create: `test_apps/vector-feld/scripts/generate-embeddings.ts`

**Step 1: Create embedding script**

```typescript
import { createClient } from "@supabase/supabase-js"
import { GoogleGenerativeAI } from "@google/generative-ai"

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || "http://localhost:8080"
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || ""
const GOOGLE_API_KEY = process.env.GOOGLE_API_KEY || ""

if (!GOOGLE_API_KEY) {
  console.error("GOOGLE_API_KEY environment variable is required")
  process.exit(1)
}

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)
const genAI = new GoogleGenerativeAI(GOOGLE_API_KEY)

// Rate limiting: Gemini free tier is 15 requests/minute
const RATE_LIMIT_DELAY = 4500 // 4.5 seconds between requests (safe margin)

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function embedWithRetry(
  text: string,
  maxRetries = 3
): Promise<number[]> {
  const model = genAI.getGenerativeModel({ model: "text-embedding-004" })

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      const result = await model.embedContent(text)
      return result.embedding.values
    } catch (error) {
      const err = error as Error & { status?: number }
      if (err.status === 429 && attempt < maxRetries) {
        // Rate limited, wait longer
        const waitTime = RATE_LIMIT_DELAY * attempt * 2
        console.log(`Rate limited, waiting ${waitTime / 1000}s...`)
        await sleep(waitTime)
        continue
      }
      throw error
    }
  }

  throw new Error("Max retries exceeded")
}

async function main() {
  // Get count of rows needing embeddings
  const { count: totalCount } = await supabase
    .from("scripts")
    .select("*", { count: "exact", head: true })
    .is("embedding", null)

  if (!totalCount) {
    console.log("All rows already have embeddings!")
    return
  }

  console.log(`${totalCount} rows need embeddings`)

  let processed = 0
  const startTime = Date.now()

  while (true) {
    // Fetch next batch of rows without embeddings
    const { data: rows, error: fetchError } = await supabase
      .from("scripts")
      .select("id, dialogue")
      .is("embedding", null)
      .order("id")
      .limit(10)

    if (fetchError) {
      console.error("Fetch error:", fetchError.message)
      throw fetchError
    }

    if (!rows || rows.length === 0) {
      break
    }

    for (const row of rows) {
      try {
        // Generate embedding
        const embedding = await embedWithRetry(row.dialogue)

        // Update row with embedding
        const { error: updateError } = await supabase
          .from("scripts")
          .update({ embedding: JSON.stringify(embedding) })
          .eq("id", row.id)

        if (updateError) {
          console.error(`Update error for row ${row.id}:`, updateError.message)
          continue
        }

        processed++
        const elapsed = (Date.now() - startTime) / 1000
        const rate = processed / elapsed
        const remaining = totalCount - processed
        const eta = remaining / rate

        console.log(
          `Embedded ${processed}/${totalCount} (${Math.round((processed / totalCount) * 100)}%) ` +
            `- ETA: ${Math.round(eta / 60)}m`
        )

        // Rate limiting delay
        await sleep(RATE_LIMIT_DELAY)
      } catch (error) {
        console.error(`Error processing row ${row.id}:`, error)
        // Continue with next row
      }
    }
  }

  const totalTime = (Date.now() - startTime) / 1000 / 60
  console.log(`\nComplete! Processed ${processed} rows in ${totalTime.toFixed(1)} minutes`)
}

main().catch((err) => {
  console.error("Embedding generation failed:", err)
  process.exit(1)
})
```

**Step 2: Commit**

```bash
git add test_apps/vector-feld/scripts/generate-embeddings.ts
git commit -m "feat(vector-feld): add embedding generation script"
```

---

## Task 9: README Documentation

**Files:**
- Create: `test_apps/vector-feld/README.md`

**Step 1: Create README**

```markdown
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
```

**Step 2: Commit**

```bash
git add test_apps/vector-feld/README.md
git commit -m "docs(vector-feld): add README with setup instructions"
```

---

## Task 10: Install Dependencies and Verify

**Step 1: Install dependencies**

```bash
cd test_apps/vector-feld && pnpm install
```

Expected: Dependencies install successfully

**Step 2: Run type check**

```bash
cd test_apps/vector-feld && pnpm exec tsc --noEmit
```

Expected: No type errors

**Step 3: Run build**

```bash
cd test_apps/vector-feld && pnpm build
```

Expected: Build succeeds

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat(vector-feld): complete vector search demo app"
```
