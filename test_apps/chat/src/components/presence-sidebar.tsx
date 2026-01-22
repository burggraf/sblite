import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { Users } from 'lucide-react'
import type { PresenceUser } from '@/hooks/use-presence'

interface PresenceSidebarProps {
  users: PresenceUser[]
  currentUserId: string
}

export function PresenceSidebar({ users, currentUserId }: PresenceSidebarProps) {
  return (
    <div className="w-48 border-l bg-muted/30 flex flex-col">
      <div className="p-3 border-b flex items-center gap-2">
        <Users className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium">Online</span>
        <Badge variant="secondary" className="ml-auto text-xs">
          {users.length}
        </Badge>
      </div>

      <ScrollArea className="flex-1 p-2">
        <div className="space-y-1">
          {users.map((user) => (
            <div
              key={user.id}
              className="flex items-center gap-2 p-2 rounded-md hover:bg-accent/50 transition-colors"
            >
              <div
                className="w-2 h-2 rounded-full animate-pulse"
                style={{ backgroundColor: user.color }}
              />
              <span className="text-sm truncate flex-1">
                {user.username}
                {user.id === currentUserId && (
                  <span className="text-muted-foreground ml-1">(you)</span>
                )}
              </span>
            </div>
          ))}

          {users.length === 0 && (
            <p className="text-sm text-muted-foreground text-center py-4">
              No users online
            </p>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
