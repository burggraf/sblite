import { useRef, useCallback } from 'react'
import type { CursorPosition, ReactionEvent } from '@/hooks/use-broadcast'

interface CursorCanvasProps {
  cursors: CursorPosition[]
  reactions: ReactionEvent[]
  onMouseMove: (x: number, y: number) => void
  onReaction: (emoji: string, x: number, y: number) => void
}

const EMOJIS = ['👍', '❤️', '🎉', '🔥', '👀', '💯']

export function CursorCanvas({ cursors, reactions, onMouseMove, onReaction }: CursorCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null)

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!containerRef.current) return
    const rect = containerRef.current.getBoundingClientRect()
    const x = ((e.clientX - rect.left) / rect.width) * 100
    const y = ((e.clientY - rect.top) / rect.height) * 100
    onMouseMove(x, y)
  }, [onMouseMove])

  const handleClick = useCallback((e: React.MouseEvent) => {
    if (!containerRef.current) return
    const rect = containerRef.current.getBoundingClientRect()
    const x = ((e.clientX - rect.left) / rect.width) * 100
    const y = ((e.clientY - rect.top) / rect.height) * 100
    const emoji = EMOJIS[Math.floor(Math.random() * EMOJIS.length)]
    onReaction(emoji, x, y)
  }, [onReaction])

  return (
    <div
      ref={containerRef}
      className="relative w-full h-full bg-gradient-to-br from-background to-muted/50 rounded-lg border overflow-hidden cursor-crosshair"
      onMouseMove={handleMouseMove}
      onClick={handleClick}
    >
      {/* Grid background */}
      <div
        className="absolute inset-0 opacity-10"
        style={{
          backgroundImage: `
            linear-gradient(to right, currentColor 1px, transparent 1px),
            linear-gradient(to bottom, currentColor 1px, transparent 1px)
          `,
          backgroundSize: '40px 40px'
        }}
      />

      {/* Instructions */}
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
        <div className="text-center text-muted-foreground">
          <p className="text-lg font-medium">Move your cursor here</p>
          <p className="text-sm">Other users will see your cursor in realtime</p>
          <p className="text-sm mt-2">Click anywhere to send a reaction!</p>
        </div>
      </div>

      {/* Remote cursors */}
      {cursors.map((cursor) => (
        <Cursor key={cursor.id} cursor={cursor} />
      ))}

      {/* Floating reactions */}
      {reactions.map((reaction, index) => (
        <Reaction key={`${reaction.id}-${index}`} reaction={reaction} />
      ))}

      {/* Emoji palette */}
      <div className="absolute bottom-4 left-1/2 -translate-x-1/2 flex gap-2 bg-background/80 backdrop-blur-sm rounded-full px-4 py-2 border shadow-lg">
        {EMOJIS.map((emoji) => (
          <button
            key={emoji}
            onClick={(e) => {
              e.stopPropagation()
              onReaction(emoji, 50, 50)
            }}
            className="text-xl hover:scale-125 transition-transform"
          >
            {emoji}
          </button>
        ))}
      </div>
    </div>
  )
}

function Cursor({ cursor }: { cursor: CursorPosition }) {
  return (
    <div
      className="absolute pointer-events-none transition-all duration-75"
      style={{
        left: `${cursor.x}%`,
        top: `${cursor.y}%`,
        transform: 'translate(-2px, -2px)'
      }}
    >
      {/* Cursor icon */}
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill={cursor.color}
        className="drop-shadow-md"
      >
        <path d="M5.5 3.21V20.8c0 .45.54.67.85.35l4.86-4.86a.5.5 0 0 1 .35-.15h6.87a.5.5 0 0 0 .35-.85L6.35 2.86a.5.5 0 0 0-.85.35Z" />
      </svg>

      {/* Username label */}
      <div
        className="absolute left-5 top-4 px-2 py-0.5 rounded text-xs text-white whitespace-nowrap shadow-md"
        style={{ backgroundColor: cursor.color }}
      >
        {cursor.username}
      </div>
    </div>
  )
}

function Reaction({ reaction }: { reaction: ReactionEvent }) {
  return (
    <div
      className="absolute pointer-events-none animate-float-up text-2xl"
      style={{
        left: `${reaction.x}%`,
        top: `${reaction.y}%`,
        transform: 'translate(-50%, -50%)'
      }}
    >
      {reaction.emoji}
    </div>
  )
}
