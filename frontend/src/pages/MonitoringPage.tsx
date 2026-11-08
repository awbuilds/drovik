import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Activity,
  ArrowDown,
  ArrowUp,
  Cpu,
  Database,
  Gauge,
  HardDrive,
  MemoryStick,
  Network,
  RefreshCw,
  Server as ServerIcon,
  Timer,
} from 'lucide-react'
import { getServerOverview } from '@/api/monitoring'
import { useAuth } from '@/auth/useAuth'
import { listServers } from '@/api/servers'
import { PageHeader } from '@/components/PageHeader'
import { HistoryPanel } from '@/components/monitoring/HistoryPanel'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { StatusDot } from '@/components/ui/StatusDot'
import { userFacingError } from '@/lib/errors'
import { formatBytes } from '@/lib/files'
import { formatDateTime } from '@/lib/format'
import { formatPercent, formatRate, formatUptime } from '@/lib/metrics'

function toneForPercent(value: number | null | undefined): string {
  if (value === null || value === undefined) return 'bg-ink-600'
  if (value >= 90) return 'bg-rose-400'
  if (value >= 75) return 'bg-signal-400'
  return 'bg-mint-400'
}

function UsageBar({ value, label }: { value: number | null | undefined; label: string }) {
  const width = value === null || value === undefined ? 0 : Math.min(100, Math.max(0, value))
  return (
    <div
      className="bg-ink-800 h-1.5 w-full overflow-hidden rounded-full"
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={value ?? undefined}
    >
      <div
        className={`h-full rounded-full transition-[width] duration-500 motion-reduce:transition-none ${toneForPercent(value)}`}
        style={{ width: `${width}%` }}
      />
    </div>
  )
}

function MetricCard({
  label,
  value,
  detail,
  icon: Icon,
  children,
}: {
  label: string
  value: string
  detail: string
  icon: typeof Cpu
  children?: ReactNode
}) {
  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">{label}</p>
          <p className="font-display text-ink-50 mt-2 text-2xl font-semibold tabular-nums">
            {value}
          </p>
          <p className="text-ink-500 mt-1 text-xs">{detail}</p>
        </div>
        <div className="bg-ink-800 text-signal-300 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg">
          <Icon className="h-4 w-4" aria-hidden="true" />
        </div>
      </div>
      {children ? <div className="mt-4">{children}</div> : null}
    </Card>
  )
}

export function MonitoringPage() {
  const { serverId } = useParams<{ serverId?: string }>()
  const navigate = useNavigate()
  const { user } = useAuth()

  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const servers = serversQuery.data?.servers ?? []
  const currentServer = servers.find((server) => server.id === serverId)

  const overviewQuery = useQuery({
    queryKey: ['overview', serverId],
    queryFn: ({ signal }) => getServerOverview(serverId ?? '', signal),
    enabled: Boolean(serverId),
    refetchInterval: 5000,
    staleTime: 2000,
    retry: 0,
  })

  const snapshot = overviewQuery.data

  if (serversQuery.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-40 w-full" />
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {[1, 2, 3, 4].map((item) => (
            <Skeleton key={item} className="h-32 w-full" />
          ))}
        </div>
      </div>
    )
  }

  if (serversQuery.isError) {
    return (
      <Alert variant="error" title="Servers could not be loaded">
        {userFacingError(serversQuery.error)}
      </Alert>
    )
  }

  if (servers.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Card className="max-w-md p-8 text-center">
          <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
            <ServerIcon className="h-6 w-6" aria-hidden="true" />
          </div>
          <h1 className="font-display text-ink-50 mt-4 text-xl font-semibold">
            Add a server first
          </h1>
          <p className="text-ink-400 mt-2 text-sm">
            Overview reads live kernel counters over SSH. Save and verify a server before collecting
            metrics.
          </p>
          <Button asChild className="mt-5">
            <Link to="/servers">Go to servers</Link>
          </Button>
        </Card>
      </div>
    )
  }

  if (serverId && !currentServer) {
    return (
      <Alert variant="error" title="Server not found">
        This server does not exist or is not assigned to your account.
      </Alert>
    )
  }

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Observe"
        title="Host overview"
        description="Live CPU, memory, disk, network and uptime collected over SSH. No agent runs on the managed host."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Select
              aria-label="Select server"
              value={serverId ?? ''}
              onChange={(event) => {
                if (event.target.value) navigate(`/monitoring/${event.target.value}`)
              }}
              className="w-56"
            >
              <option value="">Select a server…</option>
              {servers.map((server) => (
                <option key={server.id} value={server.id}>
                  {server.display_name} · {server.host}
                </option>
              ))}
            </Select>
            {serverId ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => void overviewQuery.refetch()}
                loading={overviewQuery.isFetching}
              >
                <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
                Refresh
              </Button>
            ) : null}
            <Badge variant="mint">5s auto</Badge>
          </div>
        }
      />

      {!serverId ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {servers.map((server) => (
            <Card key={server.id}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Gauge className="text-signal-400 h-4 w-4" aria-hidden="true" />
                  {server.display_name}
                </CardTitle>
                <CardDescription>
                  {server.username}@{server.host}:{server.port}
                </CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                <Button
                  variant="outline"
                  className="w-full"
                  onClick={() => navigate(`/monitoring/${server.id}`)}
                >
                  Open overview
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : overviewQuery.isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-28 w-full" />
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {[1, 2, 3, 4].map((item) => (
              <Skeleton key={item} className="h-32 w-full" />
            ))}
          </div>
          <Skeleton className="h-64 w-full" />
        </div>
      ) : overviewQuery.isError ? (
        <Alert variant="error" title="Metrics could not be collected">
          {userFacingError(overviewQuery.error)}
          <div className="mt-3">
            <Button size="sm" variant="outline" onClick={() => void overviewQuery.refetch()}>
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
              Try again
            </Button>
          </div>
        </Alert>
      ) : snapshot && currentServer ? (
        <>
          {snapshot.stale ? (
            <Alert variant="warning" title="Showing a stale sample">
              {snapshot.warning ?? 'Live metrics could not be refreshed.'}
            </Alert>
          ) : null}
          {snapshot.host.os && snapshot.host.os !== 'Linux' ? (
            <Alert variant="info" title={`Read-only ${snapshot.host.os} monitoring`}>
              Host, load, memory, disk, network and uptime are collected without an agent. CPU
              usage is a point-in-time sample; systemd, journal and Linux-only panel actions are
              unavailable on this host.
            </Alert>
          ) : null}

          <Card>
            <CardContent className="flex flex-wrap items-center gap-x-8 gap-y-4 py-5">
              <div className="flex min-w-0 items-center gap-3">
                <div className="bg-ink-800 text-signal-300 flex h-11 w-11 shrink-0 items-center justify-center rounded-xl">
                  <Activity className="h-5 w-5" aria-hidden="true" />
                </div>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="font-display text-ink-50 truncate text-lg font-semibold">
                      {snapshot.host.hostname || currentServer.display_name}
                    </h2>
                    <StatusDot
                      status={snapshot.stale ? 'warning' : 'online'}
                      label={snapshot.stale ? 'Stale' : 'Live'}
                    />
                    {snapshot.cached ? <Badge variant="outline">cached</Badge> : null}
                  </div>
                  <p className="text-ink-400 mt-1 truncate text-xs">
                    {snapshot.host.distribution || snapshot.host.os} · kernel{' '}
                    {snapshot.host.kernel || 'unknown'} · {snapshot.host.architecture || 'unknown'}
                  </p>
                </div>
              </div>
              <div className="text-ink-500 ml-auto font-mono text-[10px] uppercase">
                <p>Collected {formatDateTime(snapshot.collected_at)}</p>
                {snapshot.uptime.available ? (
                  <p className="mt-1">Booted {formatDateTime(snapshot.uptime.boot_time)}</p>
                ) : null}
              </div>
            </CardContent>
          </Card>

          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              label="CPU"
              value={
                snapshot.cpu.available ? formatPercent(snapshot.cpu.usage_percent) : 'Unavailable'
              }
              detail={
                snapshot.cpu.available
                  ? snapshot.cpu.model || `${snapshot.cpu.cores.length} logical cores`
                  : 'No readable /proc/stat counters'
              }
              icon={Cpu}
            >
              {snapshot.cpu.available ? (
                <>
                  <UsageBar value={snapshot.cpu.usage_percent} label="CPU usage" />
                  {snapshot.cpu.usage_percent === null ? (
                    <p className="text-ink-500 mt-2 text-[11px]">Collecting the first CPU baseline…</p>
                  ) : null}
                </>
              ) : null}
            </MetricCard>
            <MetricCard
              label="Load average"
              value={snapshot.load.available ? snapshot.load.one.toFixed(2) : 'Unavailable'}
              detail={
                snapshot.load.available
                  ? `${snapshot.load.five.toFixed(2)} · ${snapshot.load.fifteen.toFixed(2)} · ${snapshot.load.runnable}/${snapshot.load.total} runnable`
                  : '/proc/loadavg is not readable'
              }
              icon={Gauge}
            />
            <MetricCard
              label="Memory"
              value={
                snapshot.memory.available ? formatPercent(snapshot.memory.used_percent) : 'Unavailable'
              }
              detail={
                snapshot.memory.available
                  ? `${formatBytes(snapshot.memory.used_bytes)} of ${formatBytes(snapshot.memory.total_bytes)}`
                  : '/proc/meminfo is not readable'
              }
              icon={MemoryStick}
            >
              {snapshot.memory.available ? (
                <>
                  <UsageBar value={snapshot.memory.used_percent} label="Memory usage" />
                  {snapshot.memory.swap_total_bytes > 0 ? (
                    <p className="text-ink-500 mt-2 font-mono text-[10px]">
                      swap {formatBytes(snapshot.memory.swap_used_bytes)} /{' '}
                      {formatBytes(snapshot.memory.swap_total_bytes)}
                    </p>
                  ) : (
                    <p className="text-ink-500 mt-2 font-mono text-[10px]">swap not configured</p>
                  )}
                </>
              ) : null}
            </MetricCard>
            <MetricCard
              label="Uptime"
              value={snapshot.uptime.available ? formatUptime(snapshot.uptime.seconds) : 'Unavailable'}
              detail={`${snapshot.filesystems.length} filesystems · ${snapshot.network.length} interfaces`}
              icon={Timer}
            />
          </div>

          <div className="grid gap-4 xl:grid-cols-[1.1fr_1fr]">
            <Card>
              <CardHeader>
                <CardTitle>Per-core CPU</CardTitle>
                <CardDescription>Per-core counters where the host exposes them.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3 pt-4">
                {!snapshot.cpu.available ? (
                  <p className="text-ink-500 text-xs">
                    Per-core CPU counters are unavailable on this host.
                  </p>
                ) : snapshot.cpu.cores.length === 0 ? (
                  <p className="text-ink-500 text-xs">
                    This host did not report per-core counters.
                  </p>
                ) : (
                  snapshot.cpu.cores.map((core) => (
                    <div key={core.index} className="flex items-center gap-3">
                      <span className="text-ink-500 w-12 font-mono text-[10px] uppercase">
                        cpu{core.index}
                      </span>
                      <div className="flex-1">
                        <UsageBar value={core.usage_percent} label={`CPU core ${core.index}`} />
                      </div>
                      <span className="text-ink-200 w-14 text-right font-mono text-[11px] tabular-nums">
                        {formatPercent(core.usage_percent)}
                      </span>
                    </div>
                  ))
                )}
                <p className="text-ink-600 border-ink-700 border-t pt-3 font-mono text-[10px] leading-4">
                  CPU and network rates need two samples. The first request establishes the
                  baseline.
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Memory detail</CardTitle>
                <CardDescription>Kernel memory counters.</CardDescription>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-4 pt-4">
                {[
                  { label: 'Total', value: snapshot.memory.total_bytes },
                  { label: 'Available', value: snapshot.memory.available_bytes },
                  { label: 'Used', value: snapshot.memory.used_bytes },
                  { label: 'Free', value: snapshot.memory.free_bytes },
                  { label: 'Buffers', value: snapshot.memory.buffers_bytes },
                  { label: 'Cached', value: snapshot.memory.cached_bytes },
                ].map((item) => (
                  <div key={item.label} className="border-ink-800 rounded-md border px-3 py-2">
                    <p className="text-ink-500 font-mono text-[10px] uppercase">{item.label}</p>
                    <p className="text-ink-100 mt-1 font-mono text-xs tabular-nums">
                      {formatBytes(item.value)}
                    </p>
                  </div>
                ))}
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader className="flex-row items-start justify-between gap-3">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <HardDrive className="text-signal-400 h-4 w-4" aria-hidden="true" />
                  Filesystems
                </CardTitle>
                <CardDescription>Real mounted filesystems, sorted by mountpoint.</CardDescription>
              </div>
              {snapshot.filesystems_truncated ? <Badge variant="outline">truncated</Badge> : null}
            </CardHeader>
            <CardContent className="px-0 pt-2">
              {!snapshot.filesystems_available ? (
                <p className="text-ink-500 px-5 py-6 text-xs">
                  Filesystem usage is unavailable: df did not return readable output.
                </p>
              ) : snapshot.filesystems.length === 0 ? (
                <p className="text-ink-500 px-5 py-6 text-xs">No real filesystems were reported.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase">
                      <tr>
                        <th className="px-5 py-2 font-medium">Mount</th>
                        <th className="hidden px-5 py-2 font-medium sm:table-cell">Device</th>
                        <th className="hidden px-5 py-2 font-medium md:table-cell">Type</th>
                        <th className="px-5 py-2 font-medium">Used</th>
                        <th className="px-5 py-2 text-right font-medium">Size</th>
                      </tr>
                    </thead>
                    <tbody>
                      {snapshot.filesystems.map((filesystem) => (
                        <tr
                          key={filesystem.mountpoint}
                          className="border-ink-800 border-b last:border-0"
                        >
                          <td className="px-5 py-3">
                            <div className="flex items-center gap-2">
                              <span className="text-ink-100 font-mono text-xs">
                                {filesystem.mountpoint}
                              </span>
                              {filesystem.read_only ? <Badge variant="outline">ro</Badge> : null}
                            </div>
                            <div className="mt-2 max-w-56">
                              <UsageBar
                                value={filesystem.used_percent}
                                label={`Usage for ${filesystem.mountpoint}`}
                              />
                            </div>
                          </td>
                          <td className="text-ink-400 hidden max-w-56 truncate px-5 py-3 font-mono text-[11px] sm:table-cell">
                            {filesystem.device}
                          </td>
                          <td className="text-ink-500 hidden px-5 py-3 font-mono text-[11px] md:table-cell">
                            {filesystem.fstype || '—'}
                          </td>
                          <td className="text-ink-200 px-5 py-3 font-mono text-[11px] tabular-nums">
                            {formatPercent(filesystem.used_percent)}
                            <span className="text-ink-500 ml-2">
                              {formatBytes(filesystem.used_bytes)}
                            </span>
                          </td>
                          <td className="text-ink-400 px-5 py-3 text-right font-mono text-[11px] tabular-nums">
                            {formatBytes(filesystem.total_bytes)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex-row items-start justify-between gap-3">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Network className="text-signal-400 h-4 w-4" aria-hidden="true" />
                  Network
                </CardTitle>
                <CardDescription>
                  Interface totals and rates since the previous sample.
                </CardDescription>
              </div>
              {snapshot.network_truncated ? <Badge variant="outline">truncated</Badge> : null}
            </CardHeader>
            <CardContent className="px-0 pt-2">
              {!snapshot.network_available ? (
                <p className="text-ink-500 px-5 py-6 text-xs">
                  Network counters are unavailable: /proc/net/dev is not readable.
                </p>
              ) : snapshot.network.length === 0 ? (
                <p className="text-ink-500 px-5 py-6 text-xs">
                  No network interfaces were reported.
                </p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase">
                      <tr>
                        <th className="px-5 py-2 font-medium">Interface</th>
                        <th className="px-5 py-2 font-medium">Receive</th>
                        <th className="px-5 py-2 font-medium">Transmit</th>
                        <th className="hidden px-5 py-2 text-right font-medium sm:table-cell">
                          Total
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {snapshot.network.map((iface) => (
                        <tr key={iface.name} className="border-ink-800 border-b last:border-0">
                          <td className="text-ink-100 px-5 py-3 font-mono text-xs">{iface.name}</td>
                          <td className="px-5 py-3">
                            <span className="text-mint-300 flex items-center gap-1 font-mono text-[11px] tabular-nums">
                              <ArrowDown className="h-3 w-3" aria-hidden="true" />
                              {formatRate(iface.receive_rate_bps)}
                            </span>
                          </td>
                          <td className="px-5 py-3">
                            <span className="text-signal-300 flex items-center gap-1 font-mono text-[11px] tabular-nums">
                              <ArrowUp className="h-3 w-3" aria-hidden="true" />
                              {formatRate(iface.transmit_rate_bps)}
                            </span>
                          </td>
                          <td className="text-ink-500 hidden px-5 py-3 text-right font-mono text-[10px] tabular-nums sm:table-cell">
                            {formatBytes(iface.received_bytes)} in ·{' '}
                            {formatBytes(iface.transmitted_bytes)} out
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>

          <HistoryPanel
            serverId={serverId}
            canWrite={user?.role !== 'read_only' && currentServer?.access_level !== 'read'}
          />

          <div className="border-ink-700 bg-ink-900/60 flex gap-3 rounded-lg border p-4">
            <Database className="text-mint-400 mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="text-ink-400 text-xs leading-5">
              Metrics are read from <span className="font-mono">/proc</span> and{' '}
              <span className="font-mono">df</span> over a short-lived SSH connection, cached per
              host for the minimum interval. A bounded local history is retained only when the
              optional retention window is enabled.
            </p>
          </div>
        </>
      ) : (
        <Alert variant="info" title="No metrics yet">
          Select a server to collect a snapshot.
        </Alert>
      )}
    </div>
  )
}
