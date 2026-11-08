import { apiRequest } from './client'

export interface HostInfo {
  hostname: string
  os: string
  distribution: string
  kernel: string
  architecture: string
}

export interface CoreUsage {
  index: number
  usage_percent: number | null
}

export interface CPUMetrics {
  available: boolean
  usage_percent: number | null
  model: string
  cores: CoreUsage[]
}

export interface LoadAverage {
  available: boolean
  one: number
  five: number
  fifteen: number
  runnable: number
  total: number
}

export interface MemoryMetrics {
  available: boolean
  total_bytes: number
  available_bytes: number
  used_bytes: number
  free_bytes: number
  buffers_bytes: number
  cached_bytes: number
  swap_total_bytes: number
  swap_used_bytes: number
  used_percent: number | null
}

export interface UptimeMetrics {
  available: boolean
  seconds: number
  boot_time: string
}

export interface Filesystem {
  device: string
  mountpoint: string
  fstype: string
  total_bytes: number
  used_bytes: number
  free_bytes: number
  used_percent: number | null
  read_only: boolean
}

export interface NetworkInterface {
  name: string
  received_bytes: number
  transmitted_bytes: number
  receive_rate_bps: number | null
  transmit_rate_bps: number | null
}

export interface OverviewSnapshot {
  server_id: string
  collected_at: string
  cached: boolean
  stale: boolean
  warning?: string
  host: HostInfo
  cpu: CPUMetrics
  load: LoadAverage
  memory: MemoryMetrics
  uptime: UptimeMetrics
  filesystems: Filesystem[]
  filesystems_available: boolean
  network: NetworkInterface[]
  network_available: boolean
  filesystems_truncated: boolean
  network_truncated: boolean
}

export function getServerOverview(
  serverId: string,
  signal?: AbortSignal,
): Promise<OverviewSnapshot> {
  return apiRequest<OverviewSnapshot>(`/api/v1/servers/${serverId}/overview`, { signal })
}
