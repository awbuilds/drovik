import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ListTree, RefreshCw, Search, Server as ServerIcon, Skull } from 'lucide-react'
import { signalProcess, listProcesses, type ProcessInfo, type ProcessSignal } from '@/api/operations'
import { listServers } from '@/api/servers'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ServerPicker } from '@/components/ServerPicker'
import { SignalProcessDialog } from '@/components/operations/SignalProcessDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { StatusDot } from '@/components/ui/StatusDot'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatBytes } from '@/lib/files'
import { formatDateTime } from '@/lib/format'
import { formatUptime } from '@/lib/metrics'
import { processHealth } from '@/lib/operations'

type SortKey = 'cpu' | 'memory' | 'pid' | 'name'

export function ProcessesPage() {
  const { serverId } = useParams<{ serverId?: string }>()
  const navigate = useNavigate()
  const { csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [filter, setFilter] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('cpu')
  const [target, setTarget] = useState<ProcessInfo | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const servers = serversQuery.data?.servers ?? []
  const currentServer = servers.find((server) => server.id === serverId)

  const processesQuery = useQuery({
    queryKey: ['processes', serverId],
    queryFn: ({ signal }) => listProcesses(serverId ?? '', signal),
    enabled: Boolean(serverId),
    refetchInterval: 10000,
    staleTime: 3000,
    retry: 0,
  })

  const visible = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    const list = (processesQuery.data?.processes ?? []).filter((process) => {
      return (
        !needle ||
        process.command.toLowerCase().includes(needle) ||
        process.user.toLowerCase().includes(needle) ||
        String(process.pid) === needle
      )
    })
    return [...list].sort((left, right) => {
      switch (sortKey) {
        case 'memory':
          return right.memory_percent - left.memory_percent
        case 'pid':
          return left.pid - right.pid
        case 'name':
          return left.command.localeCompare(right.command, undefined, { sensitivity: 'base' })
        default:
          return right.cpu_percent - left.cpu_percent
      }
    })
  }, [processesQuery.data, filter, sortKey])

  const handleSignal = async (signal: ProcessSignal) => {
    if (!serverId || !target) return
    setBusy(true)
    setActionError(null)
    try {
      const result = await signalProcess(serverId, target.pid, signal, target.started_at, csrfToken)
      if (!result.delivered) {
        setActionError('The signal was not delivered.')
        return
      }
      setTarget(null)
      toast({
        title: result.outcome_unknown
          ? 'Signal sent; state could not be verified'
          : result.running
            ? 'Signal sent; process still running'
            : 'Process signaled',
        description: result.outcome_unknown
          ? `PID ${target.pid} received the signal, but its state could not be read. Refresh to re-check.`
          : result.running
            ? `PID ${target.pid} is still ${result.state ?? 'running'}. Refresh to re-check.`
            : `PID ${target.pid} is no longer running.`,
        variant: result.outcome_unknown ? 'info' : result.running ? 'info' : 'success',
      })
      await queryClient.invalidateQueries({ queryKey: ['processes', serverId] })
      if (result.running) {
        setFilter(String(target.pid))
      }
    } catch (caught) {
      setActionError(userFacingError(caught))
    } finally {
      setBusy(false)
    }
  }

  if (serversQuery.isLoading) {
    return <Skeleton className="h-[60vh] w-full" />
  }
  if (serversQuery.isError) {
    return (
      <Alert variant="error" title="Servers could not be loaded">
        {userFacingError(serversQuery.error)}
      </Alert>
    )
  }

  if (!serverId) {
    return (
      <div className="space-y-5">
        <PageHeader
          eyebrow="Operate"
          title="Processes"
          description="Inspect processes on a managed host and send enumerated signals as the connected SSH account."
        />
        {servers.length === 0 ? (
          <Card className="p-8 text-center">
            <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
              <ServerIcon className="h-6 w-6" aria-hidden="true" />
            </div>
            <h2 className="font-display text-ink-50 mt-4 text-xl font-semibold">Add a server first</h2>
            <p className="text-ink-400 mt-2 text-sm">
              Process inspection uses the saved SSH connection.
            </p>
            <Button asChild className="mt-5">
              <Link to="/servers">Go to servers</Link>
            </Button>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {servers.map((server) => (
              <Card key={server.id}>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <ListTree className="text-signal-400 h-4 w-4" aria-hidden="true" />
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
                    onClick={() => navigate(`/processes/${server.id}`)}
                  >
                    Inspect processes
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    )
  }

  if (!currentServer) {
    return (
      <Alert variant="error" title="Server not found">
        This server does not exist or is not assigned to your account.
      </Alert>
    )
  }

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Operate"
        title="Processes"
        description={`Running as ${currentServer.username} on ${currentServer.host}. Signals are subject to remote operating-system permissions.`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <ServerPicker servers={servers} serverId={serverId} onChange={(id) => navigate(`/processes/${id}`)} />
            <Button
              variant="outline"
              size="sm"
              onClick={() => void processesQuery.refetch()}
              loading={processesQuery.isFetching}
            >
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
              Refresh
            </Button>
            <Badge variant="mint">10s auto</Badge>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative">
          <Search className="text-ink-500 absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2" aria-hidden="true" />
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Filter by PID, user or command"
            className="h-9 w-64 pl-9 text-xs"
            aria-label="Filter processes"
          />
        </div>
        <Select
          value={sortKey}
          onChange={(event) => setSortKey(event.target.value as SortKey)}
          className="h-9 w-36 text-xs"
          aria-label="Sort processes"
        >
          <option value="cpu">CPU %</option>
          <option value="memory">Memory %</option>
          <option value="pid">PID</option>
          <option value="name">Command</option>
        </Select>
        <span className="text-ink-500 ml-auto font-mono text-[10px] uppercase">
          {visible.length} of {processesQuery.data?.processes.length ?? 0} processes
        </span>
      </div>

      {processesQuery.isError ? (
        <Alert variant="error" title="Processes could not be listed">
          {userFacingError(processesQuery.error)}
        </Alert>
      ) : processesQuery.isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((item) => (
            <Skeleton key={item} className="h-12 w-full" />
          ))}
        </div>
      ) : processesQuery.data ? (
        <>
          <div className="text-ink-500 flex flex-wrap gap-4 font-mono text-[10px] uppercase">
            <span>account {processesQuery.data.account}</span>
            <span>collected {formatDateTime(processesQuery.data.collected_at)}</span>
            {processesQuery.data.truncated ? <span className="text-signal-400">listing truncated</span> : null}
          </div>
          <Card>
            <CardContent className="px-0 py-0">
              {visible.length === 0 ? (
                <p className="text-ink-500 p-8 text-center text-sm">No processes match this filter.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-ink-500 border-ink-700 border-b font-mono text-[10px] uppercase">
                      <tr>
                        <th className="px-4 py-2 font-medium">PID</th>
                        <th className="px-4 py-2 font-medium">User</th>
                        <th className="px-4 py-2 font-medium">State</th>
                        <th className="px-4 py-2 text-right font-medium">CPU</th>
                        <th className="px-4 py-2 text-right font-medium">Mem</th>
                        <th className="hidden px-4 py-2 text-right font-medium lg:table-cell">RSS</th>
                        <th className="hidden px-4 py-2 text-right font-medium md:table-cell">Uptime</th>
                        <th className="px-4 py-2 font-medium">Command</th>
                        <th className="px-4 py-2 text-right font-medium">Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visible.map((process) => (
                        <tr key={process.pid} className="border-ink-800 hover:bg-ink-850/60 border-b last:border-0">
                          <td className="text-ink-300 px-4 py-2.5 font-mono text-xs tabular-nums">{process.pid}</td>
                          <td className="text-ink-400 px-4 py-2.5 font-mono text-[11px]">{process.user}</td>
                          <td className="px-4 py-2.5">
                            <StatusDot status={processHealth(process.state)} label={process.state || 'unknown'} />
                          </td>
                          <td className="text-ink-200 px-4 py-2.5 text-right font-mono text-[11px] tabular-nums">
                            {process.cpu_percent.toFixed(1)}%
                          </td>
                          <td className="text-ink-200 px-4 py-2.5 text-right font-mono text-[11px] tabular-nums">
                            {process.memory_percent.toFixed(1)}%
                          </td>
                          <td className="text-ink-500 hidden px-4 py-2.5 text-right font-mono text-[10px] tabular-nums lg:table-cell">
                            {formatBytes(process.rss_bytes)}
                          </td>
                          <td className="text-ink-500 hidden px-4 py-2.5 text-right font-mono text-[10px] md:table-cell">
                            {formatUptime(process.elapsed_seconds)}
                          </td>
                          <td className="text-ink-100 max-w-[28rem] truncate px-4 py-2.5 font-mono text-[11px]">
                            {process.command || '—'}
                          </td>
                          <td className="px-4 py-2.5 text-right">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="hover:text-rose-400 h-7 w-7"
                              aria-label={`Signal process ${process.pid}`}
                              disabled={process.pid <= 1}
                              onClick={() => {
                                setActionError(null)
                                setTarget(process)
                              }}
                            >
                              <Skull className="h-3.5 w-3.5" aria-hidden="true" />
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
          <p className="text-ink-500 text-[11px] leading-5">
            CPU and memory percentages are the lifetime averages reported by <span className="font-mono">ps</span>.
            PID 1 is never signaled from this interface.
          </p>
        </>
      ) : null}

      <SignalProcessDialog
        open={target !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTarget(null)
            setActionError(null)
          }
        }}
        process={target}
        busy={busy}
        error={actionError}
        onConfirm={(signal) => void handleSignal(signal)}
      />
    </div>
  )
}
