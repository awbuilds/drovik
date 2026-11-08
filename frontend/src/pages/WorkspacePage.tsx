import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  ArrowRight,
  Cable,
  Check,
  CircleDashed,
  FileText,
  KeyRound,
  LockKeyhole,
  MonitorCheck,
  Plus,
  ShieldCheck,
  SquareTerminal,
  Server,
} from 'lucide-react'
import { getVersion } from '@/api/client'
import { listServers } from '@/api/servers'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'
import { StatusDot } from '@/components/ui/StatusDot'
import { formatRelativeTime } from '@/lib/format'

const nextSteps = [
  {
    phase: 'Phase 2',
    title: 'SSH connections',
    detail: 'Saved hosts and host-key trust',
    icon: Cable,
  },
  {
    phase: 'Phase 3',
    title: 'Terminal',
    detail: 'Interactive PTY in the browser',
    icon: SquareTerminal,
  },
  { phase: 'Phase 4', title: 'Files', detail: 'SFTP browsing and editing', icon: FileText },
  {
    phase: 'Phase 5',
    title: 'Overview',
    detail: 'CPU, memory, disk and network',
    icon: MonitorCheck,
  },
]

const posture = [
  {
    label: 'Administrator authentication',
    detail: 'Argon2id · server-side sessions',
    icon: KeyRound,
  },
  { label: 'Request protection', detail: 'CSRF, Origin and rate limits', icon: ShieldCheck },
  {
    label: 'Credential encryption key',
    detail: 'Loaded from an external secret',
    icon: LockKeyhole,
  },
  { label: 'Telemetry', detail: 'Off by default · no external calls', icon: CircleDashed },
]

export function WorkspacePage() {
  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
    staleTime: 30_000,
  })
  const servers = serversQuery.data?.servers ?? []
  const versionQuery = useQuery({
    queryKey: ['version'],
    queryFn: ({ signal }) => getVersion(signal),
    staleTime: 60_000,
    retry: 1,
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Workspace"
        title="Command center"
        description="Your control surface is ready. Connect a Linux host to start working through SSH — no agent, no hidden privilege."
        actions={
          <>
            <Badge variant="mint">Phase 2 live</Badge>
            <Button asChild variant="outline">
              <Link to="/servers">
                <Plus className="h-4 w-4" aria-hidden="true" />
                Manage servers
              </Link>
            </Button>
          </>
        }
      />

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard
          label="Authentication"
          value="Ready"
          detail="Argon2id · server sessions"
          status="online"
        />
        <ErrorBoundary compact title="Version information unavailable">
          {versionQuery.isLoading ? (
            <Card className="p-5">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="mt-3 h-7 w-16" />
              <Skeleton className="mt-2 h-3 w-40" />
            </Card>
          ) : versionQuery.isError ? (
            <Card className="p-5">
              <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">
                Schema
              </p>
              <p className="font-display text-ink-300 mt-2 text-2xl font-semibold">Unavailable</p>
              <p className="text-ink-500 mt-1 text-xs">The API version endpoint did not respond.</p>
            </Card>
          ) : (
            <StatCard
              label="Schema"
              value={`v${versionQuery.data?.schema_version ?? 0}`}
              detail={`${versionQuery.data?.version ?? 'dev'} · SQLite migrations current`}
              status="online"
            />
          )}
        </ErrorBoundary>
        <StatCard
          label="Saved servers"
          value={String(servers.length)}
          detail="Agentless · no remote daemon"
          status={servers.length > 0 ? 'online' : 'pending'}
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-12">
        <Card className="xl:col-span-7">
          <CardHeader className="flex-row items-start justify-between gap-4">
            <div>
              <CardTitle>{servers.length > 0 ? 'Saved servers' : 'Connection'}</CardTitle>
              <CardDescription>
                {servers.length > 0
                  ? `${servers.length} SSH connection${servers.length === 1 ? '' : 's'} configured. Test a host to verify its key and capabilities.`
                  : 'The workspace is intentionally empty until you add a host.'}
              </CardDescription>
            </div>
            <Badge variant="signal">SSH</Badge>
          </CardHeader>
          <CardContent>
            <div className="border-ink-700 bg-ink-950/50 relative overflow-hidden rounded-lg border p-6">
              <div className="grid-backdrop absolute inset-0 opacity-40" aria-hidden="true" />
              {servers.length > 0 ? (
                <div className="relative space-y-2">
                  {servers.slice(0, 4).map((server) => (
                    <div
                      key={server.id}
                      className="border-ink-700 bg-ink-900/80 flex items-center gap-3 rounded-lg border px-3 py-2.5"
                    >
                      <div className="bg-ink-800 text-signal-300 flex h-8 w-8 shrink-0 items-center justify-center rounded-md">
                        <Server className="h-3.5 w-3.5" aria-hidden="true" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="text-ink-100 truncate text-sm font-medium">
                          {server.display_name}
                        </p>
                        <p className="text-ink-500 truncate font-mono text-[10px]">
                          {server.username}@{server.host}:{server.port}
                        </p>
                      </div>
                      <div className="text-right">
                        <StatusDot
                          status={server.last_connected_at ? 'online' : 'pending'}
                          label={server.last_connected_at ? 'verified' : 'untested'}
                        />
                        <p className="text-ink-600 mt-1 font-mono text-[9px]">
                          {server.last_connected_at
                            ? formatRelativeTime(server.last_connected_at)
                            : 'never'}
                        </p>
                      </div>
                    </div>
                  ))}
                  <div className="pt-2 text-right">
                    <Button asChild size="sm" variant="outline">
                      <Link to="/servers">Manage servers</Link>
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="relative flex flex-col items-center py-6 text-center">
                  <div className="bg-ink-800 ring-ink-700 text-signal-300 flex h-14 w-14 items-center justify-center rounded-2xl ring-1">
                    <Server className="h-7 w-7" aria-hidden="true" />
                  </div>
                  <h3 className="font-display text-ink-100 mt-4 text-lg font-semibold">
                    No servers connected yet
                  </h3>
                  <p className="text-ink-400 mt-2 max-w-sm text-sm leading-6">
                    Add a hostname, port and SSH identity. Drovik will show the host-key fingerprint
                    before it trusts the server.
                  </p>
                  <Button asChild variant="outline" className="mt-5">
                    <Link to="/servers">
                      <Plus className="h-4 w-4" aria-hidden="true" />
                      Add your first server
                    </Link>
                  </Button>
                </div>
              )}

              <div className="border-ink-700 relative mt-4 grid grid-cols-[1fr_auto_1fr_auto_1fr] items-center gap-2 border-t pt-5">
                <PathNode label="Browser" detail="React" />
                <ArrowRight className="text-ink-600 h-3.5 w-3.5" aria-hidden="true" />
                <PathNode label="Drovik" detail="Go API" accent />
                <ArrowRight className="text-ink-600 h-3.5 w-3.5" aria-hidden="true" />
                <PathNode label="Linux host" detail="SSH / SFTP" />
              </div>
            </div>

            <div className="mt-5 grid gap-2 sm:grid-cols-2">
              {[
                'Password or private-key authentication',
                'Explicit first-use host-key approval',
                'Changed-host-key blocking',
                'Bounded connection timeouts',
              ].map((item) => (
                <div key={item} className="text-ink-300 flex items-start gap-2 text-xs">
                  <Check className="text-mint-400 mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  {item}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        <div className="space-y-4 xl:col-span-5">
          <Card>
            <CardHeader>
              <CardTitle>Security posture</CardTitle>
              <CardDescription>Controls active in this build.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 pt-4">
              {posture.map((item) => {
                const Icon = item.icon
                return (
                  <div key={item.label} className="flex items-start gap-3">
                    <div className="bg-ink-800 text-ink-400 mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md">
                      <Icon className="h-4 w-4" aria-hidden="true" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="text-ink-200 text-xs font-medium">{item.label}</p>
                      <p className="text-ink-500 mt-0.5 text-[11px]">{item.detail}</p>
                    </div>
                    <StatusDot status="online" label="" className="mt-2" />
                  </div>
                )
              })}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Next on the surface</CardTitle>
              <CardDescription>The roadmap, in delivery order.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-1 pt-4">
              {nextSteps.map((step, index) => {
                const Icon = step.icon
                return (
                  <div key={step.title} className="relative flex gap-3 pb-4 last:pb-0">
                    {index < nextSteps.length - 1 ? (
                      <span
                        className="bg-ink-700 absolute top-8 left-[15px] h-[calc(100%-1.5rem)] w-px"
                        aria-hidden="true"
                      />
                    ) : null}
                    <div className="bg-ink-850 ring-ink-700 text-ink-400 relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full ring-1">
                      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <p className="text-ink-200 text-xs font-medium">{step.title}</p>
                        <span className="text-ink-600 font-mono text-[9px] uppercase">
                          {step.phase}
                        </span>
                      </div>
                      <p className="text-ink-500 mt-0.5 text-[11px]">{step.detail}</p>
                    </div>
                  </div>
                )
              })}
            </CardContent>
          </Card>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Operator notes</CardTitle>
            <CardDescription>What this build does — and does not — claim.</CardDescription>
          </CardHeader>
          <CardContent className="text-ink-400 space-y-3 pt-4 text-xs leading-5">
            <p>
              Phase 1 delivers the application foundation: first-run setup, local authentication,
              server-side sessions, CSRF protection and this workspace shell.
            </p>
            <p>
              Remote SSH, SFTP, terminal, monitoring, processes, services and logs are deliberately
              not implemented yet. Nothing in this build pretends they are.
            </p>
            <a
              href="https://github.com/awbuilds/drovik/blob/main/ROADMAP.md"
              target="_blank"
              rel="noreferrer"
              className="text-signal-300 hover:text-signal-200 inline-flex items-center gap-1 underline underline-offset-2"
            >
              Read the roadmap
              <ArrowRight className="h-3 w-3" aria-hidden="true" />
            </a>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Build integrity</CardTitle>
            <CardDescription>Honest status, not a demo badge.</CardDescription>
          </CardHeader>
          <CardContent className="pt-4">
            <dl className="space-y-2 font-mono text-xs">
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">release</dt>
                <dd className="text-ink-200">{versionQuery.data?.version ?? 'dev'}</dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">commit</dt>
                <dd className="text-ink-200 truncate">
                  {versionQuery.data?.commit?.slice(0, 12) ?? 'unknown'}
                </dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">schema</dt>
                <dd className="text-ink-200">
                  {versionQuery.data ? `v${versionQuery.data.schema_version}` : 'unknown'}
                </dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">license</dt>
                <dd className="text-ink-200">AGPL-3.0-only</dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  detail,
  status,
}: {
  label: string
  value: string
  detail: string
  status: 'online' | 'offline' | 'warning' | 'pending'
}) {
  return (
    <Card className="p-5">
      <div className="flex items-center justify-between">
        <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">{label}</p>
        <StatusDot status={status} label="" />
      </div>
      <p className="font-display text-ink-50 mt-2 text-2xl font-semibold tracking-tight">{value}</p>
      <p className="text-ink-500 mt-1 text-xs">{detail}</p>
    </Card>
  )
}

function PathNode({ label, detail, accent }: { label: string; detail: string; accent?: boolean }) {
  return (
    <div className="text-center">
      <div
        className={
          accent
            ? 'border-signal-500/40 bg-signal-400/10 mx-auto flex h-9 w-9 items-center justify-center rounded-lg border'
            : 'border-ink-600 bg-ink-800 mx-auto flex h-9 w-9 items-center justify-center rounded-lg border'
        }
      >
        <Server
          className={accent ? 'text-signal-300 h-4 w-4' : 'text-ink-400 h-4 w-4'}
          aria-hidden="true"
        />
      </div>
      <p className="text-ink-200 mt-2 text-[11px] font-medium">{label}</p>
      <p className="text-ink-600 font-mono text-[9px] uppercase">{detail}</p>
    </div>
  )
}
