import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  CirclePlay,
  CircleStop,
  Power,
  PowerOff,
  RefreshCw,
  RotateCw,
  Search,
  Server as ServerIcon,
  Settings2,
} from 'lucide-react'
import {
  listServices,
  runServiceAction,
  type ServiceAction,
  type ServiceUnit,
} from '@/api/operations'
import { listServers } from '@/api/servers'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ServerPicker } from '@/components/ServerPicker'
import { ServiceActionDialog } from '@/components/operations/ServiceActionDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { StatusDot } from '@/components/ui/StatusDot'
import { Tooltip } from '@/components/ui/Tooltip'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'
import { serviceHealth } from '@/lib/operations'

interface ActionTarget {
  unit: ServiceUnit
  action: ServiceAction
}

export function ServicesPage() {
  const { serverId } = useParams<{ serverId?: string }>()
  const navigate = useNavigate()
  const { csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [filter, setFilter] = useState('')
  const [target, setTarget] = useState<ActionTarget | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

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
    refetchInterval: 15000,
    staleTime: 3000,
    retry: 0,
  })

  const units = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    return (servicesQuery.data?.units ?? []).filter((unit) => {
      return (
        !needle ||
        unit.name.toLowerCase().includes(needle) ||
        unit.description.toLowerCase().includes(needle) ||
        unit.active.toLowerCase().includes(needle)
      )
    })
  }, [servicesQuery.data, filter])

  const requestAction = (unit: ServiceUnit, action: ServiceAction) => {
    setActionError(null)
    setTarget({ unit, action })
  }

  const handleAction = async () => {
    if (!serverId || !target) return
    setBusy(true)
    setActionError(null)
    try {
      const result = await runServiceAction(serverId, target.unit.name, target.action, csrfToken)
      setTarget(null)
      toast({
        title: result.state_known ? `Service ${target.action} completed` : 'Action completed; state unknown',
        description: result.state_known
          ? `${result.unit} · ${result.active_state}/${result.sub_state}`
          : result.message,
        variant: result.state_known ? 'success' : 'info',
      })
      await queryClient.invalidateQueries({ queryKey: ['services', serverId] })
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
          title="Services"
          description="Inspect systemd units and perform enumerated actions as the connected SSH account."
        />
        {servers.length === 0 ? (
          <Card className="p-8 text-center">
            <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
              <ServerIcon className="h-6 w-6" aria-hidden="true" />
            </div>
            <h2 className="font-display text-ink-50 mt-4 text-xl font-semibold">Add a server first</h2>
            <p className="text-ink-400 mt-2 text-sm">
              Service management uses the saved SSH connection and systemd.
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
                    <Settings2 className="text-signal-400 h-4 w-4" aria-hidden="true" />
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
                    onClick={() => navigate(`/services/${server.id}`)}
                  >
                    Inspect services
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
        title="Services"
        description={`systemd units on ${currentServer.host} as ${currentServer.username}. Drovik never escalates privileges.`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <ServerPicker servers={servers} serverId={serverId} onChange={(id) => navigate(`/services/${id}`)} />
            <Button
              variant="outline"
              size="sm"
              onClick={() => void servicesQuery.refetch()}
              loading={servicesQuery.isFetching}
            >
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
              Refresh
            </Button>
            <Badge variant="mint">15s auto</Badge>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative">
          <Search className="text-ink-500 absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2" aria-hidden="true" />
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Filter unit or description"
            className="h-9 w-72 pl-9 text-xs"
            aria-label="Filter services"
          />
        </div>
        <span className="text-ink-500 ml-auto font-mono text-[10px] uppercase">
          {units.length} of {servicesQuery.data?.units.length ?? 0} units
        </span>
      </div>

      {servicesQuery.isError ? (
        <Alert variant="error" title="systemd is unavailable or inaccessible">
          {userFacingError(servicesQuery.error)}
          <p className="text-ink-400 mt-2 text-xs leading-5">
            The host may not use systemd, or the connected SSH account cannot query units. Drovik
            does not retry with elevated privileges.
          </p>
        </Alert>
      ) : servicesQuery.isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((item) => (
            <Skeleton key={item} className="h-12 w-full" />
          ))}
        </div>
      ) : servicesQuery.data ? (
        <>
          <div className="text-ink-500 flex flex-wrap gap-4 font-mono text-[10px] uppercase">
            <span>collected {formatDateTime(servicesQuery.data.collected_at)}</span>
            {servicesQuery.data.truncated ? <span className="text-signal-400">listing truncated</span> : null}
          </div>
          <Card>
            <CardContent className="px-0 py-0">
              {units.length === 0 ? (
                <p className="text-ink-500 p-8 text-center text-sm">No units match this filter.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-ink-500 border-ink-700 border-b font-mono text-[10px] uppercase">
                      <tr>
                        <th className="px-4 py-2 font-medium">Unit</th>
                        <th className="px-4 py-2 font-medium">State</th>
                        <th className="hidden px-4 py-2 font-medium md:table-cell">Boot</th>
                        <th className="px-4 py-2 text-right font-medium">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {units.map((unit) => {
                        const running = unit.active === 'active'
                        return (
                          <tr key={unit.name} className="border-ink-800 hover:bg-ink-850/60 border-b last:border-0">
                            <td className="max-w-96 px-4 py-3">
                              <p className="text-ink-100 truncate font-mono text-xs">{unit.name}</p>
                              <p className="text-ink-500 mt-0.5 truncate text-[11px]">{unit.description || 'No description'}</p>
                            </td>
                            <td className="px-4 py-3">
                              <StatusDot status={serviceHealth(unit.active)} label={unit.active || 'unknown'} />
                              <p className="text-ink-600 mt-1 font-mono text-[9px]">{unit.sub}</p>
                            </td>
                            <td className="hidden px-4 py-3 md:table-cell">
                              <Badge variant={unit.file_state === 'enabled' ? 'mint' : 'outline'}>
                                {unit.file_state || 'unknown'}
                              </Badge>
                            </td>
                            <td className="px-4 py-3">
                              <div className="flex items-center justify-end gap-1">
                                {!running ? (
                                  <Tooltip label="Start">
                                    <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Start ${unit.name}`} onClick={() => requestAction(unit, 'start')}>
                                      <CirclePlay className="h-3.5 w-3.5" aria-hidden="true" />
                                    </Button>
                                  </Tooltip>
                                ) : (
                                  <>
                                    <Tooltip label="Stop">
                                      <Button size="icon" variant="ghost" className="hover:text-rose-400 h-7 w-7" aria-label={`Stop ${unit.name}`} onClick={() => requestAction(unit, 'stop')}>
                                        <CircleStop className="h-3.5 w-3.5" aria-hidden="true" />
                                      </Button>
                                    </Tooltip>
                                    <Tooltip label="Restart">
                                      <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Restart ${unit.name}`} onClick={() => requestAction(unit, 'restart')}>
                                        <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
                                      </Button>
                                    </Tooltip>
                                    <Tooltip label="Reload">
                                      <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Reload ${unit.name}`} onClick={() => requestAction(unit, 'reload')}>
                                        <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
                                      </Button>
                                    </Tooltip>
                                  </>
                                )}
                                {unit.file_state === 'enabled' ? (
                                  <Tooltip label="Disable at boot">
                                    <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Disable ${unit.name}`} onClick={() => requestAction(unit, 'disable')}>
                                      <PowerOff className="h-3.5 w-3.5" aria-hidden="true" />
                                    </Button>
                                  </Tooltip>
                                ) : (
                                  <Tooltip label="Enable at boot">
                                    <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Enable ${unit.name}`} onClick={() => requestAction(unit, 'enable')}>
                                      <Power className="h-3.5 w-3.5" aria-hidden="true" />
                                    </Button>
                                  </Tooltip>
                                )}
                              </div>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
          <p className="text-ink-500 text-[11px] leading-5">
            States are re-read after each action. Static, masked or unreadable units may reject an
            action; the remote error is shown without retrying as another user.
          </p>
        </>
      ) : null}

      <ServiceActionDialog
        open={target !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTarget(null)
            setActionError(null)
          }
        }}
        unit={target?.unit ?? null}
        action={target?.action ?? 'restart'}
        busy={busy}
        error={actionError}
        onConfirm={() => void handleAction()}
      />
    </div>
  )
}
