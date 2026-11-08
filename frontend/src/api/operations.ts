import { apiRequest } from './client'

export interface ProcessInfo {
  pid: number
  ppid: number
  user: string
  state: string
  cpu_percent: number
  memory_percent: number
  rss_bytes: number
  elapsed_seconds: number
  started_at: string
  command: string
}

export interface ProcessList {
  collected_at: string
  account: string
  processes: ProcessInfo[]
  truncated: boolean
}

export type ProcessSignal = 'SIGTERM' | 'SIGKILL'

export interface SignalResult {
  pid: number
  signal: ProcessSignal
  delivered: boolean
  running: boolean
  state?: string
}

export interface ServiceUnit {
  name: string
  load: string
  active: string
  sub: string
  file_state: string
  description: string
}

export interface ServiceList {
  collected_at: string
  units: ServiceUnit[]
  truncated: boolean
}

export type ServiceAction = 'start' | 'stop' | 'restart' | 'reload' | 'enable' | 'disable'

export interface ServiceActionResult {
  unit: string
  action: ServiceAction
  active_state: string
  sub_state: string
  unit_file_state: string
  state_known: boolean
  message?: string
}

export interface LogEntry {
  timestamp: string
  priority: number
  severity: string
  message: string
  unit?: string
  identifier?: string
}

export interface LogResult {
  collected_at: string
  unit?: string
  since: string
  lines: number
  entries: LogEntry[]
  truncated: boolean
}

export function listProcesses(serverId: string, signal?: AbortSignal): Promise<ProcessList> {
  return apiRequest<ProcessList>(`/api/v1/servers/${serverId}/processes`, { signal })
}

export function signalProcess(
  serverId: string,
  pid: number,
  signal: ProcessSignal,
  expectedStart: string,
  csrf: string,
): Promise<SignalResult> {
  return apiRequest<SignalResult>(`/api/v1/servers/${serverId}/processes/${pid}/signal`, {
    method: 'POST',
    csrf,
    body: { signal, expected_started_at: expectedStart },
  })
}

export function listServices(serverId: string, signal?: AbortSignal): Promise<ServiceList> {
  return apiRequest<ServiceList>(`/api/v1/servers/${serverId}/services`, { signal })
}

export function runServiceAction(
  serverId: string,
  unit: string,
  action: ServiceAction,
  csrf: string,
): Promise<ServiceActionResult> {
  return apiRequest<ServiceActionResult>(
    `/api/v1/servers/${serverId}/services/${encodeURIComponent(unit)}/action`,
    { method: 'POST', csrf, body: { action } },
  )
}

export function readLogs(
  serverId: string,
  options: { unit?: string; since: string; lines: number },
  signal?: AbortSignal,
): Promise<LogResult> {
  const query = new URLSearchParams()
  if (options.unit) query.set('unit', options.unit)
  if (options.since) query.set('since', options.since)
  query.set('lines', String(options.lines))
  return apiRequest<LogResult>(`/api/v1/servers/${serverId}/logs?${query.toString()}`, { signal })
}
