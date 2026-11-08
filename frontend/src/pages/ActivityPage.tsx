import { useDeferredValue, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Activity, Download, RefreshCw, Search } from 'lucide-react'
import { activityExportURL, listActivity } from '@/api/activity'
import { listServers } from '@/api/servers'
import { PageHeader } from '@/components/PageHeader'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { userFacingError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'

function eventLabel(eventType: string): string {
  return eventType.replaceAll('.', ' ').replaceAll('_', ' ')
}

export function ActivityPage() {
  const [filter, setFilter] = useState('')
  const [eventType, setEventType] = useState('')
  const [outcome, setOutcome] = useState('all')
  const deferredFilter = useDeferredValue(filter)

  const activityQuery = useQuery({
    queryKey: ['activity', deferredFilter, eventType, outcome],
    queryFn: ({ signal }) =>
      listActivity({ q: deferredFilter, eventType, outcome, limit: 200 }, signal),
    refetchInterval: 15000,
    staleTime: 5000,
  })
  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
    staleTime: 30000,
  })

  const serverNames = useMemo(() => {
    const map = new Map<string, string>()
    for (const server of serversQuery.data?.servers ?? []) {
      map.set(server.id, server.display_name)
    }
    return map
  }, [serversQuery.data])

  const events = useMemo(() => activityQuery.data?.events ?? [], [activityQuery.data?.events])

  const downloadExport = (format: 'csv' | 'json') => {
    const url = activityExportURL(
      { q: deferredFilter, eventType, outcome, limit: 5000 },
      format,
    )
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Observe"
        title="Activity"
        description="Audit history for connection, file, process and service actions performed through Drovik. Events never contain credentials, terminal content or log contents."
        actions={
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => downloadExport('csv')}>
              <Download className="h-3.5 w-3.5" aria-hidden="true" />
              CSV
            </Button>
            <Button variant="outline" size="sm" onClick={() => downloadExport('json')}>
              <Download className="h-3.5 w-3.5" aria-hidden="true" />
              JSON
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void activityQuery.refetch()}
              loading={activityQuery.isFetching}
            >
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
              Refresh
            </Button>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative">
          <Search className="text-ink-500 absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2" aria-hidden="true" />
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Search event, target or error"
            className="h-9 w-72 pl-9 text-xs"
            aria-label="Search activity"
          />
        </div>
        <Input
          value={eventType}
          onChange={(event) => setEventType(event.target.value)}
          placeholder="Event type, e.g. auth."
          className="h-9 w-48 font-mono text-xs"
          aria-label="Filter by event type"
        />
        <Select
          value={outcome}
          onChange={(event) => setOutcome(event.target.value)}
          className="h-9 w-36 text-xs"
          aria-label="Filter outcome"
        >
          <option value="all">All outcomes</option>
          <option value="success">Success</option>
          <option value="failure">Failure</option>
          <option value="denied">Denied</option>
          <option value="unknown">Unknown</option>
        </Select>
        <span className="text-ink-500 ml-auto font-mono text-[10px] uppercase">
          {events.length} events (latest 200)
        </span>
      </div>

      {activityQuery.isError ? (
        <Alert variant="error" title="Activity could not be loaded">
          {userFacingError(activityQuery.error)}
        </Alert>
      ) : activityQuery.isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((item) => (
            <Skeleton key={item} className="h-11 w-full" />
          ))}
        </div>
      ) : (
        <Card>
          <CardContent className="px-0 py-0">
            {events.length === 0 ? (
              <div className="p-10 text-center">
                <Activity className="text-ink-500 mx-auto h-8 w-8" aria-hidden="true" />
                <p className="text-ink-300 mt-3 text-sm">No audit events match this filter.</p>
                <p className="text-ink-500 mt-1 text-xs">Sensitive operations appear here after they run.</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500 border-ink-700 border-b font-mono text-[10px] uppercase">
                    <tr>
                      <th className="px-4 py-2 font-medium">Time</th>
                      <th className="px-4 py-2 font-medium">Event</th>
                      <th className="hidden px-4 py-2 font-medium md:table-cell">Server</th>
                      <th className="px-4 py-2 font-medium">Target</th>
                      <th className="px-4 py-2 font-medium">Outcome</th>
                    </tr>
                  </thead>
                  <tbody>
                    {events.map((event) => (
                      <tr key={event.id} className="border-ink-800 hover:bg-ink-850/60 border-b last:border-0">
                        <td className="text-ink-500 px-4 py-2.5 font-mono text-[10px] whitespace-nowrap">
                          {formatDateTime(event.created_at)}
                        </td>
                        <td className="text-ink-100 px-4 py-2.5 font-mono text-[11px]">{eventLabel(event.event_type)}</td>
                        <td className="text-ink-400 hidden max-w-48 truncate px-4 py-2.5 text-xs md:table-cell">
                          {serverNames.get(event.server_id ?? '') ?? (event.server_id ? 'deleted server' : 'account')}
                        </td>
                        <td className="text-ink-300 max-w-80 truncate px-4 py-2.5 font-mono text-[10px]" title={event.target_summary}>
                          {event.target_summary || '—'}
                        </td>
                        <td className="px-4 py-2.5">
                          <Badge
                            variant={
                              event.outcome === 'success'
                                ? 'mint'
                                : event.outcome === 'unknown'
                                  ? 'signal'
                                  : 'rose'
                            }
                          >
                            {event.outcome}
                          </Badge>
                          {event.safe_error_code ? (
                            <p className="text-ink-500 mt-1 font-mono text-[9px]">{event.safe_error_code}</p>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <p className="text-ink-500 text-[11px] leading-5">
        Audit records are scoped to your account and the servers you own, are assigned, or receive
        through a team. They store event type, safe target summary, outcome and a stable error code;
        never secrets or file/log contents. CSV exports prefix spreadsheet formula characters.
      </p>
    </div>
  )
}
