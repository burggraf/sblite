import { useEffect, useState } from "react"
import { ScrollArea } from "@/components/ui/scroll-area"
import { ResultCard, type ScriptResult, type EpisodeInfo } from "./result-card"
import { ContextModal } from "./context-modal"
import { supabase } from "@/lib/supabase"

export type { ScriptResult }

interface ResultsListProps {
  results: ScriptResult[]
  query: string
}

export function ResultsList({ results, query }: ResultsListProps) {
  const [selectedResult, setSelectedResult] = useState<ScriptResult | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [episodeInfoMap, setEpisodeInfoMap] = useState<Record<string, EpisodeInfo>>({})

  // Fetch episode info for all unique seids
  useEffect(() => {
    if (results.length === 0) {
      setEpisodeInfoMap({})
      return
    }

    const uniqueSeids = [...new Set(results.map((r) => r.seid))]

    async function fetchEpisodeInfo() {
      const { data, error } = await supabase
        .from("episode_info")
        .select("seid, title, air_date")
        .in("seid", uniqueSeids)

      if (error) {
        console.error("Error fetching episode info:", error)
        return
      }

      const infoMap: Record<string, EpisodeInfo> = {}
      for (const ep of data || []) {
        infoMap[ep.seid] = { title: ep.title, air_date: ep.air_date }
      }
      setEpisodeInfoMap(infoMap)
    }

    fetchEpisodeInfo()
  }, [results])

  const handleResultClick = (result: ScriptResult) => {
    setSelectedResult(result)
    setModalOpen(true)
  }

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
            <ResultCard
              key={result.id}
              result={result}
              episodeInfo={episodeInfoMap[result.seid]}
              onClick={() => handleResultClick(result)}
            />
          ))}
        </div>
      </ScrollArea>

      <ContextModal
        result={selectedResult}
        open={modalOpen}
        onOpenChange={setModalOpen}
      />
    </div>
  )
}
