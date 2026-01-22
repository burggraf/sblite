import { useState, useEffect } from 'react'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Wifi, WifiOff } from 'lucide-react'

interface AppHeaderProps {
  username: string
  onUsernameChange: (username: string) => void
  isConnected: boolean
}

const STORAGE_KEY = 'realtime-demo-username'

export function AppHeader({ username, onUsernameChange, isConnected }: AppHeaderProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [editValue, setEditValue] = useState(username)

  // Load username from localStorage on mount
  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      onUsernameChange(saved)
      setEditValue(saved)
    }
  }, [onUsernameChange])

  const handleSave = () => {
    if (editValue.trim()) {
      const newUsername = editValue.trim()
      onUsernameChange(newUsername)
      localStorage.setItem(STORAGE_KEY, newUsername)
    } else {
      setEditValue(username)
    }
    setIsEditing(false)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSave()
    } else if (e.key === 'Escape') {
      setEditValue(username)
      setIsEditing(false)
    }
  }

  return (
    <header className="h-14 border-b bg-card flex items-center justify-between px-4">
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-semibold">sblite Realtime Demo</h1>
      </div>

      <div className="flex items-center gap-4">
        {/* Username */}
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Username:</span>
          {isEditing ? (
            <Input
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onBlur={handleSave}
              onKeyDown={handleKeyDown}
              className="h-7 w-32"
              autoFocus
            />
          ) : (
            <button
              onClick={() => setIsEditing(true)}
              className="text-sm font-medium hover:underline"
            >
              {username || 'Click to set'}
            </button>
          )}
        </div>

        {/* Connection status */}
        <Badge variant={isConnected ? 'success' : 'destructive'} className="gap-1">
          {isConnected ? (
            <>
              <Wifi className="h-3 w-3" />
              Connected
            </>
          ) : (
            <>
              <WifiOff className="h-3 w-3" />
              Disconnected
            </>
          )}
        </Badge>
      </div>
    </header>
  )
}
