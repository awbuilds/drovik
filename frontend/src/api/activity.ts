import { apiRequest, apiURL } from './client'

export interface AuditEvent {
  id: string
  actor_user_id?: string
  server_id?: string
  event_type: string
  target_summary: string
  outcome: string
  safe_error_code?: string
  correlation_id?: string
  created_at: string
}

export interface ActivityFilters {
  q?: string
  eventType?: string
  outcome?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

function queryString(filters: ActivityFilters): string {
  const params = new URLSearchParams()
  if (filters.q?.trim()) params.set('q', filters.q.trim())
  if (filters.eventType?.trim()) params.set('event_type', filters.eventType.trim())
  if (filters.outcome && filters.outcome !== 'all') params.set('outcome', filters.outcome)
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  params.set('limit', String(filters.limit ?? 100))
  if (filters.offset) params.set('offset', String(filters.offset))
  return params.toString()
}

export function listActivity(
  filters: ActivityFilters = {},
  signal?: AbortSignal,
): Promise<{ events: AuditEvent[] }> {
  return apiRequest<{ events: AuditEvent[] }>(`/api/v1/activity?${queryString(filters)}`, { signal })
}

export function activityExportURL(filters: ActivityFilters, format: 'csv' | 'json'): string {
  const params = new URLSearchParams(queryString(filters))
  params.set('format', format)
  return apiURL(`/api/v1/activity/export?${params.toString()}`)
}
