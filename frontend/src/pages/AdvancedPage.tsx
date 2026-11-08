import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Boxes,
  CalendarClock,
  HardDrive,
  Network,
  PackageSearch,
  Play,
  Plus,
  RefreshCw,
  Server as ServerIcon,
  Square,
  Trash2,
  RotateCw,
} from 'lucide-react'
import {
  dockerAction,
  dockerLogs,
  getDocker,
  getNetwork,
  getPackages,
  getSchedules,
  createTask,
  deleteTask,
  packageAction,
  type PackageAction,
  getStorage,
  type DockerAction,
  type DockerContainer,
} from '@/api/advanced'
import { getVersion } from '@/api/client'
import { listServers } from '@/api/servers'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ServerPicker } from '@/components/ServerPicker'
import { HostOpsPanel } from '@/components/advanced/HostOpsPanel'
import { ConfirmDialog } from '@/components/servers/ConfirmDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { StatusDot } from '@/components/ui/StatusDot'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatBytes } from '@/lib/files'
import { formatDateTime } from '@/lib/format'

type Tab = 'docker' | 'storage' | 'network' | 'packages' | 'schedules'

const tabs: Array<{ id: Tab; label: string; icon: typeof Boxes }> = [
  { id: 'docker', label: 'Docker', icon: Boxes },
  { id: 'storage', label: 'Storage', icon: HardDrive },
  { id: 'network', label: 'Network', icon: Network },
  { id: 'packages', label: 'Packages', icon: PackageSearch },
  { id: 'schedules', label: 'Schedules', icon: CalendarClock },
]

export function AdvancedPage() {
  const { serverId } = useParams<{ serverId?: string }>()
  const navigate = useNavigate()
  const { csrfToken, user } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>('docker')
  const [dockerTarget, setDockerTarget] = useState<{ container: DockerContainer; action: DockerAction } | null>(null)
  const [logsTarget, setLogsTarget] = useState<DockerContainer | null>(null)
  const [busy, setBusy] = useState(false)
  const [packageFilter, setPackageFilter] = useState('')
  const [packageTarget, setPackageTarget] = useState<{ name: string; action: PackageAction } | null>(null)
  const [packageName, setPackageName] = useState('')
  const [packageConfirm, setPackageConfirm] = useState(false)
  const [packageBusy, setPackageBusy] = useState(false)
  const [packageOutput, setPackageOutput] = useState('')
  const [packageError, setPackageError] = useState<string | null>(null)
  const [taskOpen, setTaskOpen] = useState(false)
  const [taskSchedule, setTaskSchedule] = useState('*/5 * * * *')
  const [taskCommand, setTaskCommand] = useState('')
  const [taskConfirm, setTaskConfirm] = useState(false)
  const [taskBusy, setTaskBusy] = useState(false)
  const [taskError, setTaskError] = useState<string | null>(null)
  const [taskDelete, setTaskDelete] = useState<{ id: string; command: string } | null>(null)

  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const versionQuery = useQuery({
    queryKey: ['version'],
    queryFn: ({ signal }) => getVersion(signal),
    staleTime: 60_000,
  })
  const servers = serversQuery.data?.servers ?? []
  const currentServer = servers.find((server) => server.id === serverId)
  const canWrite = user?.role !== 'read_only'

  const dockerQuery = useQuery({
    queryKey: ['docker', serverId],
    queryFn: ({ signal }) => getDocker(serverId ?? '', signal),
    enabled: Boolean(serverId) && tab === 'docker',
    retry: 0,
    refetchInterval: 15000,
  })
  const storageQuery = useQuery({
    queryKey: ['storage-tools', serverId],
    queryFn: ({ signal }) => getStorage(serverId ?? '', signal),
    enabled: Boolean(serverId) && tab === 'storage',
    retry: 0,
  })
  const networkQuery = useQuery({
    queryKey: ['network-tools', serverId],
    queryFn: ({ signal }) => getNetwork(serverId ?? '', signal),
    enabled: Boolean(serverId) && tab === 'network',
    retry: 0,
  })
  const packagesQuery = useQuery({
    queryKey: ['packages', serverId],
    queryFn: ({ signal }) => getPackages(serverId ?? '', signal),
    enabled: Boolean(serverId) && tab === 'packages',
    retry: 0,
  })
  const schedulesQuery = useQuery({
    queryKey: ['schedules', serverId],
    queryFn: ({ signal }) => getSchedules(serverId ?? '', signal),
    enabled: Boolean(serverId) && tab === 'schedules',
    retry: 0,
  })
  const logsQuery = useQuery({
    queryKey: ['docker-logs', serverId, logsTarget?.id, 200],
    queryFn: ({ signal }) => dockerLogs(serverId ?? '', logsTarget?.id ?? '', 200, signal),
    enabled: Boolean(serverId && logsTarget),
    retry: 0,
  })

  const activeQuery = {
    docker: dockerQuery,
    storage: storageQuery,
    network: networkQuery,
    packages: packagesQuery,
    schedules: schedulesQuery,
  }[tab]

  const refreshActive = () => {
    void activeQuery.refetch()
  }

  const handleDockerAction = async () => {
    if (!serverId || !dockerTarget) return
    setBusy(true)
    try {
      await dockerAction(serverId, dockerTarget.container.id, dockerTarget.action, csrfToken)
      toast({
        title: `Container ${dockerTarget.action} requested`,
        description: dockerTarget.container.name,
        variant: 'success',
      })
      setDockerTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['docker', serverId] })
    } catch (caught) {
      toast({ title: 'Docker action failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setBusy(false)
    }
  }

  const openPackageDialog = (name: string, action: PackageAction) => {
    setPackageTarget({ name, action })
    setPackageName(name)
    setPackageConfirm(false)
    setPackageOutput('')
    setPackageError(null)
  }

  const runPackageAction = async () => {
    if (!serverId || !packageTarget || !packageConfirm) return
    const name = packageName.trim()
    if (!name) {
      setPackageError('Enter a package name.')
      return
    }
    setPackageBusy(true)
    setPackageError(null)
    setPackageOutput('')
    try {
      const result = await packageAction(serverId, name, packageTarget.action, csrfToken)
      setPackageOutput(result.output || '(no output)')
      if (!result.success) {
        setPackageError(`The package manager reported a failure. Review the output below.`)
      } else {
        toast({ title: `Package ${packageTarget.action} completed`, description: name, variant: 'success' })
        await queryClient.invalidateQueries({ queryKey: ['packages', serverId] })
      }
    } catch (caught) {
      setPackageError(userFacingError(caught))
    } finally {
      setPackageBusy(false)
    }
  }

  const runCreateTask = async () => {
    if (!serverId || !taskConfirm) return
    setTaskBusy(true)
    setTaskError(null)
    try {
      await createTask(serverId, { schedule: taskSchedule.trim(), command: taskCommand.trim() }, csrfToken)
      toast({ title: 'Scheduled task created', description: taskSchedule.trim(), variant: 'success' })
      setTaskOpen(false)
      setTaskConfirm(false)
      setTaskCommand('')
      await queryClient.invalidateQueries({ queryKey: ['schedules', serverId] })
    } catch (caught) {
      setTaskError(userFacingError(caught))
    } finally {
      setTaskBusy(false)
    }
  }

  const runDeleteTask = async () => {
    if (!serverId || !taskDelete) return
    setTaskBusy(true)
    try {
      await deleteTask(serverId, taskDelete.id, csrfToken)
      toast({ title: 'Scheduled task deleted', description: taskDelete.command, variant: 'success' })
      setTaskDelete(null)
      await queryClient.invalidateQueries({ queryKey: ['schedules', serverId] })
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setTaskBusy(false)
    }
  }

  const packages = useMemo(() => {
    const needle = packageFilter.trim().toLowerCase()
    return (packagesQuery.data?.packages ?? [])
      .filter((pkg) => !needle || pkg.name.toLowerCase().includes(needle))
      .slice(0, 500)
  }, [packagesQuery.data, packageFilter])

  if (serversQuery.isLoading) return <Skeleton className="h-[60vh] w-full" />
  if (serversQuery.isError) {
    return <Alert variant="error" title="Servers could not be loaded">{userFacingError(serversQuery.error)}</Alert>
  }

  if (!serverId) {
    return (
      <div className="space-y-5">
        <PageHeader
          eyebrow="Operate"
          title="Advanced management"
          description="Read-first Docker, storage, network, package and scheduled-task inspection with explicit actions where implemented."
        />
        {servers.length === 0 ? (
          <Card className="p-8 text-center">
            <ServerIcon className="text-signal-300 mx-auto h-8 w-8" aria-hidden="true" />
            <h2 className="font-display text-ink-50 mt-4 text-xl font-semibold">Add a server first</h2>
            <Button asChild className="mt-5"><Link to="/servers">Go to servers</Link></Button>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {servers.map((server) => (
              <Card key={server.id}>
                <CardHeader>
                  <CardTitle>{server.display_name}</CardTitle>
                  <CardDescription>{server.username}@{server.host}:{server.port}</CardDescription>
                </CardHeader>
                <CardContent className="pt-4">
                  <Button variant="outline" className="w-full" onClick={() => navigate(`/advanced/${server.id}`)}>
                    Open advanced tools
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
    return <Alert variant="error" title="Server not found">This server does not exist or is not assigned to your account.</Alert>
  }

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Operate"
        title="Advanced management"
        description={`Read-first diagnostics on ${currentServer.host}. Destructive platform changes are intentionally not exposed.`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <ServerPicker servers={servers} serverId={serverId} onChange={(id) => navigate(`/advanced/${id}`)} />
            <Button variant="outline" size="sm" onClick={refreshActive} loading={activeQuery.isFetching}>
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
              Refresh
            </Button>
          </div>
        }
      />

      <div className="border-ink-700 flex flex-wrap gap-1 rounded-lg border p-1">
        {tabs.map((item) => {
          const Icon = item.icon
          return (
            <Button
              key={item.id}
              size="sm"
              variant={tab === item.id ? 'primary' : 'ghost'}
              onClick={() => setTab(item.id)}
            >
              <Icon className="h-3.5 w-3.5" aria-hidden="true" />
              {item.label}
            </Button>
          )
        })}
      </div>

      {tab === 'docker' ? (
        <div className="space-y-4">
          <Alert variant="warning" title="Docker access is root-equivalent on many hosts">
            The connected SSH account can control the Docker daemon. Drovik never mounts a Docker
            socket into its own container and performs only enumerated container actions. Treat this
            account as privileged.
          </Alert>
          {dockerQuery.isError ? (
            <Alert variant="error" title="Docker is unavailable or inaccessible">{userFacingError(dockerQuery.error)}</Alert>
          ) : dockerQuery.isLoading ? (
            <Skeleton className="h-48 w-full" />
          ) : dockerQuery.data ? (
            <>
              <Card>
                <CardHeader className="flex-row items-center justify-between">
                  <div>
                    <CardTitle>Containers</CardTitle>
                    <CardDescription>{dockerQuery.data.containers.length} containers · {formatDateTime(dockerQuery.data.collected_at)}</CardDescription>
                  </div>
                  {dockerQuery.data.truncated ? <Badge variant="signal">truncated</Badge> : null}
                </CardHeader>
                <CardContent className="px-0 pt-2">
                  {dockerQuery.data.containers.length === 0 ? (
                    <p className="text-ink-500 px-5 py-6 text-xs">No containers reported.</p>
                  ) : (
                    <div className="overflow-x-auto">
                      <table className="w-full text-left text-sm">
                        <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase">
                          <tr>
                            <th className="px-4 py-2 font-medium">Container</th>
                            <th className="px-4 py-2 font-medium">Image</th>
                            <th className="px-4 py-2 font-medium">State</th>
                            <th className="hidden px-4 py-2 font-medium md:table-cell">Ports</th>
                            <th className="px-4 py-2 text-right font-medium">Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {dockerQuery.data.containers.map((container) => (
                            <tr key={container.id} className="border-ink-800 border-b last:border-0">
                              <td className="px-4 py-3">
                                <p className="text-ink-100 text-xs font-medium">{container.name || container.id.slice(0, 12)}</p>
                                <p className="text-ink-600 font-mono text-[9px]">{container.id.slice(0, 12)}</p>
                              </td>
                              <td className="text-ink-400 max-w-56 truncate px-4 py-3 font-mono text-[10px]">{container.image}</td>
                              <td className="px-4 py-3">
                                <StatusDot status={container.state === 'running' ? 'online' : container.state === 'exited' ? 'offline' : 'pending'} label={container.state} />
                                <p className="text-ink-600 mt-1 truncate text-[9px]">{container.status}</p>
                              </td>
                              <td className="text-ink-500 hidden max-w-48 truncate px-4 py-3 font-mono text-[10px] md:table-cell">{container.ports || '—'}</td>
                              <td className="px-4 py-3">
                                <div className="flex justify-end gap-1">
                                  {canWrite ? (
                                    <>
                                      <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Start ${container.name}`} onClick={() => setDockerTarget({ container, action: 'start' })}><Play className="h-3.5 w-3.5" aria-hidden="true" /></Button>
                                      <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Stop ${container.name}`} onClick={() => setDockerTarget({ container, action: 'stop' })}><Square className="h-3.5 w-3.5" aria-hidden="true" /></Button>
                                      <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={`Restart ${container.name}`} onClick={() => setDockerTarget({ container, action: 'restart' })}><RotateCw className="h-3.5 w-3.5" aria-hidden="true" /></Button>
                                      <Button size="icon" variant="ghost" className="hover:text-rose-400 h-7 w-7" aria-label={`Remove ${container.name}`} onClick={() => setDockerTarget({ container, action: 'remove' })}><Trash2 className="h-3.5 w-3.5" aria-hidden="true" /></Button>
                                    </>
                                  ) : null}
                                  <Button size="sm" variant="ghost" onClick={() => setLogsTarget(container)}>Logs</Button>
                                </div>
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
                <CardHeader><CardTitle>Images</CardTitle><CardDescription>Local image cache.</CardDescription></CardHeader>
                <CardContent className="px-0 pt-2">
                  <div className="overflow-x-auto">
                    <table className="w-full text-left text-sm">
                      <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase">
                        <tr><th className="px-4 py-2 font-medium">Repository</th><th className="px-4 py-2 font-medium">Tag</th><th className="px-4 py-2 font-medium">Size</th><th className="px-4 py-2 font-medium">Created</th></tr>
                      </thead>
                      <tbody>
                        {dockerQuery.data.images.slice(0, 200).map((image) => (
                          <tr key={`${image.id}-${image.tag}`} className="border-ink-800 border-b last:border-0">
                            <td className="text-ink-200 px-4 py-2 font-mono text-[10px]">{image.repository}</td>
                            <td className="text-ink-400 px-4 py-2 font-mono text-[10px]">{image.tag}</td>
                            <td className="text-ink-500 px-4 py-2 font-mono text-[10px]">{image.size}</td>
                            <td className="text-ink-500 px-4 py-2 text-[10px]">{image.created}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              </Card>
            </>
          ) : null}
        </div>
      ) : null}

      {tab === 'storage' ? (
        storageQuery.isError ? <Alert variant="error" title="Storage tools unavailable">{userFacingError(storageQuery.error)}</Alert> :
        storageQuery.isLoading ? <Skeleton className="h-64 w-full" /> :
        storageQuery.data ? (
          <div className="space-y-4">
            <HostOpsPanel serverId={serverId} kind="storage" enabled={Boolean(versionQuery.data?.destructive_host_ops) && canWrite} />
            <Card>
              <CardHeader><CardTitle>Block devices</CardTitle><CardDescription>Read-only lsblk inspection. No mount, format or partition actions are exposed.</CardDescription></CardHeader>
              <CardContent className="px-0 pt-2">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase">
                    <tr><th className="px-4 py-2 font-medium">Device</th><th className="px-4 py-2 font-medium">Type</th><th className="px-4 py-2 font-medium">Size</th><th className="hidden px-4 py-2 font-medium md:table-cell">Filesystem</th><th className="px-4 py-2 font-medium">Mount</th></tr>
                  </thead>
                  <tbody>
                    {storageQuery.data.block_devices.flatMap((device) => [device, ...(device.children ?? [])]).map((device) => (
                      <tr key={`${device.name}-${device.mountpoint}`} className="border-ink-800 border-b last:border-0">
                        <td className="text-ink-100 px-4 py-2 font-mono text-[10px]">{device.name}</td>
                        <td className="text-ink-400 px-4 py-2 text-[10px]">{device.type}</td>
                        <td className="text-ink-400 px-4 py-2 font-mono text-[10px]">{formatBytes(device.size_bytes)}</td>
                        <td className="text-ink-500 hidden px-4 py-2 font-mono text-[10px] md:table-cell">{device.fstype || '—'}</td>
                        <td className="text-ink-300 px-4 py-2 font-mono text-[10px]">{device.mountpoint || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>Filesystem and inode usage</CardTitle><CardDescription>Capacity and inode pressure from df.</CardDescription></CardHeader>
              <CardContent className="px-0 pt-2">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase">
                    <tr><th className="px-4 py-2 font-medium">Mount</th><th className="px-4 py-2 font-medium">Device</th><th className="px-4 py-2 font-medium">Capacity</th><th className="px-4 py-2 font-medium">Inodes</th></tr>
                  </thead>
                  <tbody>
                    {storageQuery.data.filesystems.map((filesystem) => (
                      <tr key={filesystem.mountpoint} className="border-ink-800 border-b last:border-0">
                        <td className="text-ink-100 px-4 py-2 font-mono text-[10px]">{filesystem.mountpoint}{filesystem.read_only ? ' (ro)' : ''}</td>
                        <td className="text-ink-500 max-w-48 truncate px-4 py-2 font-mono text-[10px]">{filesystem.device}</td>
                        <td className="px-4 py-2">
                          <p className="text-ink-300 font-mono text-[10px]">{formatBytes(filesystem.used_bytes)} / {formatBytes(filesystem.total_bytes)}</p>
                          <div className="bg-ink-800 mt-1 h-1.5 w-32 overflow-hidden rounded-full"><div className="bg-signal-400 h-full" style={{ width: `${filesystem.total_bytes ? Math.min(100, (filesystem.used_bytes / filesystem.total_bytes) * 100) : 0}%` }} /></div>
                        </td>
                        <td className="px-4 py-2">
                          <p className="text-ink-300 font-mono text-[10px]">{filesystem.inodes_used.toLocaleString()} / {filesystem.inodes.toLocaleString()}</p>
                          <div className="bg-ink-800 mt-1 h-1.5 w-32 overflow-hidden rounded-full"><div className="bg-mint-400 h-full" style={{ width: `${Math.min(100, filesystem.inode_percent)}%` }} /></div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          </div>
        ) : null
      ) : null}

      {tab === 'network' ? (
        networkQuery.isError ? <Alert variant="error" title="Network tools unavailable">{userFacingError(networkQuery.error)}</Alert> :
        networkQuery.isLoading ? <Skeleton className="h-64 w-full" /> :
        networkQuery.data ? (
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {networkQuery.data.interfaces.map((iface) => (
                <Card key={iface.name} className="p-4">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-ink-100 font-mono text-xs">{iface.name}</p>
                    <StatusDot status={iface.state === 'UP' ? 'online' : 'offline'} label={iface.state} />
                  </div>
                  <p className="text-ink-500 mt-2 font-mono text-[10px]">MTU {iface.mtu} {iface.mac ? `· ${iface.mac}` : ''}</p>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {iface.addresses.length === 0 ? <Badge variant="outline">no address</Badge> : iface.addresses.map((address) => <Badge key={address} variant="neutral">{address}</Badge>)}
                  </div>
                </Card>
              ))}
            </div>
            <HostOpsPanel serverId={serverId} kind="network" enabled={Boolean(versionQuery.data?.destructive_host_ops) && canWrite} />
            <Card>
              <CardHeader><CardTitle>Routes</CardTitle><CardDescription>Read-only routing table.</CardDescription></CardHeader>
              <CardContent className="px-0 pt-2">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase"><tr><th className="px-4 py-2 font-medium">Destination</th><th className="px-4 py-2 font-medium">Gateway</th><th className="px-4 py-2 font-medium">Device</th><th className="px-4 py-2 font-medium">Metric</th></tr></thead>
                  <tbody>{networkQuery.data.routes.map((route) => (<tr key={`${route.destination}-${route.gateway}-${route.metric}`} className="border-ink-800 border-b last:border-0"><td className="text-ink-200 px-4 py-2 font-mono text-[10px]">{route.destination}</td><td className="text-ink-400 px-4 py-2 font-mono text-[10px]">{route.gateway || '—'}</td><td className="text-ink-400 px-4 py-2 font-mono text-[10px]">{route.device}</td><td className="text-ink-500 px-4 py-2 font-mono text-[10px]">{route.metric || '—'}</td></tr>))}</tbody>
                </table>
              </CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>Listening sockets</CardTitle><CardDescription>ss read-only output. Firewall and interface changes are not exposed.</CardDescription></CardHeader>
              <CardContent className="px-0 pt-2">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase"><tr><th className="px-4 py-2 font-medium">Proto</th><th className="px-4 py-2 font-medium">Local</th><th className="px-4 py-2 font-medium">Peer</th><th className="hidden px-4 py-2 font-medium md:table-cell">Process</th></tr></thead>
                  <tbody>{networkQuery.data.listeners.map((listener, index) => (<tr key={`${listener.local}-${index}`} className="border-ink-800 border-b last:border-0"><td className="text-ink-400 px-4 py-2 font-mono text-[10px]">{listener.protocol}</td><td className="text-ink-200 px-4 py-2 font-mono text-[10px]">{listener.local}</td><td className="text-ink-500 px-4 py-2 font-mono text-[10px]">{listener.peer}</td><td className="text-ink-500 hidden max-w-64 truncate px-4 py-2 font-mono text-[10px] md:table-cell">{listener.process || '—'}</td></tr>))}</tbody>
                </table>
              </CardContent>
            </Card>
          </div>
        ) : null
      ) : null}

      {tab === 'packages' ? (
        packagesQuery.isError ? <Alert variant="error" title="Package tools unavailable">{userFacingError(packagesQuery.error)}</Alert> :
        packagesQuery.isLoading ? <Skeleton className="h-64 w-full" /> :
        packagesQuery.data ? (
          <Card>
            <CardHeader className="flex-row items-start justify-between gap-3">
              <div><CardTitle>Installed packages</CardTitle><CardDescription>Enumerated install, upgrade and remove actions run as the connected SSH account. No privilege escalation is attempted.</CardDescription></div>
              <div className="flex items-center gap-2">
                <Badge variant="outline">{packagesQuery.data.packages.length} installed</Badge>
                {canWrite ? <Button size="sm" variant="outline" onClick={() => openPackageDialog('', 'install')}><Plus className="h-3.5 w-3.5" aria-hidden="true" />Install package</Button> : null}
              </div>
            </CardHeader>
            <CardContent className="space-y-3 pt-4">
              <Input value={packageFilter} onChange={(event) => setPackageFilter(event.target.value)} placeholder="Filter installed packages" className="h-9 max-w-md text-xs" aria-label="Filter packages" />
              <div className="max-h-[60vh] overflow-y-auto">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500 bg-ink-900 border-ink-700 sticky top-0 border-y font-mono text-[10px] uppercase"><tr><th className="px-4 py-2 font-medium">Package</th><th className="px-4 py-2 font-medium">Version</th><th className="px-4 py-2 text-right font-medium">Actions</th></tr></thead>
                  <tbody>{packages.map((pkg) => (
                    <tr key={pkg.name} className="border-ink-800 border-b last:border-0">
                      <td className="text-ink-200 px-4 py-2 font-mono text-[10px]">{pkg.name}</td>
                      <td className="text-ink-500 px-4 py-2 font-mono text-[10px]">{pkg.version}</td>
                      <td className="px-4 py-2 text-right">
                        {canWrite ? (
                          <div className="flex justify-end gap-1">
                            <Button size="sm" variant="ghost" className="h-7 text-[10px]" onClick={() => openPackageDialog(pkg.name, 'upgrade')}>Upgrade</Button>
                            <Button size="sm" variant="ghost" className="hover:text-rose-400 h-7 text-[10px]" onClick={() => openPackageDialog(pkg.name, 'remove')}>Remove</Button>
                          </div>
                        ) : null}
                      </td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
              <p className="text-ink-500 text-[10px]">Showing at most 500 filtered packages. Update availability is not queried to avoid contacting package repositories; manager commands use the host's configured repositories.</p>
            </CardContent>
          </Card>
        ) : null
      ) : null}

      {tab === 'schedules' ? (
        schedulesQuery.isError ? <Alert variant="error" title="Scheduled tasks unavailable">{userFacingError(schedulesQuery.error)}</Alert> :
        schedulesQuery.isLoading ? <Skeleton className="h-64 w-full" /> :
        schedulesQuery.data ? (
          <div className="space-y-4">
            <Card>
              <CardHeader className="flex-row items-start justify-between gap-3">
                <div>
                  <CardTitle>Drovik-managed tasks</CardTitle>
                  <CardDescription>Entries created here are marked in the SSH user's crontab and can be removed safely.</CardDescription>
                </div>
                {canWrite ? <Button size="sm" variant="outline" onClick={() => { setTaskError(null); setTaskOpen(true) }}><Plus className="h-3.5 w-3.5" aria-hidden="true" />Add task</Button> : null}
              </CardHeader>
              <CardContent className="px-0 pt-2">
                {schedulesQuery.data.managed_tasks.length === 0 ? (
                  <p className="text-ink-500 px-5 py-4 text-xs">No Drovik-managed tasks.</p>
                ) : (
                  <table className="w-full text-left text-sm">
                    <thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase"><tr><th className="px-4 py-2 font-medium">Schedule</th><th className="px-4 py-2 font-medium">Command</th><th className="px-4 py-2 text-right font-medium">Action</th></tr></thead>
                    <tbody>{schedulesQuery.data.managed_tasks.map((task) => (
                      <tr key={task.id} className="border-ink-800 border-b last:border-0">
                        <td className="text-ink-200 px-4 py-2 font-mono text-[10px]">{task.schedule}</td>
                        <td className="text-ink-300 px-4 py-2 font-mono text-[10px] break-all">{task.command}</td>
                        <td className="px-4 py-2 text-right">{canWrite ? <Button size="sm" variant="ghost" className="hover:text-rose-400 h-7 text-[10px]" onClick={() => setTaskDelete({ id: task.id, command: task.command })}>Remove</Button> : null}</td>
                      </tr>
                    ))}</tbody>
                  </table>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>Cron entries</CardTitle><CardDescription>Read-only view of user and system cron. Editing is intentionally not exposed yet.</CardDescription></CardHeader>
              <CardContent className="px-0 pt-2">
                {schedulesQuery.data.cron.length === 0 ? <p className="text-ink-500 px-5 py-4 text-xs">No cron entries visible to this account.</p> : (
                  <table className="w-full text-left text-sm"><thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase"><tr><th className="px-4 py-2 font-medium">Source</th><th className="px-4 py-2 font-medium">Line</th></tr></thead><tbody>{schedulesQuery.data.cron.map((entry, index) => (<tr key={`${entry.source}-${index}`} className="border-ink-800 border-b last:border-0"><td className="text-ink-500 px-4 py-2 font-mono text-[10px]">{entry.source}</td><td className="text-ink-200 px-4 py-2 font-mono text-[10px] break-all">{entry.line}</td></tr>))}</tbody></table>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>systemd timers</CardTitle><CardDescription>Read-only timer state.</CardDescription></CardHeader>
              <CardContent className="px-0 pt-2">
                {schedulesQuery.data.timers.length === 0 ? <p className="text-ink-500 px-5 py-4 text-xs">No systemd timers reported.</p> : (
                  <table className="w-full text-left text-sm"><thead className="text-ink-500 border-ink-700 border-y font-mono text-[10px] uppercase"><tr><th className="px-4 py-2 font-medium">Unit</th><th className="px-4 py-2 font-medium">Next</th><th className="px-4 py-2 font-medium">Left</th><th className="hidden px-4 py-2 font-medium md:table-cell">Activates</th></tr></thead><tbody>{schedulesQuery.data.timers.map((timer) => (<tr key={timer.unit} className="border-ink-800 border-b last:border-0"><td className="text-ink-200 px-4 py-2 font-mono text-[10px]">{timer.unit}</td><td className="text-ink-400 px-4 py-2 text-[10px]">{timer.next}</td><td className="text-ink-500 px-4 py-2 text-[10px]">{timer.left}</td><td className="text-ink-500 hidden px-4 py-2 font-mono text-[10px] md:table-cell">{timer.activates}</td></tr>))}</tbody></table>
                )}
              </CardContent>
            </Card>
          </div>
        ) : null
      ) : null}

      <Dialog
        open={taskOpen}
        onOpenChange={setTaskOpen}
        title="Add scheduled task"
        description="Creates one five-field cron entry in the SSH user's crontab."
        size="md"
        footer={
          <>
            <Button variant="ghost" onClick={() => setTaskOpen(false)} disabled={taskBusy}>Cancel</Button>
            <Button onClick={() => void runCreateTask()} loading={taskBusy} disabled={!taskConfirm || !taskCommand.trim() || !taskSchedule.trim()}>Create task</Button>
          </>
        }
      >
        <div className="space-y-4">
          {taskError ? <Alert variant="error">{taskError}</Alert> : null}
          <Alert variant="warning" title="Cron changes run on the managed host">
            The task runs as the connected SSH account on its schedule. Drovik writes only its own
            marked block and preserves the existing crontab.
          </Alert>
          <Field label="Schedule" htmlFor="task-schedule" hint="Five cron fields: minute hour day month weekday.">
            <Input id="task-schedule" value={taskSchedule} onChange={(event) => setTaskSchedule(event.target.value)} className="font-mono" autoComplete="off" />
          </Field>
          <Field label="Command" htmlFor="task-command" hint="No newlines or percent signs; maximum 500 bytes.">
            <Input id="task-command" value={taskCommand} onChange={(event) => setTaskCommand(event.target.value)} className="font-mono" autoComplete="off" placeholder="/usr/local/bin/backup" />
          </Field>
          <label className="text-ink-300 flex items-start gap-2 text-xs">
            <input type="checkbox" checked={taskConfirm} onChange={(event) => setTaskConfirm(event.target.checked)} className="accent-signal-400 mt-0.5 h-4 w-4" />
            I confirm this recurring command should run on the selected server.
          </label>
        </div>
      </Dialog>

      <ConfirmDialog
        open={taskDelete !== null}
        onOpenChange={(open) => { if (!open) setTaskDelete(null) }}
        title="Delete scheduled task?"
        description={taskDelete ? `Remove the Drovik-managed cron entry: ${taskDelete.command}` : ''}
        confirmLabel="Delete task"
        destructive
        busy={taskBusy}
        onConfirm={() => void runDeleteTask()}
      />

      <Dialog
        open={packageTarget !== null}
        onOpenChange={(open) => { if (!open) setPackageTarget(null) }}
        title={packageTarget ? `${packageTarget.action} package` : 'Package action'}
        description="Runs one enumerated package-manager command as the connected SSH account."
        size="md"
        footer={
          <>
            <Button variant="ghost" onClick={() => setPackageTarget(null)} disabled={packageBusy}>Close</Button>
            <Button
              onClick={() => void runPackageAction()}
              loading={packageBusy}
              disabled={!packageConfirm || !packageName.trim()}
            >
              {packageTarget?.action === 'remove' ? 'Remove package' : packageTarget?.action === 'upgrade' ? 'Upgrade package' : 'Install package'}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {packageError ? <Alert variant="error">{packageError}</Alert> : null}
          <Alert variant={packageTarget?.action === 'remove' ? 'warning' : 'info'} title="Package changes affect the managed host">
            Installing, upgrading or removing packages changes software on the server and can restart
            or break services. Drovik does not use sudo; the remote account's permissions decide the outcome.
          </Alert>
          <Field label="Package name" htmlFor="package-name">
            <Input
              id="package-name"
              value={packageName}
              onChange={(event) => setPackageName(event.target.value)}
              disabled={packageTarget?.action !== 'install'}
              placeholder="nginx"
              autoComplete="off"
            />
          </Field>
          <label className="text-ink-300 flex items-start gap-2 text-xs">
            <input type="checkbox" checked={packageConfirm} onChange={(event) => setPackageConfirm(event.target.checked)} className="accent-signal-400 mt-0.5 h-4 w-4" />
            I confirm the {packageTarget?.action ?? 'package'} action for {packageName || 'this package'} on this server.
          </label>
          {packageOutput ? (
            <pre className="bg-ink-950 text-ink-200 max-h-64 overflow-auto rounded-md p-3 font-mono text-[10px] whitespace-pre-wrap">{packageOutput}</pre>
          ) : null}
        </div>
      </Dialog>

      <ConfirmDialog
        open={dockerTarget !== null}
        onOpenChange={(open) => { if (!open) setDockerTarget(null) }}
        title={`${dockerTarget?.action ?? 'Docker'} container?`}
        description={`${dockerTarget?.container.name ?? 'Container'} · this runs as the connected SSH account and may be root-equivalent.`}
        confirmLabel={dockerTarget?.action ?? 'Confirm'}
        destructive={dockerTarget?.action === 'remove' || dockerTarget?.action === 'stop'}
        busy={busy}
        onConfirm={() => void handleDockerAction()}
      />

      <Dialog
        open={logsTarget !== null}
        onOpenChange={(open) => { if (!open) setLogsTarget(null) }}
        title="Container logs"
        description={logsTarget?.name}
        size="lg"
        footer={<Button variant="ghost" onClick={() => setLogsTarget(null)}>Close</Button>}
      >
        {logsQuery.isError ? <Alert variant="error">{userFacingError(logsQuery.error)}</Alert> : logsQuery.isLoading ? <Skeleton className="h-64 w-full" /> : (
          <pre className="bg-ink-950 text-ink-200 max-h-[55vh] overflow-auto rounded-md p-3 font-mono text-[11px] leading-5 whitespace-pre-wrap">{logsQuery.data?.output || 'No log output.'}</pre>
        )}
      </Dialog>
    </div>
  )
}
