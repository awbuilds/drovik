import { apiRequest } from './client'

export type AuthMethod = 'password' | 'private_key'

export interface Server {
  id: string
  display_name: string
  host: string
  port: number
  username: string
  auth_method: AuthMethod
  tags: string
  notes: string
  created_at: string
  updated_at: string
  last_connected_at?: string | null
  is_active: boolean
}

export interface HostKeyInfo {
  key_type: string
  fingerprint: string
  public_key: string
}

export interface Capabilities {
  os: string
  kernel: string
  architecture: string
  distribution: string
  systemd: boolean
  journalctl: boolean
  proc: boolean
  process_tools: boolean
  disk_tools: boolean
  sftp: boolean
  detected_at: string
}

export interface ConnectionTest {
  server: Server
  host_key: HostKeyInfo
  capabilities: Capabilities
}

export interface CreateServerInput {
  display_name: string
  host: string
  port: number
  username: string
  auth_method: AuthMethod
  password?: string
  private_key?: string
  passphrase?: string
  tags?: string
  notes?: string
}

export type UpdateServerInput = Partial<CreateServerInput>

export interface TrustHostKeyInput {
  fingerprint: string
  key_type: string
  public_key: string
  replace?: boolean
}

export function listServers(signal?: AbortSignal): Promise<{ servers: Server[] }> {
  return apiRequest<{ servers: Server[] }>('/api/v1/servers', { signal })
}

export function createServer(input: CreateServerInput, csrf: string): Promise<Server> {
  return apiRequest<Server>('/api/v1/servers', { method: 'POST', body: input, csrf })
}

export function updateServer(id: string, input: UpdateServerInput, csrf: string): Promise<Server> {
  return apiRequest<Server>(`/api/v1/servers/${id}`, { method: 'PATCH', body: input, csrf })
}

export function deleteServer(id: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/servers/${id}`, { method: 'DELETE', csrf })
}

export function testServer(id: string, csrf: string): Promise<ConnectionTest> {
  return apiRequest<ConnectionTest>(`/api/v1/servers/${id}/test`, { method: 'POST', csrf })
}

export function trustHostKey(id: string, input: TrustHostKeyInput, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/servers/${id}/trust-host-key`, {
    method: 'POST',
    body: input,
    csrf,
  })
}

export function getServerCapabilities(id: string, signal?: AbortSignal): Promise<Capabilities> {
  return apiRequest<Capabilities>(`/api/v1/servers/${id}/capabilities`, { signal })
}
