import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, Plus, RefreshCw, Trash2 } from 'lucide-react'
import {
  deleteAlertRule,
  getHistory,
  listAlertRules,
  saveAlertRule,
  type AlertRule,
  type HistoryPoint,
} from '@/api/advanced'
import { useAuth } from '@/auth/useAuth'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'

const ranges = [
  { value: '1h', label: '1 hour' },
  { value: '24h', label: '24 hours' },
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
]

function ruleID(rule: AlertRule): string {
  return rule.id ?? rule.ID ?? ''
}

function linePath(samples: HistoryPoint[], key: 'cpu_percent' | 'memory_percent' | 'disk_percent'): string {
  const points: string[] = []
  let index = 0
  for (const sample of samples) {
    const value = sample[key]
    if (value === null || value === undefined) {
      index++
      continue
    }
    const x = samples.length <= 1 ? 0 : (index / (samples.length - 1)) * 600
    const y = 160 - Math.min(100, Math.max(0, value)) * 1.5
    points.push(`${x.toFixed(1)},${y.toFixed(1)}`)
    index++
  }
  return points.length > 1 ? `M${points.join(' L')}` : ''
}

export function HistoryPanel({ serverId }: { serverId: string }) {
  const { csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [range, setRange] = useState('1h')
  const [metric, setMetric] = useState('cpu_percent')
  const [comparator, setComparator] = useState('gt')
  const [threshold, setThreshold] = useState('85')
  const [busy, setBusy] = useState(false)

  const historyQuery = useQuery({
    queryKey: ['history', serverId, range],
    queryFn: ({ signal }) => getHistory(serverId, range, signal),
    retry: 0,
  })
  const alertsQuery = useQuery({
    queryKey: ['alert-rules', serverId],
    queryFn: ({ signal }) => listAlertRules(serverId, signal),
    retry: 0,
  })

  const samples = useMemo(() => historyQuery.data?.samples ?? [], [historyQuery.data?.samples])
  const paths = useMemo(
    () => ({
      cpu: linePath(samples, 'cpu_percent'),
      memory: linePath(samples, 'memory_percent'),
      disk: linePath(samples, 'disk_percent'),
    }),
    [samples],
  )

  const addRule = async () => {
    const value = Number(threshold)
    if (!Number.isFinite(value)) return
    setBusy(true)
    try {
      await saveAlertRule(serverId, '', { metric, comparator, threshold: value, enabled: true }, csrfToken)
      toast({ title: 'Alert rule saved', variant: 'success' })
      await queryClient.invalidateQueries({ queryKey: ['alert-rules', serverId] })
      await queryClient.invalidateQueries({ queryKey: ['history', serverId, range] })
    } catch (caught) {
      toast({ title: 'Alert rule failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setBusy(false)
    }
  }

  const removeRule = async (id: string) => {
    try {
      await deleteAlertRule(serverId, id, csrfToken)
      await queryClient.invalidateQueries({ queryKey: ['alert-rules', serverId] })
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    }
  }

  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>History</CardTitle>
          <CardDescription>
            Local metric retention ({historyQuery.data?.retention ?? '—'}). This is not an enterprise
            monitoring time series.
          </CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Select value={range} onChange={(event) => setRange(event.target.value)} className="h-8 w-32 text-xs" aria-label="History range">
            {ranges.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </Select>
          <Button size="sm" variant="outline" onClick={() => void historyQuery.refetch()} loading={historyQuery.isFetching}>
            <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-5 pt-4">
        {historyQuery.isError ? (
          <Alert variant="error" title="History could not be loaded">{userFacingError(historyQuery.error)}</Alert>
        ) : historyQuery.isLoading ? (
          <Skeleton className="h-48 w-full" />
        ) : samples.length === 0 ? (
          <p className="text-ink-500 py-8 text-center text-xs">No retained samples in this window. Open the overview periodically to collect them.</p>
        ) : (
          <>
            <div className="overflow-x-auto">
              <svg viewBox="0 0 600 160" className="border-ink-700 bg-ink-950 h-44 w-full min-w-[36rem] rounded-md border" role="img" aria-label="CPU, memory and disk history">
                {[0, 50, 100].map((value) => (
                  <g key={value}>
                    <line x1="0" x2="600" y1={160 - value * 1.5} y2={160 - value * 1.5} className="stroke-ink-800" strokeWidth="1" />
                    <text x="4" y={156 - value * 1.5} className="fill-ink-600" fontSize="8">{value}%</text>
                  </g>
                ))}
                {paths.cpu ? <path d={paths.cpu} fill="none" className="stroke-signal-400" strokeWidth="2" /> : null}
                {paths.memory ? <path d={paths.memory} fill="none" className="stroke-mint-400" strokeWidth="1.5" /> : null}
                {paths.disk ? <path d={paths.disk} fill="none" className="stroke-rose-400" strokeWidth="1.5" /> : null}
              </svg>
              <div className="text-ink-500 mt-2 flex flex-wrap gap-4 font-mono text-[10px] uppercase">
                <span className="text-signal-300">CPU</span>
                <span className="text-mint-300">Memory</span>
                <span className="text-rose-300">Disk max</span>
                <span>{samples.length} samples</span>
              </div>
            </div>
          </>
        )}

        <div className="border-ink-700 border-t pt-4">
          <div className="flex items-center gap-2">
            <Bell className="text-signal-400 h-4 w-4" aria-hidden="true" />
            <h3 className="text-ink-200 text-sm font-medium">Threshold rules</h3>
            <span className="text-ink-500 text-[10px]">in-app only; external notifications are not configured</span>
          </div>
          <div className="mt-3 space-y-2">
            {(alertsQuery.data?.rules ?? []).map((rule) => (
              <div key={ruleID(rule)} className="border-ink-800 flex items-center gap-3 rounded-md border px-3 py-2 text-xs">
                <Badge variant="outline">{rule.metric}</Badge>
                <span className="text-ink-300 font-mono text-[10px]">{rule.comparator === 'gt' ? '>' : '≥'} {rule.threshold}</span>
                <span className="text-ink-500 ml-auto font-mono text-[9px]">{rule.enabled ? 'enabled' : 'disabled'}</span>
                <Button size="icon" variant="ghost" className="hover:text-rose-400 h-6 w-6" aria-label="Delete alert rule" onClick={() => void removeRule(ruleID(rule))}>
                  <Trash2 className="h-3 w-3" aria-hidden="true" />
                </Button>
              </div>
            ))}
            {alertsQuery.isError ? <p className="text-ink-500 text-xs">Alert rules could not be loaded.</p> : null}
          </div>
          <div className="mt-3 flex flex-wrap items-end gap-2">
            <Select value={metric} onChange={(event) => setMetric(event.target.value)} className="h-8 w-36 text-xs" aria-label="Alert metric">
              <option value="cpu_percent">CPU %</option>
              <option value="memory_percent">Memory %</option>
              <option value="disk_percent">Disk %</option>
              <option value="load_one">Load 1m</option>
            </Select>
            <Select value={comparator} onChange={(event) => setComparator(event.target.value)} className="h-8 w-24 text-xs" aria-label="Alert comparator">
              <option value="gt">greater than</option>
              <option value="gte">at least</option>
            </Select>
            <Input value={threshold} onChange={(event) => setThreshold(event.target.value)} type="number" className="h-8 w-24 text-xs" aria-label="Alert threshold" />
            <Button size="sm" variant="outline" onClick={() => void addRule()} loading={busy}>
              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
              Add rule
            </Button>
          </div>
          {(historyQuery.data?.alerts ?? []).filter((alert) => alert.active).length > 0 ? (
            <Alert variant="warning" title="Active threshold" >
              {historyQuery.data?.alerts.filter((alert) => alert.active).map((alert) => `${alert.metric} = ${alert.value.toFixed(1)} (threshold ${alert.threshold})`).join(' · ')}
            </Alert>
          ) : null}
        </div>

        {historyQuery.data?.samples.length ? (
          <p className="text-ink-600 font-mono text-[9px]">Latest sample {formatDateTime(historyQuery.data.samples[historyQuery.data.samples.length - 1]?.collected_at ?? '')}</p>
        ) : null}
      </CardContent>
    </Card>
  )
}
