import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Cable, Plus, Server as ServerIcon, ShieldCheck } from 'lucide-react'
import { ApiError } from '@/api/client'
import {
  deleteServer,
  listServers,
  testServer,
  trustHostKey,
  type Capabilities,
  type HostKeyInfo,
  type Server,
} from '@/api/servers'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ConfirmDialog } from '@/components/servers/ConfirmDialog'
import { HostKeyDialog } from '@/components/servers/HostKeyDialog'
import { ServerCard } from '@/components/servers/ServerCard'
import { ServerDialog } from '@/components/servers/ServerDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'

interface HostKeyPrompt {
  server: Server
  changed: boolean
  presented: HostKeyInfo
  previous?: HostKeyInfo
}

function isHostKeyInfo(value: unknown): value is HostKeyInfo {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  return (
    typeof candidate.key_type === 'string' &&
    typeof candidate.fingerprint === 'string' &&
    typeof candidate.public_key === 'string'
  )
}

function isChangedDetails(
  value: unknown,
): value is { previous: HostKeyInfo; presented: HostKeyInfo } {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  return isHostKeyInfo(candidate.previous) && isHostKeyInfo(candidate.presented)
}

export function ServersPage() {
  const { csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Server | null>(null)
  const [testingId, setTestingId] = useState<string | null>(null)
  const [capabilities, setCapabilities] = useState<Record<string, Capabilities>>({})
  const [hostKeyPrompt, setHostKeyPrompt] = useState<HostKeyPrompt | null>(null)
  const [trusting, setTrusting] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Server | null>(null)
  const [deleting, setDeleting] = useState(false)

  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })

  const servers = serversQuery.data?.servers ?? []

  const runTest = async (server: Server) => {
    setTestingId(server.id)
    try {
      const result = await testServer(server.id, csrfToken)
      setCapabilities((current) => ({ ...current, [server.id]: result.capabilities }))
      await queryClient.invalidateQueries({ queryKey: ['servers'] })
      toast({
        title: 'Connection verified',
        description: `${server.display_name} · ${result.capabilities.distribution || result.capabilities.os || 'Linux'}`,
        variant: 'success',
      })
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.code === 'SSH_HOST_KEY_UNKNOWN' && isHostKeyInfo(caught.details)) {
          setHostKeyPrompt({ server, changed: false, presented: caught.details })
          return
        }
        if (caught.code === 'SSH_HOST_KEY_CHANGED' && isChangedDetails(caught.details)) {
          setHostKeyPrompt({
            server,
            changed: true,
            presented: caught.details.presented,
            previous: caught.details.previous,
          })
          return
        }
      }
      toast({
        title: 'Connection failed',
        description: userFacingError(caught),
        variant: 'error',
      })
    } finally {
      setTestingId(null)
    }
  }

  const handleTrust = async (replace: boolean) => {
    if (!hostKeyPrompt) return
    setTrusting(true)
    try {
      await trustHostKey(
        hostKeyPrompt.server.id,
        {
          fingerprint: hostKeyPrompt.presented.fingerprint,
          key_type: hostKeyPrompt.presented.key_type,
          public_key: hostKeyPrompt.presented.public_key,
          replace,
        },
        csrfToken,
      )
      const server = hostKeyPrompt.server
      setHostKeyPrompt(null)
      toast({
        title: replace ? 'Host key replaced' : 'Host key trusted',
        description: 'Testing the connection again.',
        variant: 'success',
      })
      await runTest(server)
    } catch (caught) {
      toast({
        title: 'Host key could not be saved',
        description: userFacingError(caught),
        variant: 'error',
      })
    } finally {
      setTrusting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await deleteServer(deleteTarget.id, csrfToken)
      await queryClient.invalidateQueries({ queryKey: ['servers'] })
      toast({
        title: 'Server removed',
        description: 'The saved connection and credential were deleted.',
        variant: 'success',
      })
      setDeleteTarget(null)
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Workspace"
        title="Servers"
        description="Saved SSH connections. Drovik verifies each host key before trusting a server and never returns stored credentials to the browser."
        actions={
          <Button
            onClick={() => {
              setEditing(null)
              setDialogOpen(true)
            }}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Add server
          </Button>
        }
      />

      <div className="grid gap-3 sm:grid-cols-3">
        <Card className="p-5">
          <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">
            Saved servers
          </p>
          <p className="font-display text-ink-50 mt-2 text-2xl font-semibold">{servers.length}</p>
          <p className="text-ink-500 mt-1 text-xs">Agentless · SSH only</p>
        </Card>
        <Card className="p-5">
          <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">
            Host trust
          </p>
          <p className="font-display text-ink-50 mt-2 text-2xl font-semibold">Explicit</p>
          <p className="text-ink-500 mt-1 text-xs">First-use and changed-key approval</p>
        </Card>
        <Card className="p-5">
          <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">
            Credentials
          </p>
          <p className="font-display text-ink-50 mt-2 text-2xl font-semibold">AEAD</p>
          <p className="text-ink-500 mt-1 text-xs">AES-256-GCM · key outside the database</p>
        </Card>
      </div>

      {serversQuery.isError ? (
        <Alert variant="error" title="Servers could not be loaded">
          {userFacingError(serversQuery.error)}
        </Alert>
      ) : null}

      {serversQuery.isLoading ? (
        <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
          {[1, 2, 3].map((item) => (
            <Card key={item} className="p-5">
              <Skeleton className="h-5 w-40" />
              <Skeleton className="mt-3 h-3 w-56" />
              <Skeleton className="mt-6 h-8 w-32" />
            </Card>
          ))}
        </div>
      ) : servers.length === 0 ? (
        <div className="grid gap-4 lg:grid-cols-[1fr_22rem]">
          <Card>
            <CardContent className="flex flex-col items-center py-16 text-center">
              <div className="bg-ink-800 text-signal-300 flex h-14 w-14 items-center justify-center rounded-2xl">
                <ServerIcon className="h-7 w-7" aria-hidden="true" />
              </div>
              <h2 className="font-display text-ink-100 mt-4 text-xl font-semibold">
                No servers yet
              </h2>
              <p className="text-ink-400 mt-2 max-w-md text-sm leading-6">
                Add an SSH host to begin. The first connection shows the host-key fingerprint and
                waits for your approval before sending credentials.
              </p>
              <Button
                className="mt-5"
                onClick={() => {
                  setEditing(null)
                  setDialogOpen(true)
                }}
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Add your first server
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Connection path</CardTitle>
              <CardDescription>No agent on the managed host.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 pt-4">
              {[
                { icon: ServerIcon, title: 'Browser', detail: 'React workspace' },
                {
                  icon: Cable,
                  title: 'Drovik backend',
                  detail: 'Go · egress policy · host-key verification',
                },
                {
                  icon: ShieldCheck,
                  title: 'Managed Linux host',
                  detail: 'Existing OpenSSH account',
                },
              ].map((item, index) => {
                const Icon = item.icon
                return (
                  <div key={item.title} className="relative flex gap-3">
                    {index < 2 ? (
                      <span
                        className="bg-ink-700 absolute top-8 left-[15px] h-[calc(100%-1.5rem)] w-px"
                        aria-hidden="true"
                      />
                    ) : null}
                    <div className="bg-ink-850 ring-ink-700 text-ink-400 relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full ring-1">
                      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                    </div>
                    <div>
                      <p className="text-ink-200 text-xs font-medium">{item.title}</p>
                      <p className="text-ink-500 mt-0.5 text-[11px]">{item.detail}</p>
                    </div>
                  </div>
                )
              })}
              <div className="border-ink-700 border-t pt-3">
                <Badge variant="mint">Host keys verified</Badge>
              </div>
            </CardContent>
          </Card>
        </div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
          {servers.map((server) => (
            <ServerCard
              key={server.id}
              server={server}
              capabilities={capabilities[server.id]}
              testing={testingId === server.id}
              onTest={() => void runTest(server)}
              onEdit={() => {
                setEditing(server)
                setDialogOpen(true)
              }}
              onDelete={() => setDeleteTarget(server)}
            />
          ))}
        </div>
      )}

      <ServerDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        server={editing}
        csrfToken={csrfToken}
        onSaved={() => {
          void queryClient.invalidateQueries({ queryKey: ['servers'] })
        }}
      />

      <HostKeyDialog
        open={hostKeyPrompt !== null}
        onOpenChange={(open) => {
          if (!open) setHostKeyPrompt(null)
        }}
        server={hostKeyPrompt?.server ?? null}
        hostKey={hostKeyPrompt?.presented ?? null}
        previous={hostKeyPrompt?.previous}
        changed={hostKeyPrompt?.changed ?? false}
        busy={trusting}
        onTrust={(replace) => void handleTrust(replace)}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title="Remove saved server?"
        description={`Remove ${deleteTarget?.display_name ?? 'this server'} from Drovik?`}
        confirmLabel="Remove server"
        destructive
        busy={deleting}
        onConfirm={() => void handleDelete()}
      />
    </div>
  )
}
