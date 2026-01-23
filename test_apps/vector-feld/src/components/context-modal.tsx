import { useEffect, useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { supabase } from "@/lib/supabase"
import type { ScriptResult } from "./result-card"

interface DialogueLine {
  id: number
  character: string
  dialogue: string
}

interface ContextModalProps {
  result: ScriptResult | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

const CONTEXT_LINES = 5 // Number of lines before and after

export function ContextModal({ result, open, onOpenChange }: ContextModalProps) {
  const [lines, setLines] = useState<DialogueLine[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!result || !open) return

    async function fetchContext() {
      setLoading(true)
      setError(null)

      try {
        // Get lines from the same episode, ordered by id
        // We need lines with id around result.id that have the same seid
        const { data, error: queryError } = await supabase
          .from("scripts")
          .select("id, character, dialogue")
          .eq("seid", result!.seid)
          .gte("id", result!.id - CONTEXT_LINES * 10) // Cast wider net since ids may not be sequential within episode
          .lte("id", result!.id + CONTEXT_LINES * 10)
          .order("id", { ascending: true })

        if (queryError) throw queryError

        if (!data || data.length === 0) {
          setLines([{ id: result!.id, character: result!.character, dialogue: result!.dialogue }])
          return
        }

        // Find the target line index and extract context
        const targetIndex = data.findIndex((line) => line.id === result!.id)
        if (targetIndex === -1) {
          // Target not found in result, add it
          setLines([{ id: result!.id, character: result!.character, dialogue: result!.dialogue }])
          return
        }

        // Extract lines before and after, respecting episode boundaries
        const startIndex = Math.max(0, targetIndex - CONTEXT_LINES)
        const endIndex = Math.min(data.length - 1, targetIndex + CONTEXT_LINES)

        setLines(data.slice(startIndex, endIndex + 1))
      } catch (err) {
        console.error("Error fetching context:", err)
        setError(err instanceof Error ? err.message : "Failed to load context")
      } finally {
        setLoading(false)
      }
    }

    fetchContext()
  }, [result, open])

  if (!result) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <span>Episode Context</span>
            <Badge variant="outline">{result.seid}</Badge>
          </DialogTitle>
        </DialogHeader>

        <div className="overflow-y-auto flex-1 -mx-6 px-6">
          {loading && (
            <div className="text-center text-muted-foreground py-8">
              Loading context...
            </div>
          )}

          {error && (
            <div className="text-center text-destructive py-8">
              {error}
            </div>
          )}

          {!loading && !error && (
            <div className="space-y-3">
              {lines.map((line) => {
                const isTarget = line.id === result.id
                return (
                  <div
                    key={line.id}
                    className={`rounded-lg p-3 transition-colors ${
                      isTarget
                        ? "bg-primary/10 border-2 border-primary"
                        : "bg-muted/50"
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <span
                        className={`font-semibold text-xs uppercase tracking-wide ${
                          isTarget ? "text-primary" : "text-muted-foreground"
                        }`}
                      >
                        {line.character}
                      </span>
                      {isTarget && (
                        <Badge variant="default" className="text-xs">
                          Match
                        </Badge>
                      )}
                    </div>
                    <p
                      className={`text-sm leading-relaxed ${
                        isTarget ? "text-foreground" : "text-muted-foreground"
                      }`}
                    >
                      "{line.dialogue}"
                    </p>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
