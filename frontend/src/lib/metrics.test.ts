import { describe, expect, it } from 'vitest'
import { formatPercent, formatRate, formatUptime, memoryUsedPercent } from './metrics'

describe('metric formatting', () => {
  it('formats percentages with honest unavailable values', () => {
    expect(formatPercent(0)).toBe('0.0%')
    expect(formatPercent(12.34)).toBe('12%')
    expect(formatPercent(4.2)).toBe('4.2%')
    expect(formatPercent(null)).toBe('—')
  })

  it('formats rates from bytes per second', () => {
    expect(formatRate(0)).toBe('0 B/s')
    expect(formatRate(2048)).toBe('2.0 KiB/s')
    expect(formatRate(null)).toBe('—')
    expect(formatRate(undefined)).toBe('—')
  })

  it('formats uptime at useful precision', () => {
    expect(formatUptime(45)).toBe('45s')
    expect(formatUptime(3660)).toBe('1h 1m')
    expect(formatUptime(90061)).toBe('1d 1h')
    expect(formatUptime(-1)).toBe('—')
  })

  it('computes memory percentages defensively', () => {
    expect(memoryUsedPercent(50, 100)).toBe(50)
    expect(memoryUsedPercent(0, 0)).toBeNull()
    expect(memoryUsedPercent(150, 100)).toBe(100)
  })
})
