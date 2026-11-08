import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { RefreshCw, ScrollText, Search, Server as ServerIcon } from 'lucide-react'
import { listServices, readLogs } from '@/api/operations'
import { listServers } from '@/api/servers'
import { PageHeader } from '@/components/PageHeader'
import { ServerPicker } from '@/components/ServerPicker'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { userFacingError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'
import { filterLogs, severityVariant } from '@/lib/operations'

const sinceOptions = [
  { value: '15m', label: 'Last 15 minutes' },
  { value: '1h', label: 'Last hour' },
  { value: '24h', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
  { value: 'all', label: 'All available' },
]

const lineOptions = [50, 100, 200, 500]

function formatLogTime(value: string): string {
  if (!value || value.startsWith('0001-')) return '—'
  return formatDateTime(value)
}

export function LogsPage() {
  const { serverId } = useParams<{ serverId?: string }>()
  const navigate = useNavigate()

  const [unit, setUnit] = useState('')
  const [since, setSince] = useState('1h')
  const [lines, setLines] = useState(200)
  const [filter, setFilter] = useState('')

  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const servers = serversQuery.data?.servers ?? []
  const currentServer = servers.find((server) => server.id === serverId)

  const servicesQuery = useQuery({
    queryKey: ['services', serverId],
    queryFn: ({ signal }) => listServices(serverId ?? '', signal),
    enabled: Boolean(serverId),
    retry: 0,
    staleTime: 30000,
  })

  const logsQuery = useQuery({
    queryKey: ['logs', serverId, unit, since, lines],
    queryFn: ({ signal }) =>
      readLogs(serverId ?? '', { unit: unit || undefined, since, lines }, signal),
    enabled: Boolean(serverId),
    retry: 0,
    staleTime: 2000,
  })

  useEffect(() => {
    // oxlint-disable-next-line react/set-state-in-effect -- the unit filter belongs to the selected server
    setUnit('')
    // oxlint-disable-next-line react/set-state-in-effect -- the filter belongs to the selected server
    setFilter('')
  }, [serverId])

  const entries = useMemo(
    () => filterLogs(logsQuery.data?.entries ?? [], filter),
    [logsQuery.data, filter],
  )

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
          eyebrow="Observe"
          title="Logs"
          description="Read a bounded window of the systemd journal as the connected SSH account."
        />
        {servers.length === 0 ? (
          <Card className="p-8 text-center">
            <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
              <ServerIcon className="h-6 w-6" aria-hidden="true" />
            </div>
            <h2 className="font-display text-ink-50 mt-4 text-xl font-semibold">Add a server first</h2>
            <p className="text-ink-400 mt-2 text-sm">
              Journal access uses the saved SSH connection.
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
                    <ScrollText className="text-signal-400 h-4 w-4" aria-hidden="true" />
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
                    onClick={() => navigate(`/logs/${server.id}`)}
                  >
                    Open logs
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

  const units = servicesQuery.data?.units ?? []

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Observe"
        title="Logs"
        description={`Bounded journal window from ${currentServer.host} as ${currentServer.username}. Log text is untrusted and rendered as text only.`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <ServerPicker servers={servers} serverId={serverId} onChange={(id) => navigate(`/logs/${id}`)} />
            <Button
              variant="outline"
              size="sm"
              onClick={() => void logsQuery.refetch()}
              loading={logsQuery.isFetching}
            >
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
              Refresh
            </Button>
          </div>
        }
      />

      <Card>
        <CardContent className="flex flex-wrap items-end gap-3 py-4">
          <label className="space-y-1.5">
            <span className="text-ink-500 block font-mono text-[10px] uppercase">Unit</span>
            <Select value={unit} onChange={(event) => setUnit(event.target.value)} className="h-9 w-64 text-xs" aria-label="Log unit">
              <option value="">System journal</option>
              {units.map((item) => (
                <option key={item.name} value={item.name}>
                  {item.name}
                </option>
              ))}
            </Select>
          </label>
          <label className="space-y-1.5">
            <span className="text-ink-500 block font-mono text-[10px] uppercase">Time range</span>
            <Select value={since} onChange={(event) => setSince(event.target.value)} className="h-9 w-44 text-xs" aria-label="Time range">
              {sinceOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </label>
          <label className="space-y-1.5">
            <span className="text-ink-500 block font-mono text-[10px] uppercase">Lines</span>
            <Select
              value={String(lines)}
              onChange={(event) => setLines(Number(event.target.value))}
              className="h-9 w-28 text-xs"
              aria-label="Line count"
            >
              {lineOptions.map((count) => (
                <option key={count} value={count}>
                  {count}
                </option>
              ))}
            </Select>
          </label>
          <label className="min-w-48 flex-1 space-y-1.5">
            <span className="text-ink-500 block font-mono text-[10px] uppercase">Filter window</span>
            <div className="relative">
              <Search className="text-ink-500 absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2" aria-hidden="true" />
              <Input
                value={filter}
                onChange={(event) => setFilter(event.target.value)}
                placeholder="Search the returned entries"
                className="h-9 pl-9 text-xs"
                aria-label="Filter logs"
              />
            </div>
          </label>
        </CardContent>
      </Card>

      {servicesQuery.isError ? (
        <Alert variant="info" title="Unit list unavailable">
          The journal can still be queried for system logs. The service list failed: {userFacingError(servicesQuery.error)}
        </Alert>
      ) : null}

      {logsQuery.isError ? (
        <Alert variant="error" title="Journal could not be read">
          {userFacingError(logsQuery.error)}
          <p className="text-ink-400 mt-2 text-xs leading-5">
            The connected SSH account may lack journal access, or the host may not use systemd. Drovik
            does not retry with elevated privileges.
          </p>
        </Alert>
      ) : logsQuery.isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5, 6].map((item) => (
            <Skeleton key={item} className="h-10 w-full" />
          ))}
        </div>
      ) : logsQuery.data ? (
        <>
          <div className="text-ink-500 flex flex-wrap items-center gap-4 font-mono text-[10px] uppercase">
            <span>{entries.length} entries shown</span>
            <span>collected {formatDateTime(logsQuery.data.collected_at)}</span>
            {logsQuery.data.truncated ? <Badge variant="signal">window may be truncated</Badge> : null}
          </div>

          <Card>
            <CardContent className="px-0 py-0">
              {entries.length === 0 ? (
                <p className="text-ink-500 p-8 text-center text-sm">
                  No journal entries matched this range and filter.
                </p>
              ) : (
                <div className="max-h-[65vh] overflow-y-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-ink-500 bg-ink-900 border-ink-700 sticky top-0 z-10 border-b font-mono text-[10px] uppercase">
                      <tr>
                        <th className="px-4 py-2 font-medium">Time</th>
                        <th className="px-4 py-2 font-medium">Severity</th>
                        <th className="hidden px-4 py-2 font-medium md:table-cell">Source</th>
                        <th className="px-4 py-2 font-medium">Message</th>
                      </tr>
                    </thead>
                    <tbody>
                      {entries.map((entry, index) => (
                        <tr key={`${entry.timestamp}-${index}`} className="border-ink-800 border-b align-top last:border-0">
                          <td className="text-ink-500 px-4 py-2 font-mono text-[10px] whitespace-nowrap">
                            {formatLogTime(entry.timestamp)}
                          </td>
                          <td className="px-4 py-2">
                            <Badge variant={severityVariant(entry.severity)}>{entry.severity}</Badge>
                          </td>
                          <td className="text-ink-400 hidden px-4 py-2 font-mono text-[10px] md:table-cell">
                            {entry.identifier || entry.unit || 'journal'}
                          </td>
                          <td className="text-ink-200 px-4 py-2 font-mono text-[11px] leading-5 break-words whitespace-pre-wrap">
                            {entry.message}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>

          <div className="border-ink-700 bg-ink-900/60 flex gap-3 rounded-lg border p-4">
            <ScrollText className="text-mint-400 mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="text-ink-400 text-xs leading-5">
              Journal output is bound to the requested line count and command timeout. Drovik does not
              store log contents on the server; this window exists only in the browser response.
            </p>
          </div>
        </>
      ) : null}
    </div>
  )
}
