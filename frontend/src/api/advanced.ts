import { apiRequest } from './client'

export interface DockerContainer {
  id: string
  name: string
  image: string
  state: string
  status: string
  ports: string
  created_at: string
  running_for: string
}

export interface DockerImage {
  id: string
  repository: string
  tag: string
  size: string
  created: string
}

export interface DockerOverview {
  available: boolean
  containers: DockerContainer[]
  images: DockerImage[]
  truncated: boolean
  collected_at: string
}

export type DockerAction = 'start' | 'stop' | 'restart' | 'remove'

export function getDocker(serverId: string, signal?: AbortSignal): Promise<DockerOverview> {
  return apiRequest<DockerOverview>(`/api/v1/servers/${serverId}/docker`, { signal })
}

export function dockerAction(
  serverId: string,
  containerId: string,
  action: DockerAction,
  csrf: string,
): Promise<void> {
  return apiRequest<void>(
    `/api/v1/servers/${serverId}/docker/${encodeURIComponent(containerId)}/action`,
    { method: 'POST', csrf, body: { action } },
  )
}

export function dockerLogs(
  serverId: string,
  containerId: string,
  tail: number,
  signal?: AbortSignal,
): Promise<{ container: string; output: string; truncated: boolean }> {
  return apiRequest(
    `/api/v1/servers/${serverId}/docker/${encodeURIComponent(containerId)}/logs?tail=${tail}`,
    { signal },
  )
}

export function dockerInspect(
  serverId: string,
  containerId: string,
  signal?: AbortSignal,
): Promise<unknown> {
  return apiRequest(
    `/api/v1/servers/${serverId}/docker/${encodeURIComponent(containerId)}/inspect`,
    { signal },
  )
}

export interface BlockDevice {
  name: string
  type: string
  size_bytes: number
  fstype?: string
  mountpoint?: string
  read_only: boolean
  model?: string
  transport?: string
  children?: BlockDevice[]
}

export interface FilesystemUsage {
  device: string
  mountpoint: string
  fstype: string
  read_only: boolean
  total_bytes: number
  used_bytes: number
  free_bytes: number
  inodes: number
  inodes_used: number
  inodes_free: number
  inode_percent: number
}

export interface StorageOverview {
  available: boolean
  block_devices: BlockDevice[]
  filesystems: FilesystemUsage[]
  truncated: boolean
  collected_at: string
}

export function getStorage(serverId: string, signal?: AbortSignal): Promise<StorageOverview> {
  return apiRequest<StorageOverview>(`/api/v1/servers/${serverId}/storage-tools`, { signal })
}

export interface NetworkInterface {
  name: string
  state: string
  mtu: number
  mac?: string
  addresses: string[]
}

export interface NetworkRoute {
  destination: string
  gateway?: string
  device: string
  protocol?: string
  scope?: string
  metric?: number
}

export interface Listener {
  protocol: string
  local: string
  peer: string
  process?: string
}

export interface NetworkOverview {
  available: boolean
  interfaces: NetworkInterface[]
  routes: NetworkRoute[]
  listeners: Listener[]
  truncated: boolean
  collected_at: string
}

export function getNetwork(serverId: string, signal?: AbortSignal): Promise<NetworkOverview> {
  return apiRequest<NetworkOverview>(`/api/v1/servers/${serverId}/network-tools`, { signal })
}

export type PackageAction = 'install' | 'upgrade' | 'remove'

export interface PackageActionResult {
  manager: string
  package: string
  action: PackageAction
  output: string
  success: boolean
  truncated: boolean
}

export interface PackageInfo {
  name: string
  version: string
}

export interface PackageOverview {
  available: boolean
  manager: string
  packages: PackageInfo[]
  truncated: boolean
  update_check_available: boolean
  collected_at: string
}

export function getPackages(serverId: string, signal?: AbortSignal): Promise<PackageOverview> {
  return apiRequest<PackageOverview>(`/api/v1/servers/${serverId}/packages`, { signal })
}

export function packageAction(
  serverId: string,
  pkg: string,
  action: PackageAction,
  csrf: string,
): Promise<PackageActionResult> {
  return apiRequest<PackageActionResult>(`/api/v1/servers/${serverId}/packages/action`, {
    method: 'POST',
    csrf,
    body: { package: pkg, action, confirm: true },
  })
}

export interface CronEntry {
  source: string
  line: string
}

export interface TimerEntry {
  unit: string
  next: string
  left: string
  last: string
  passed: string
  activates: string
}

export interface ManagedTask {
  id: string
  schedule: string
  command: string
}

export interface ScheduleOverview {
  available: boolean
  cron: CronEntry[]
  timers: TimerEntry[]
  managed_tasks: ManagedTask[]
  collected_at: string
}

export function createTask(
  serverId: string,
  input: { schedule: string; command: string },
  csrf: string,
): Promise<{ task: ManagedTask }> {
  return apiRequest<{ task: ManagedTask }>(`/api/v1/servers/${serverId}/tasks`, {
    method: 'POST',
    csrf,
    body: { ...input, confirm: true },
  })
}

export function deleteTask(serverId: string, taskId: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/servers/${serverId}/tasks/${taskId}?confirm=true`, {
    method: 'DELETE',
    csrf,
  })
}

export function getSchedules(serverId: string, signal?: AbortSignal): Promise<ScheduleOverview> {
  return apiRequest<ScheduleOverview>(`/api/v1/servers/${serverId}/schedules`, { signal })
}

export interface SnippetVariable {
  name: string
  description?: string
  default?: string
}

export interface Snippet {
  id: string
  name: string
  description: string
  command: string
  variables: SnippetVariable[]
  created_at: string
  updated_at: string
}

export interface SnippetInput {
  name: string
  description: string
  command: string
  variables: SnippetVariable[]
}

export interface SnippetRun {
  snippet_id: string
  command: string
  output: string
  truncated: boolean
}

export function listSnippets(signal?: AbortSignal): Promise<{ snippets: Snippet[] }> {
  return apiRequest<{ snippets: Snippet[] }>('/api/v1/snippets', { signal })
}

export function createSnippet(input: SnippetInput, csrf: string): Promise<Snippet> {
  return apiRequest<Snippet>('/api/v1/snippets', { method: 'POST', body: input, csrf })
}

export function updateSnippet(id: string, input: SnippetInput, csrf: string): Promise<Snippet> {
  return apiRequest<Snippet>(`/api/v1/snippets/${id}`, { method: 'PATCH', body: input, csrf })
}

export function deleteSnippet(id: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/snippets/${id}`, { method: 'DELETE', csrf })
}

export function previewSnippet(
  id: string,
  values: Record<string, string>,
  csrf: string,
): Promise<{ command: string }> {
  return apiRequest<{ command: string }>(`/api/v1/snippets/${id}/preview`, {
    method: 'POST',
    body: { values },
    csrf,
  })
}

export function runSnippet(
  serverId: string,
  snippetId: string,
  values: Record<string, string>,
  csrf: string,
): Promise<SnippetRun> {
  return apiRequest<SnippetRun>(`/api/v1/servers/${serverId}/snippets/${snippetId}/run`, {
    method: 'POST',
    csrf,
    body: { values, confirm: true },
  })
}

export interface HistoryPoint {
  collected_at: string
  cpu_percent: number | null
  memory_percent: number | null
  disk_percent: number | null
  load_one: number | null
  rx_bps: number | null
  tx_bps: number | null
}

export interface AlertState {
  rule_id: string
  metric: string
  comparator: string
  threshold: number
  value: number
  active: boolean
}

export interface HistoryResult {
  server_id: string
  since: string
  samples: HistoryPoint[]
  alerts: AlertState[]
  retention: string
  data_available: boolean
}

export function getHistory(
  serverId: string,
  range: string,
  signal?: AbortSignal,
): Promise<HistoryResult> {
  return apiRequest<HistoryResult>(`/api/v1/servers/${serverId}/history?range=${range}`, { signal })
}

export interface AlertRule {
  ID?: string
  id?: string
  metric: string
  comparator: string
  threshold: number
  enabled: boolean
}

export function listAlertRules(serverId: string, signal?: AbortSignal): Promise<{ rules: AlertRule[] }> {
  return apiRequest<{ rules: AlertRule[] }>(`/api/v1/servers/${serverId}/alerts`, { signal })
}

export function saveAlertRule(
  serverId: string,
  ruleId: string,
  input: { metric: string; comparator: string; threshold: number; enabled: boolean },
  csrf: string,
): Promise<AlertRule> {
  return apiRequest<AlertRule>(`/api/v1/servers/${serverId}/alerts/${ruleId || 'new'}`, {
    method: 'PUT',
    body: input,
    csrf,
  })
}

export function deleteAlertRule(serverId: string, ruleId: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/servers/${serverId}/alerts/${ruleId}`, {
    method: 'DELETE',
    csrf,
  })
}
