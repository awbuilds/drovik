import { apiRequest } from './client'

export interface AuditEvent {
  id: string
  server_id?: string
  event_type: string
  target_summary: string
  outcome: string
  safe_error_code?: string
  created_at: string
}

export function listActivity(limit = 100, signal?: AbortSignal): Promise<{ events: AuditEvent[] }> {
  return apiRequest<{ events: AuditEvent[] }>(`/api/v1/activity?limit=${limit}`, { signal })
}
