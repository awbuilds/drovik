import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Plus, ShieldCheck, Trash2, Users } from 'lucide-react'
import {
  createUser,
  deleteUser,
  listUsers,
  resetUserPassword,
  updateUser,
  type ManagedUser,
  type UserRole,
} from '@/api/users'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ConfirmDialog } from '@/components/servers/ConfirmDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent } from '@/components/ui/Card'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatDateTime, formatRelativeTime } from '@/lib/format'

const roleLabels: Record<UserRole, string> = {
  admin: 'Administrator',
  operator: 'Operator',
  read_only: 'Read-only',
}

export function UsersPage() {
  const { user, csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<ManagedUser | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ManagedUser | null>(null)
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<UserRole>('operator')
  const [newPassword, setNewPassword] = useState('')

  const usersQuery = useQuery({
    queryKey: ['users'],
    queryFn: ({ signal }) => listUsers(signal),
    enabled: user?.role === 'admin',
    retry: 0,
  })

  if (user?.role !== 'admin') {
    return (
      <Alert variant="warning" title="Administrator access required">
        User management is limited to administrators. Ask an administrator to change your role.
      </Alert>
    )
  }

  const resetForm = () => {
    setUsername('')
    setPassword('')
    setRole('operator')
    setFormError(null)
  }

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ['users'] })
  }

  const handleCreate = async () => {
    setBusy(true)
    setFormError(null)
    try {
      await createUser({ username: username.trim(), password, role }, csrfToken)
      toast({ title: 'User created', description: username.trim(), variant: 'success' })
      setCreateOpen(false)
      resetForm()
      await refresh()
    } catch (caught) {
      setFormError(userFacingError(caught))
    } finally {
      setBusy(false)
    }
  }

  const handleRole = async (target: ManagedUser, nextRole: UserRole) => {
    try {
      await updateUser(target.id, { role: nextRole }, csrfToken)
      toast({ title: 'Role updated', description: `${target.username} · ${roleLabels[nextRole]}`, variant: 'success' })
      await refresh()
    } catch (caught) {
      toast({ title: 'Role update failed', description: userFacingError(caught), variant: 'error' })
    }
  }

  const toggleActive = async (target: ManagedUser) => {
    try {
      await updateUser(target.id, { is_active: !target.is_active }, csrfToken)
      toast({
        title: target.is_active ? 'User disabled' : 'User enabled',
        description: target.username,
        variant: 'success',
      })
      await refresh()
    } catch (caught) {
      toast({ title: 'Update failed', description: userFacingError(caught), variant: 'error' })
    }
  }

  const handleReset = async () => {
    if (!resetTarget) return
    setBusy(true)
    setFormError(null)
    try {
      await resetUserPassword(resetTarget.id, newPassword, csrfToken)
      toast({ title: 'Password reset', description: `${resetTarget.username}'s sessions were revoked.`, variant: 'success' })
      setResetTarget(null)
      setNewPassword('')
    } catch (caught) {
      setFormError(userFacingError(caught))
    } finally {
      setBusy(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setBusy(true)
    try {
      await deleteUser(deleteTarget.id, csrfToken)
      toast({ title: 'User deleted', description: deleteTarget.username, variant: 'success' })
      setDeleteTarget(null)
      await refresh()
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setBusy(false)
    }
  }

  const users = usersQuery.data?.users ?? []

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="System"
        title="Users"
        description="Local accounts and roles. Administrators manage servers and users; operators act on assigned servers; read-only accounts can inspect but not mutate."
        actions={
          <Button
            onClick={() => {
              resetForm()
              setCreateOpen(true)
            }}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Add user
          </Button>
        }
      />

      {usersQuery.isError ? (
        <Alert variant="error" title="Users could not be loaded">
          {userFacingError(usersQuery.error)}
        </Alert>
      ) : usersQuery.isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3].map((item) => (
            <Skeleton key={item} className="h-14 w-full" />
          ))}
        </div>
      ) : (
        <Card>
          <CardContent className="px-0 py-0">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="text-ink-500 border-ink-700 border-b font-mono text-[10px] uppercase">
                  <tr>
                    <th className="px-4 py-2 font-medium">User</th>
                    <th className="px-4 py-2 font-medium">Role</th>
                    <th className="px-4 py-2 font-medium">Status</th>
                    <th className="hidden px-4 py-2 font-medium md:table-cell">Last login</th>
                    <th className="px-4 py-2 font-medium">Created</th>
                    <th className="px-4 py-2 text-right font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((item) => (
                    <tr key={item.id} className="border-ink-800 border-b last:border-0">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <div className="bg-ink-800 text-signal-300 flex h-8 w-8 items-center justify-center rounded-full">
                            <Users className="h-3.5 w-3.5" aria-hidden="true" />
                          </div>
                          <div>
                            <p className="text-ink-100 text-xs font-medium">{item.username}</p>
                            <p className="text-ink-500 font-mono text-[9px]">{item.id.slice(0, 12)}</p>
                          </div>
                          {item.id === user.id ? <Badge variant="outline">you</Badge> : null}
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <Select
                          value={item.role}
                          onChange={(event) => void handleRole(item, event.target.value as UserRole)}
                          className="h-8 w-36 text-xs"
                          aria-label={`Role for ${item.username}`}
                          disabled={item.id === user.id}
                        >
                          <option value="admin">Administrator</option>
                          <option value="operator">Operator</option>
                          <option value="read_only">Read-only</option>
                        </Select>
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          onClick={() => void toggleActive(item)}
                          disabled={item.id === user.id}
                          className="focus-visible:ring-signal-400 rounded focus-visible:ring-2 focus-visible:outline-none"
                        >
                          <Badge variant={item.is_active ? 'mint' : 'rose'}>
                            {item.is_active ? 'active' : 'disabled'}
                          </Badge>
                        </button>
                      </td>
                      <td className="text-ink-500 hidden px-4 py-3 text-xs md:table-cell">
                        {item.last_login_at ? formatRelativeTime(item.last_login_at) : 'never'}
                      </td>
                      <td className="text-ink-500 px-4 py-3 font-mono text-[10px]">
                        {formatDateTime(item.created_at)}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            size="icon"
                            variant="ghost"
                            className="h-7 w-7"
                            aria-label={`Reset password for ${item.username}`}
                            onClick={() => {
                              setFormError(null)
                              setNewPassword('')
                              setResetTarget(item)
                            }}
                          >
                            <KeyRound className="h-3.5 w-3.5" aria-hidden="true" />
                          </Button>
                          <Button
                            size="icon"
                            variant="ghost"
                            className="hover:text-rose-400 h-7 w-7"
                            aria-label={`Delete ${item.username}`}
                            disabled={item.id === user.id}
                            onClick={() => setDeleteTarget(item)}
                          >
                            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="border-ink-700 bg-ink-900/60 flex gap-3 rounded-lg border p-4">
        <ShieldCheck className="text-mint-400 mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <p className="text-ink-400 text-xs leading-5">
          Roles are enforced by the backend. Read-only accounts cannot open a terminal, signal
          processes, change services, edit files or run snippets. Assign servers from the Servers page.
        </p>
      </div>

      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open)
          if (!open) resetForm()
        }}
        title="Add local user"
        description="Create an account with a role. The password is hashed with Argon2id."
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateOpen(false)} disabled={busy}>
              Cancel
            </Button>
            <Button onClick={() => void handleCreate()} loading={busy}>
              Create user
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {formError ? <Alert variant="error">{formError}</Alert> : null}
          <Field label="Username" htmlFor="new-user-name">
            <Input
              id="new-user-name"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="off"
            />
          </Field>
          <Field label="Initial password" htmlFor="new-user-password" hint="At least 12 characters.">
            <Input
              id="new-user-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
            />
          </Field>
          <Field label="Role" htmlFor="new-user-role">
            <Select id="new-user-role" value={role} onChange={(event) => setRole(event.target.value as UserRole)}>
              <option value="operator">Operator</option>
              <option value="read_only">Read-only</option>
              <option value="admin">Administrator</option>
            </Select>
          </Field>
        </div>
      </Dialog>

      <Dialog
        open={resetTarget !== null}
        onOpenChange={(open) => {
          if (!open) setResetTarget(null)
        }}
        title="Reset password"
        description={resetTarget ? `${resetTarget.username}'s existing sessions will be revoked.` : undefined}
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setResetTarget(null)} disabled={busy}>
              Cancel
            </Button>
            <Button onClick={() => void handleReset()} loading={busy}>
              Reset password
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {formError ? <Alert variant="error">{formError}</Alert> : null}
          <Field label="New password" htmlFor="reset-password" hint="At least 12 characters.">
            <Input
              id="reset-password"
              type="password"
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              autoComplete="new-password"
            />
          </Field>
        </div>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title="Delete user?"
        description={`Delete ${deleteTarget?.username ?? 'this user'}? Their sessions, assignments and snippets are removed.`}
        confirmLabel="Delete user"
        destructive
        busy={busy}
        onConfirm={() => void handleDelete()}
      />
    </div>
  )
}
