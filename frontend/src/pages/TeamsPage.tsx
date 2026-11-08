import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, ShieldCheck, Trash2, UserPlus, Users } from 'lucide-react'
import {
  addTeamMember,
  createTeam,
  deleteTeam,
  getTeam,
  listTeams,
  removeTeamMember,
  removeTeamServer,
  setTeamServer,
  updateTeam,
  type Team,
} from '@/api/teams'
import { listServers } from '@/api/servers'
import { listUsers } from '@/api/users'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ConfirmDialog } from '@/components/servers/ConfirmDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { Textarea } from '@/components/ui/Textarea'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'

export function TeamsPage() {
  const { user, csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<Team | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [manageID, setManageID] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Team | null>(null)
  const [memberToAdd, setMemberToAdd] = useState('')
  const [serverToAdd, setServerToAdd] = useState('')
  const [serverAccess, setServerAccess] = useState<'read' | 'write'>('read')
  const [actionBusy, setActionBusy] = useState(false)

  const teamsQuery = useQuery({
    queryKey: ['teams'],
    queryFn: ({ signal }) => listTeams(signal),
    enabled: user?.role === 'admin',
    retry: 0,
  })
  const usersQuery = useQuery({
    queryKey: ['users'],
    queryFn: ({ signal }) => listUsers(signal),
    enabled: user?.role === 'admin',
  })
  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
    enabled: user?.role === 'admin',
  })
  const detailQuery = useQuery({
    queryKey: ['team', manageID],
    queryFn: ({ signal }) => getTeam(manageID ?? '', signal),
    enabled: Boolean(manageID) && user?.role === 'admin',
    retry: 0,
  })

  if (user?.role !== 'admin') {
    return (
      <Alert variant="warning" title="Administrator access required">
        Teams are managed by administrators. Ask an administrator to add you to a team.
      </Alert>
    )
  }

  const teams = teamsQuery.data?.teams ?? []
  const members = detailQuery.data?.members ?? []
  const teamServers = detailQuery.data?.servers ?? []
  const memberIDs = new Set(members.map((member) => member.user_id))
  const serverIDs = new Set(teamServers.map((server) => server.server_id))
  const availableUsers = (usersQuery.data?.users ?? []).filter((candidate) => !memberIDs.has(candidate.id))
  const availableServers = (serversQuery.data?.servers ?? []).filter((server) => !serverIDs.has(server.id))

  const openCreate = () => {
    setEditing(null)
    setName('')
    setDescription('')
    setFormError(null)
    setEditorOpen(true)
  }

  const openEdit = (team: Team) => {
    setEditing(team)
    setName(team.name)
    setDescription(team.description)
    setFormError(null)
    setEditorOpen(true)
  }

  const saveTeam = async () => {
    setSaving(true)
    setFormError(null)
    try {
      if (editing) {
        await updateTeam(editing.id, { name: name.trim(), description: description.trim() }, csrfToken)
        toast({ title: 'Team updated', description: name.trim(), variant: 'success' })
      } else {
        await createTeam({ name: name.trim(), description: description.trim() }, csrfToken)
        toast({ title: 'Team created', description: name.trim(), variant: 'success' })
      }
      setEditorOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['teams'] })
      if (editing) await queryClient.invalidateQueries({ queryKey: ['team', editing.id] })
    } catch (caught) {
      setFormError(userFacingError(caught))
    } finally {
      setSaving(false)
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setActionBusy(true)
    try {
      await deleteTeam(deleteTarget.id, csrfToken)
      toast({ title: 'Team deleted', description: deleteTarget.name, variant: 'success' })
      if (manageID === deleteTarget.id) setManageID(null)
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['teams'] })
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setActionBusy(false)
    }
  }

  const addMember = async () => {
    if (!manageID || !memberToAdd) return
    setActionBusy(true)
    try {
      await addTeamMember(manageID, memberToAdd, csrfToken)
      setMemberToAdd('')
      await queryClient.invalidateQueries({ queryKey: ['team', manageID] })
      await queryClient.invalidateQueries({ queryKey: ['teams'] })
    } catch (caught) {
      toast({ title: 'Could not add member', description: userFacingError(caught), variant: 'error' })
    } finally {
      setActionBusy(false)
    }
  }

  const removeMember = async (userID: string) => {
    if (!manageID) return
    setActionBusy(true)
    try {
      await removeTeamMember(manageID, userID, csrfToken)
      await queryClient.invalidateQueries({ queryKey: ['team', manageID] })
      await queryClient.invalidateQueries({ queryKey: ['teams'] })
    } catch (caught) {
      toast({ title: 'Could not remove member', description: userFacingError(caught), variant: 'error' })
    } finally {
      setActionBusy(false)
    }
  }

  const addServer = async () => {
    if (!manageID || !serverToAdd) return
    setActionBusy(true)
    try {
      await setTeamServer(manageID, serverToAdd, serverAccess, csrfToken)
      setServerToAdd('')
      await queryClient.invalidateQueries({ queryKey: ['team', manageID] })
      await queryClient.invalidateQueries({ queryKey: ['teams'] })
    } catch (caught) {
      toast({ title: 'Could not assign server', description: userFacingError(caught), variant: 'error' })
    } finally {
      setActionBusy(false)
    }
  }

  const removeServer = async (serverID: string) => {
    if (!manageID) return
    setActionBusy(true)
    try {
      await removeTeamServer(manageID, serverID, csrfToken)
      await queryClient.invalidateQueries({ queryKey: ['team', manageID] })
      await queryClient.invalidateQueries({ queryKey: ['teams'] })
    } catch (caught) {
      toast({ title: 'Could not remove server', description: userFacingError(caught), variant: 'error' })
    } finally {
      setActionBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="System"
        title="Teams"
        description="Group users and share servers with a read or write assignment. Team access is additive to direct assignments."
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            New team
          </Button>
        }
      />

      {teamsQuery.isLoading ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {[1, 2, 3].map((item) => <Skeleton key={item} className="h-40 w-full" />)}
        </div>
      ) : teamsQuery.isError ? (
        <Alert variant="error">{userFacingError(teamsQuery.error)}</Alert>
      ) : teams.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <Users className="text-ink-500 mx-auto h-7 w-7" aria-hidden="true" />
            <p className="text-ink-200 mt-3 text-sm font-medium">No teams yet</p>
            <p className="text-ink-500 mt-1 text-xs">Create a team, add members, then assign servers.</p>
            <Button className="mt-4" size="sm" onClick={openCreate}>Create the first team</Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {teams.map((team) => (
            <Card key={team.id}>
              <CardHeader>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <CardTitle>{team.name}</CardTitle>
                    <CardDescription>{team.description || 'No description'}</CardDescription>
                  </div>
                  <Badge variant="outline">{team.member_count} members</Badge>
                </div>
              </CardHeader>
              <CardContent className="space-y-4 pt-2">
                <div className="text-ink-500 flex items-center gap-2 text-xs">
                  <ShieldCheck className="h-3.5 w-3.5" aria-hidden="true" />
                  {team.server_count} server{team.server_count === 1 ? '' : 's'} shared
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" variant="outline" onClick={() => { setManageID(team.id); setMemberToAdd(''); setServerToAdd('') }}>
                    Manage
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => openEdit(team)}>Edit</Button>
                  <Button size="sm" variant="ghost" className="hover:text-rose-400" onClick={() => setDeleteTarget(team)}>
                    <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                    Delete
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        title={editing ? 'Edit team' : 'New team'}
        description="Teams are an administrative grouping. Members inherit server access from the team."
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditorOpen(false)} disabled={saving}>Cancel</Button>
            <Button onClick={() => void saveTeam()} loading={saving} disabled={!name.trim()}>{editing ? 'Save changes' : 'Create team'}</Button>
          </>
        }
      >
        <div className="space-y-4">
          {formError ? <Alert variant="error">{formError}</Alert> : null}
          <Field label="Name" htmlFor="team-name">
            <Input id="team-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Operations" autoComplete="off" />
          </Field>
          <Field label="Description" htmlFor="team-description" hint="Optional. Do not store secrets here.">
            <Textarea id="team-description" value={description} onChange={(event) => setDescription(event.target.value)} className="min-h-20" placeholder="Who this team is for" />
          </Field>
        </div>
      </Dialog>

      <Dialog
        open={Boolean(manageID)}
        onOpenChange={(open) => { if (!open) setManageID(null) }}
        title={detailQuery.data?.team.name ?? 'Team'}
        description="Add members and choose the server access they inherit."
        size="lg"
      >
        {detailQuery.isLoading ? <Skeleton className="h-40 w-full" /> : detailQuery.isError ? (
          <Alert variant="error">{userFacingError(detailQuery.error)}</Alert>
        ) : (
          <div className="space-y-6">
            <section className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-ink-100 text-sm font-medium">Members</h3>
                <Badge variant="outline">{members.length}</Badge>
              </div>
              <div className="flex gap-2">
                <Select value={memberToAdd} onChange={(event) => setMemberToAdd(event.target.value)} aria-label="User to add">
                  <option value="">Select a user</option>
                  {availableUsers.map((candidate) => (
                    <option key={candidate.id} value={candidate.id}>{candidate.username} · {candidate.role}</option>
                  ))}
                </Select>
                <Button size="sm" variant="outline" onClick={() => void addMember()} disabled={!memberToAdd || actionBusy}>
                  <UserPlus className="h-3.5 w-3.5" aria-hidden="true" />
                  Add
                </Button>
              </div>
              {members.length === 0 ? <p className="text-ink-500 text-xs">No members yet.</p> : (
                <div className="divide-ink-800 divide-y">
                  {members.map((member) => (
                    <div key={member.user_id} className="flex items-center justify-between gap-3 py-2">
                      <div>
                        <p className="text-ink-200 text-xs font-medium">{member.username}</p>
                        <p className="text-ink-500 font-mono text-[10px] uppercase">{member.role}</p>
                      </div>
                      <Button size="sm" variant="ghost" className="hover:text-rose-400" onClick={() => void removeMember(member.user_id)} disabled={actionBusy}>
                        Remove
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-ink-100 text-sm font-medium">Servers</h3>
                <Badge variant="outline">{teamServers.length}</Badge>
              </div>
              <div className="flex flex-wrap gap-2">
                <Select value={serverToAdd} onChange={(event) => setServerToAdd(event.target.value)} aria-label="Server to assign" className="min-w-44 flex-1">
                  <option value="">Select a server</option>
                  {availableServers.map((server) => (
                    <option key={server.id} value={server.id}>{server.display_name} · {server.host}</option>
                  ))}
                </Select>
                <Select value={serverAccess} onChange={(event) => setServerAccess(event.target.value as 'read' | 'write')} aria-label="Team server access">
                  <option value="read">read</option>
                  <option value="write">write</option>
                </Select>
                <Button size="sm" variant="outline" onClick={() => void addServer()} disabled={!serverToAdd || actionBusy}>Assign</Button>
              </div>
              {teamServers.length === 0 ? <p className="text-ink-500 text-xs">No servers shared with this team.</p> : (
                <div className="divide-ink-800 divide-y">
                  {teamServers.map((server) => (
                    <div key={server.server_id} className="flex items-center justify-between gap-3 py-2">
                      <div>
                        <p className="text-ink-200 text-xs font-medium">{server.display_name}</p>
                        <p className="text-ink-500 font-mono text-[10px]">{server.host} · {server.access_level}</p>
                      </div>
                      <Button size="sm" variant="ghost" className="hover:text-rose-400" onClick={() => void removeServer(server.server_id)} disabled={actionBusy}>
                        Remove
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        )}
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}
        title="Delete team?"
        description={deleteTarget ? `Delete ${deleteTarget.name}. Members lose the access inherited from this team.` : ''}
        confirmLabel="Delete team"
        destructive
        busy={actionBusy}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  )
}
