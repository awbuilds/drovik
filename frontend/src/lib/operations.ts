import type { LogEntry } from '@/api/operations'

export type HealthStatus = 'online' | 'offline' | 'warning' | 'pending'
export type SeverityVariant = 'neutral' | 'signal' | 'mint' | 'rose' | 'outline'

export function processHealth(state: string): HealthStatus {
  const code = state.slice(0, 1).toUpperCase()
  switch (code) {
    case 'R':
    case 'S':
    case 'D':
      return 'online'
    case 'Z':
    case 'T':
    case 't':
    case 'X':
      return 'warning'
    default:
      return 'pending'
  }
}

export function serviceHealth(active: string): HealthStatus {
  switch (active) {
    case 'active':
      return 'online'
    case 'failed':
      return 'warning'
    case 'activating':
    case 'deactivating':
    case 'reloading':
      return 'pending'
    default:
      return 'offline'
  }
}

export function severityVariant(severity: string): SeverityVariant {
  switch (severity) {
    case 'emergency':
    case 'alert':
    case 'critical':
    case 'error':
      return 'rose'
    case 'warning':
      return 'signal'
    case 'notice':
      return 'mint'
    default:
      return 'neutral'
  }
}

export function filterLogs(entries: LogEntry[], query: string): LogEntry[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return entries
  return entries.filter((entry) => {
    return (
      entry.message.toLowerCase().includes(needle) ||
      entry.severity.toLowerCase().includes(needle) ||
      (entry.unit ?? '').toLowerCase().includes(needle) ||
      (entry.identifier ?? '').toLowerCase().includes(needle)
    )
  })
}
