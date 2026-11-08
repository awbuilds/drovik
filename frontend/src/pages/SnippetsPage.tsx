import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Code2, Play, Plus, Trash2 } from 'lucide-react'
import {
  createSnippet,
  deleteSnippet,
  listSnippets,
  previewSnippet,
  runSnippet,
  updateSnippet,
  type Snippet,
  type SnippetInput,
  type SnippetVariable,
} from '@/api/advanced'
import { listServers } from '@/api/servers'
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
import { formatDateTime } from '@/lib/format'

interface SnippetFormState {
  name: string
  description: string
  command: string
  variables: string
}

const emptyForm: SnippetFormState = { name: '', description: '', command: '', variables: '' }

function parseVariables(raw: string): SnippetVariable[] {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [name = '', description = '', defaultValue = ''] = line
        .split('|')
        .map((part) => part.trim())
      return { name, description, default: defaultValue }
    })
}

function formatVariables(variables: SnippetVariable[]): string {
  return variables.map((variable) => `${variable.name}|${variable.description ?? ''}|${variable.default ?? ''}`).join('\n')
}

export function SnippetsPage() {
  const { csrfToken, user } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<Snippet | null>(null)
  const [form, setForm] = useState<SnippetFormState>(emptyForm)
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Snippet | null>(null)
  const [runTarget, setRunTarget] = useState<Snippet | null>(null)
  const [runServer, setRunServer] = useState('')
  const [values, setValues] = useState<Record<string, string>>({})
  const [preview, setPreview] = useState('')
  const [runOutput, setRunOutput] = useState('')
  const [runError, setRunError] = useState<string | null>(null)
  const [runBusy, setRunBusy] = useState(false)
  const [confirmed, setConfirmed] = useState(false)

  const snippetsQuery = useQuery({
    queryKey: ['snippets'],
    queryFn: ({ signal }) => listSnippets(signal),
  })
  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const snippets = useMemo(() => snippetsQuery.data?.snippets ?? [], [snippetsQuery.data?.snippets])
  const servers = serversQuery.data?.servers ?? []
  const canWrite = user?.role !== 'read_only'

  const sorted = useMemo(() => [...snippets].sort((a, b) => a.name.localeCompare(b.name)), [snippets])

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm)
    setFormError(null)
    setEditorOpen(true)
  }

  const openEdit = (snippet: Snippet) => {
    setEditing(snippet)
    setForm({
      name: snippet.name,
      description: snippet.description,
      command: snippet.command,
      variables: formatVariables(snippet.variables),
    })
    setFormError(null)
    setEditorOpen(true)
  }

  const save = async () => {
    setSaving(true)
    setFormError(null)
    try {
      const input: SnippetInput = {
        name: form.name.trim(),
        description: form.description.trim(),
        command: form.command,
        variables: parseVariables(form.variables),
      }
      if (editing) {
        await updateSnippet(editing.id, input, csrfToken)
        toast({ title: 'Snippet updated', description: input.name, variant: 'success' })
      } else {
        await createSnippet(input, csrfToken)
        toast({ title: 'Snippet created', description: input.name, variant: 'success' })
      }
      setEditorOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['snippets'] })
    } catch (caught) {
      setFormError(userFacingError(caught))
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (!deleteTarget) return
    setSaving(true)
    try {
      await deleteSnippet(deleteTarget.id, csrfToken)
      toast({ title: 'Snippet deleted', description: deleteTarget.name, variant: 'success' })
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['snippets'] })
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const writableServers = servers.filter((server) => server.access_level !== 'read')

  const openRun = (snippet: Snippet) => {
    setRunTarget(snippet)
    setRunServer(writableServers[0]?.id ?? '')
    setValues(Object.fromEntries(snippet.variables.map((variable) => [variable.name, variable.default ?? ''])))
    setPreview('')
    setRunOutput('')
    setRunError(null)
    setConfirmed(false)
  }

  const previewCommand = async () => {
    if (!runTarget) return
    setRunBusy(true)
    setRunError(null)
    try {
      const result = await previewSnippet(runTarget.id, values, csrfToken)
      setPreview(result.command)
    } catch (caught) {
      setRunError(userFacingError(caught))
    } finally {
      setRunBusy(false)
    }
  }

  const execute = async () => {
    if (!runTarget || !runServer || !confirmed) return
    setRunBusy(true)
    setRunError(null)
    setRunOutput('')
    try {
      const result = await runSnippet(runServer, runTarget.id, values, csrfToken)
      setPreview(result.command)
      setRunOutput(result.output || '(no output)')
      toast({ title: 'Snippet executed', description: runTarget.name, variant: 'success' })
    } catch (caught) {
      setRunError(userFacingError(caught))
    } finally {
      setRunBusy(false)
    }
  }

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Operate"
        title="Snippets"
        description="Reusable command templates with an explicit target, fully substituted preview and confirmation. Snippets execute as the connected SSH account."
        actions={
          canWrite ? (
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" aria-hidden="true" />
              New snippet
            </Button>
          ) : null
        }
      />

      <Alert variant="warning" title="Do not store secrets in snippets">
        Commands and variable defaults are stored in Drovik's database and shown in the preview. Use
        placeholders for non-secret values only; secrets belong in the SSH credential store, not a
        snippet.
      </Alert>

      {snippetsQuery.isError ? (
        <Alert variant="error" title="Snippets could not be loaded">{userFacingError(snippetsQuery.error)}</Alert>
      ) : snippetsQuery.isLoading ? (
        <div className="space-y-2">{[1, 2, 3].map((item) => <Skeleton key={item} className="h-16 w-full" />)}</div>
      ) : sorted.length === 0 ? (
        <Card className="p-10 text-center">
          <Code2 className="text-signal-300 mx-auto h-8 w-8" aria-hidden="true" />
          <h2 className="font-display text-ink-50 mt-4 text-xl font-semibold">No snippets yet</h2>
          <p className="text-ink-400 mt-2 text-sm">Create a named command template, preview it, then run it against a selected server.</p>
        </Card>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          {sorted.map((snippet) => (
            <Card key={snippet.id}>
              <CardHeader className="flex-row items-start justify-between gap-3">
                <div>
                  <CardTitle className="flex items-center gap-2"><Code2 className="text-signal-400 h-4 w-4" aria-hidden="true" />{snippet.name}</CardTitle>
                  <CardDescription>{snippet.description || 'No description'}</CardDescription>
                </div>
                <div className="flex gap-1">
                  {snippet.variables.length > 0 ? <Badge variant="outline">{snippet.variables.length} vars</Badge> : null}
                </div>
              </CardHeader>
              <CardContent className="space-y-3 pt-4">
                <pre className="bg-ink-950 text-ink-300 max-h-32 overflow-auto rounded-md p-3 font-mono text-[10px] whitespace-pre-wrap">{snippet.command}</pre>
                <div className="text-ink-500 flex items-center justify-between gap-2 font-mono text-[10px]">
                  <span>updated {formatDateTime(snippet.updated_at)}</span>
                  <div className="flex gap-1">
                    {canWrite ? (
                      <>
                        <Button size="sm" variant="outline" onClick={() => openRun(snippet)}><Play className="h-3.5 w-3.5" aria-hidden="true" />Run</Button>
                        <Button size="sm" variant="ghost" onClick={() => openEdit(snippet)}>Edit</Button>
                        <Button size="icon" variant="ghost" className="hover:text-rose-400 h-8 w-8" aria-label={`Delete ${snippet.name}`} onClick={() => setDeleteTarget(snippet)}><Trash2 className="h-3.5 w-3.5" aria-hidden="true" /></Button>
                      </>
                    ) : null}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        title={editing ? 'Edit snippet' : 'New snippet'}
        description="Use {{variable}} placeholders. Values are shell-quoted when substituted."
        size="lg"
        footer={<><Button variant="ghost" onClick={() => setEditorOpen(false)} disabled={saving}>Cancel</Button><Button onClick={() => void save()} loading={saving}>{editing ? 'Save changes' : 'Create snippet'}</Button></>}
      >
        <div className="space-y-4">
          {formError ? <Alert variant="error">{formError}</Alert> : null}
          <Field label="Name" htmlFor="snippet-name"><Input id="snippet-name" value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} /></Field>
          <Field label="Description" htmlFor="snippet-description"><Input id="snippet-description" value={form.description} onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))} /></Field>
          <Field label="Command" htmlFor="snippet-command" hint="Example: systemctl status {{unit}}">
            <Textarea id="snippet-command" value={form.command} onChange={(event) => setForm((current) => ({ ...current, command: event.target.value }))} className="min-h-32" spellCheck={false} />
          </Field>
          <Field label="Variables" htmlFor="snippet-variables" hint="One per line: name|description|default. Names use letters, numbers and underscore.">
            <Textarea id="snippet-variables" value={form.variables} onChange={(event) => setForm((current) => ({ ...current, variables: event.target.value }))} className="min-h-20" spellCheck={false} placeholder={'unit|systemd unit|ssh.service'} />
          </Field>
        </div>
      </Dialog>

      <Dialog
        open={runTarget !== null}
        onOpenChange={(open) => { if (!open) setRunTarget(null) }}
        title={`Run ${runTarget?.name ?? 'snippet'}`}
        description="Select a target, fill variables, preview, then confirm execution."
        size="lg"
        footer={<><Button variant="ghost" onClick={() => setRunTarget(null)} disabled={runBusy}>Close</Button><Button onClick={() => void previewCommand()} variant="outline" loading={runBusy}>Preview</Button><Button onClick={() => void execute()} loading={runBusy} disabled={!confirmed || !runServer || writableServers.length === 0}>Run on server</Button></>}
      >
        <div className="space-y-4">
          {runError ? <Alert variant="error">{runError}</Alert> : null}
          <Field label="Target server" htmlFor="snippet-server">
            <Select id="snippet-server" value={runServer} onChange={(event) => setRunServer(event.target.value)}>
              <option value="">Select a server…</option>
              {writableServers.map((server) => <option key={server.id} value={server.id}>{server.display_name} · {server.host}</option>)}
            </Select>
          </Field>
          {(runTarget?.variables ?? []).map((variable) => (
            <Field key={variable.name} label={variable.name} htmlFor={`snippet-var-${variable.name}`} hint={variable.description}>
              <Input id={`snippet-var-${variable.name}`} value={values[variable.name] ?? ''} onChange={(event) => setValues((current) => ({ ...current, [variable.name]: event.target.value }))} />
            </Field>
          ))}
          <div>
            <p className="text-ink-500 font-mono text-[10px] uppercase">Substituted preview</p>
            <pre className="bg-ink-950 text-ink-200 mt-2 min-h-16 overflow-auto rounded-md p-3 font-mono text-[11px] whitespace-pre-wrap">{preview || 'Preview has not been generated yet.'}</pre>
          </div>
          {writableServers.length === 0 ? (
            <Alert variant="warning" title="No writable servers">
              You have read-only access to every server. Ask an administrator for write access before
              running snippets.
            </Alert>
          ) : (
            <label className="text-ink-300 flex items-start gap-2 text-xs">
              <input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} className="accent-signal-400 mt-0.5 h-4 w-4" />
              I confirm this command should run on the selected server as the connected SSH account.
            </label>
          )}
          {runOutput ? <pre className="bg-ink-950 text-mint-200 max-h-52 overflow-auto rounded-md p-3 font-mono text-[11px] whitespace-pre-wrap">{runOutput}</pre> : null}
        </div>
      </Dialog>

      <ConfirmDialog open={deleteTarget !== null} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }} title="Delete snippet?" description={`Delete ${deleteTarget?.name ?? 'this snippet'} from Drovik?`} confirmLabel="Delete snippet" destructive busy={saving} onConfirm={() => void remove()} />
    </div>
  )
}
