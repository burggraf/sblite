/**
 * Users Page
 * User management with filtering, pagination, and modals
 * Using simple React state (no Zustand)
 */

import { useState, useEffect } from 'react'
import { useToast } from '@/hooks'
import { formatDateTime } from '@/lib/utils'
import type { User } from '@/lib/api-client'
import { CreateUserModal } from './CreateUserModal'
import { InviteUserModal } from './InviteUserModal'
import { ViewUserModal } from './ViewUserModal'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

type UserFilter = 'all' | 'regular' | 'anonymous'

export function UsersPage() {
  const { success, error: showError } = useToast()

  // State
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState<UserFilter>('all')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showInviteModal, setShowInviteModal] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [showViewModal, setShowViewModal] = useState(false)

  // Load users
  const loadUsers = async () => {
    setLoading(true)
    try {
      const offset = (page - 1) * pageSize
      const res = await fetch(`/_/api/users?limit=${pageSize}&offset=${offset}&filter=${filter}`)
      if (!res.ok) throw new Error('Failed to load users')
      const data = await res.json()
      setUsers(data.users)
      setTotal(data.total)
    } catch (err) {
      showError('Error', err instanceof Error ? err.message : 'Failed to load users')
    } finally {
      setLoading(false)
    }
  }

  // Load on filter/pagination change
  useEffect(() => {
    loadUsers()
  }, [filter, page, pageSize])

  const totalPages = Math.ceil(total / pageSize) || 1

  const handleDeleteUser = async (user: User) => {
    if (!confirm(`Are you sure you want to delete user "${user.email || 'anonymous'}"?`)) {
      return
    }

    try {
      const res = await fetch(`/_/api/users/${user.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error('Failed to delete user')
      success('User deleted', `User "${user.email}" has been deleted.`)
      loadUsers()
    } catch {
      showError('Delete failed', 'Failed to delete user. Please try again.')
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">Users</h2>
        <div className="flex items-center gap-4">
          <Select value={filter} onValueChange={(value) => { setFilter(value as UserFilter); setPage(1) }}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Users</SelectItem>
              <SelectItem value="regular">Regular</SelectItem>
              <SelectItem value="anonymous">Anonymous</SelectItem>
            </SelectContent>
          </Select>
          <Button onClick={() => setShowCreateModal(true)}>+ Create User</Button>
          <Button variant="secondary" onClick={() => setShowInviteModal(true)}>Invite User</Button>
          <span className="text-muted-foreground text-sm">
            {total} user{total !== 1 ? 's' : ''}
          </span>
        </div>
      </div>

      {/* Table */}
      {loading ? (
        <div className="flex justify-center py-8">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Email</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Last Sign In</TableHead>
                <TableHead>Confirmed</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                    No users
                  </TableCell>
                </TableRow>
              ) : (
                users.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell>
                      {user.is_anonymous ? (
                        <div className="flex items-center gap-2">
                          <span className="text-muted-foreground">(anonymous)</span>
                          <Badge variant="secondary">Anon</Badge>
                        </div>
                      ) : (
                        user.email || '—'
                      )}
                    </TableCell>
                    <TableCell>{formatDateTime(user.created_at)}</TableCell>
                    <TableCell>{formatDateTime(user.last_sign_in_at)}</TableCell>
                    <TableCell>
                      {user.email_confirmed_at ? (
                        <span className="text-green-500">✓</span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => { setSelectedUser(user); setShowViewModal(true) }}
                      >
                        View
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => handleDeleteUser(user)}
                      >
                        Delete
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <div className="text-sm text-muted-foreground">
          {total} users | Page {page} of {totalPages}
        </div>
        <div className="flex items-center gap-2">
          <Select
            value={pageSize.toString()}
            onValueChange={(value) => { setPageSize(parseInt(value)); setPage(1) }}
          >
            <SelectTrigger className="w-[70px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="25">25</SelectItem>
              <SelectItem value="50">50</SelectItem>
              <SelectItem value="100">100</SelectItem>
            </SelectContent>
          </Select>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
          >
            Prev
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
          >
            Next
          </Button>
        </div>
      </div>

      {/* Modals */}
      {showCreateModal && (
        <CreateUserModal
          onClose={() => setShowCreateModal(false)}
          onCreated={() => { setShowCreateModal(false); loadUsers() }}
        />
      )}
      {showInviteModal && (
        <InviteUserModal
          onClose={() => setShowInviteModal(false)}
          onInvited={() => setShowInviteModal(false)}
        />
      )}
      {showViewModal && selectedUser && (
        <ViewUserModal
          user={selectedUser}
          onClose={() => setShowViewModal(false)}
          onUpdated={() => { setShowViewModal(false); loadUsers() }}
        />
      )}
    </div>
  )
}
