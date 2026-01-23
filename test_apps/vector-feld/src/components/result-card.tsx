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

export interface EpisodeInfo {
  title: string
  air_date: string | null
}

interface ResultCardProps {
  result: ScriptResult
  episodeInfo?: EpisodeInfo | null
  onClick?: () => void
}

export function ResultCard({ result, episodeInfo, onClick }: ResultCardProps) {
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
            <div className="flex items-center gap-2 mb-1 flex-wrap">
              <span className="font-semibold text-sm uppercase tracking-wide">
                {result.character}
              </span>
              <Badge variant="outline" className="text-xs">
                {result.seid}
              </Badge>
              {episodeInfo && (
                <>
                  <span className="text-xs text-muted-foreground">
                    "{episodeInfo.title}"
                  </span>
                  {episodeInfo.air_date && (
                    <span className="text-xs text-muted-foreground">
                      ({episodeInfo.air_date})
                    </span>
                  )}
                </>
              )}
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
