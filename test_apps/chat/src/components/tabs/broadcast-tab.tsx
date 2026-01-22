import { useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CursorCanvas } from '@/components/cursor-canvas'
import { Badge } from '@/components/ui/badge'
import { useBroadcast, type CursorPosition, type ReactionEvent } from '@/hooks/use-broadcast'
import { Radio, Users } from 'lucide-react'

interface BroadcastTabProps {
  username: string
  onCursor?: (cursor: CursorPosition) => void
  onReaction?: (reaction: ReactionEvent) => void
}

export function BroadcastTab({ username, onCursor, onReaction }: BroadcastTabProps) {
  const { cursors, reactions, sendCursor, sendReaction } = useBroadcast(username, {
    channelName: 'cursor-broadcast',
    onCursor,
    onReaction
  })

  const handleMouseMove = useCallback((x: number, y: number) => {
    sendCursor(x, y)
  }, [sendCursor])

  const handleReaction = useCallback((emoji: string, x: number, y: number) => {
    sendReaction(emoji, x, y)
  }, [sendReaction])

  return (
    <div className="h-full p-4 flex flex-col gap-4">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Radio className="h-4 w-4" />
            Broadcast Demo
            <Badge variant="secondary" className="ml-auto gap-1">
              <Users className="h-3 w-3" />
              {cursors.length} cursor{cursors.length !== 1 ? 's' : ''} visible
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Move your mouse in the canvas below. Other users will see your cursor position in realtime via <code className="bg-muted px-1 rounded">broadcast</code> messages.
            Click to send emoji reactions!
          </p>
        </CardContent>
      </Card>

      <Card className="flex-1 min-h-0">
        <CardContent className="h-full p-4">
          <CursorCanvas
            cursors={cursors}
            reactions={reactions}
            onMouseMove={handleMouseMove}
            onReaction={handleReaction}
          />
        </CardContent>
      </Card>
    </div>
  )
}
