import { useMemo } from 'react'
import type { ManagedUser } from '@/api/users'
import type { Server, ServerAssignment } from '@/api/servers'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Select } from '@/components/ui/Select'

interface ServerAccessDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
  users: ManagedUser[]
  assignments: ServerAssignment[]
  loading: boolean
  busyUserId: string | null
  onChange: (userId: string, level: 'read' | 'write' | null) => void
}

export function ServerAccessDialog({
  open,
  onOpenChange,
  server,
  users,
  assignments,
  loading,
  busyUserId,
  onChange,
}: ServerAccessDialogProps) {
  const byUser = useMemo(() => {
    const map = new Map<string, ServerAssignment>()
    for (const assignment of assignments) map.set(assignment.user_id, assignment)
    return map
  }, [assignments])

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Server access"
      description={server ? `${server.display_name} · ${server.username}@${server.host}` : undefined}
      size="lg"
      footer={<Button variant="ghost" onClick={() => onOpenChange(false)}>Close</Button>}
    >
      <div className="space-y-4">
        <Alert variant="info" title="Administrators already have access">
          Assign operators or read-only users to this server. Administrators can access every server
          and manage assignments.
        </Alert>
        {loading ? (
          <p className="text-ink-500 text-xs">Loading assignments…</p>
        ) : (
          <div className="space-y-2">
            {users.map((user) => {
              const assignment = byUser.get(user.id)
              return (
                <div key={user.id} className="border-ink-700 flex flex-wrap items-center gap-3 rounded-md border p-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-ink-100 text-xs font-medium">{user.username}</p>
                    <p className="text-ink-500 font-mono text-[9px] uppercase">{user.role}</p>
                  </div>
                  {user.role === 'admin' ? (
                    <Badge variant="mint">administrator access</Badge>
                  ) : (
                    <Select
                      aria-label={`Access for ${user.username}`}
                      value={assignment?.access_level ?? 'none'}
                      disabled={busyUserId === user.id}
                      onChange={(event) => {
                        const value = event.target.value
                        onChange(user.id, value === 'none' ? null : (value as 'read' | 'write'))
                      }}
                      className="h-8 w-36 text-xs"
                    >
                      <option value="none">No access</option>
                      <option value="read">Read-only</option>
                      <option value="write">Operator</option>
                    </Select>
                  )}
                </div>
              )
            })}
          </div>
        )}
        <p className="text-ink-500 text-[11px] leading-5">
          Read-only assignment blocks terminals, signals, service actions, file mutations and snippet
          execution at the backend. A read-only platform account is still shown as a read-only role.
        </p>
      </div>
    </Dialog>
  )
}
