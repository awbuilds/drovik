import { describe, expect, it } from 'vitest'
import { filterLogs, processHealth, serviceHealth, severityVariant } from './operations'
import type { LogEntry } from '@/api/operations'

const entry = (message: string, severity = 'info'): LogEntry => ({
  timestamp: '2026-11-08T00:00:00Z',
  priority: 6,
  severity,
  message,
})

describe('operations helpers', () => {
  it('maps process states to health', () => {
    expect(processHealth('Rsl')).toBe('online')
    expect(processHealth('S')).toBe('online')
    expect(processHealth('Z')).toBe('warning')
    expect(processHealth('T')).toBe('warning')
    expect(processHealth('?')).toBe('pending')
  })

  it('maps service and severity states', () => {
    expect(serviceHealth('active')).toBe('online')
    expect(serviceHealth('failed')).toBe('warning')
    expect(serviceHealth('inactive')).toBe('offline')
    expect(severityVariant('error')).toBe('rose')
    expect(severityVariant('warning')).toBe('signal')
    expect(severityVariant('info')).toBe('neutral')
  })

  it('filters log entries by text without interpreting content', () => {
    const entries = [entry('ssh accepted connection'), entry('<script>alert(1)</script>', 'warning')]
    expect(filterLogs(entries, 'ACCEPTED')).toHaveLength(1)
    expect(filterLogs(entries, '<script>')[0]?.message).toContain('<script>')
    expect(filterLogs(entries, 'warning')).toHaveLength(1)
    expect(filterLogs(entries, '')).toHaveLength(2)
  })
})
