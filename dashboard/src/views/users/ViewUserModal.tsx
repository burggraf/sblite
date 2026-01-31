/**
 * View/Edit User Modal
 */

import { useState } from 'react'
import { useToast } from '@/hooks'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import type { User } from '@/lib/api-client'

interface ViewUserModalProps {
  user: User
  onClose: () => void
  onUpdated: () => void
}

export function ViewUserModal({ user, onClose, onUpdated }: ViewUserModalProps) {
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Form state
  const [emailConfirmed, setEmailConfirmed] = useState(!!user.email_confirmed_at)
  const [userMetadata, setUserMetadata] = useState(
    JSON.stringify(user.user_metadata || {}, null, 2)
  )
  const [metadataError, setMetadataError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setMetadataError('')

    // Validate JSON metadata
    let metadata: Record<string, unknown> = {}
    if (userMetadata) {
      try {
        metadata = JSON.parse(userMetadata)
      } catch {
        setMetadataError('Invalid JSON format')
        return
      }
    }

    setLoading(true)
    try {
      const res = await fetch(`/_/api/users/${user.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email_confirmed: emailConfirmed,
          raw_user_meta_data: userMetadata,
        }),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error || 'Failed to update user')
      }
      success('User updated', `User has been updated successfully.`)
      onUpdated()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update user')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div className="bg-background rounded-lg shadow-lg w-full max-w-md p-6 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold">User Details</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="id">ID</Label>
            <Input
              id="id"
              value={user.id}
              disabled
              className="bg-muted"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              value={user.email || '(anonymous)'}
              disabled
              className="bg-muted"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="created">Created</Label>
            <Input
              id="created"
              value={user.created_at ? new Date(user.created_at).toLocaleString() : '—'}
              disabled
              className="bg-muted"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="lastSignIn">Last Sign In</Label>
            <Input
              id="lastSignIn"
              value={user.last_sign_in_at ? new Date(user.last_sign_in_at).toLocaleString() : 'Never'}
              disabled
              className="bg-muted"
            />
          </div>

          <div className="flex items-center space-x-2">
            <Checkbox
              id="emailConfirmed"
              checked={emailConfirmed}
              onCheckedChange={setEmailConfirmed}
              disabled={loading}
            />
            <Label htmlFor="emailConfirmed" className="text-sm cursor-pointer">
              Email Confirmed
            </Label>
          </div>

          <div className="space-y-2">
            <Label htmlFor="userMetadata">User Metadata (JSON)</Label>
            <Input
              id="userMetadata"
              value={userMetadata}
              onChange={(e) => setUserMetadata(e.target.value)}
              placeholder="{}"
              disabled={loading}
              className="font-mono text-sm"
            />
            {metadataError && (
              <p className="text-sm text-destructive">{metadataError}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="appMetadata">App Metadata (JSON)</Label>
            <Input
              id="appMetadata"
              value={JSON.stringify(user.app_metadata || {}, null, 2)}
              disabled
              className="bg-muted font-mono text-sm"
            />
            <p className="text-xs text-muted-foreground">
              App metadata is read-only
            </p>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Saving...' : 'Save Changes'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
