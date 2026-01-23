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
