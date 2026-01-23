import { useState } from "react"
import { ScrollArea } from "@/components/ui/scroll-area"
import { ResultCard, type ScriptResult } from "./result-card"
import { ContextModal } from "./context-modal"

export type { ScriptResult }

interface ResultsListProps {
  results: ScriptResult[]
  query: string
}

export function ResultsList({ results, query }: ResultsListProps) {
  const [selectedResult, setSelectedResult] = useState<ScriptResult | null>(null)
  const [modalOpen, setModalOpen] = useState(false)

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
