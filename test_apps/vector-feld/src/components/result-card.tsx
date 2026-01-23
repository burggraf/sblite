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
  onClick?: () => void
}

export function ResultCard({ result, onClick }: ResultCardProps) {
  const similarityPercent = Math.round(result.similarity * 100)

  return (
    <Card
      className={onClick ? "cursor-pointer hover:bg-accent/50 transition-colors" : ""}
      onClick={onClick}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={onClick ? (e) => e.key === "Enter" && onClick() : undefined}
    >
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
