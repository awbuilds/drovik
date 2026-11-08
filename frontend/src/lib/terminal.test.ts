import { describe, expect, it } from 'vitest'
import { parseTerminalMessage, terminalWebSocketURL } from './terminal'

describe('terminal helpers', () => {
  it('parses output, status and error messages', () => {
    expect(parseTerminalMessage('{"type":"output","data":"hello"}')).toEqual({
      type: 'output',
      data: 'hello',
      state: undefined,
      code: undefined,
      message: undefined,
    })
    expect(parseTerminalMessage('{"type":"status","state":"connected"}')?.state).toBe('connected')
    expect(parseTerminalMessage('{"type":"error","code":"SSH_AUTH_FAILED"}')?.code).toBe(
      'SSH_AUTH_FAILED',
    )
  })

  it('rejects malformed or unknown messages', () => {
    expect(parseTerminalMessage('not-json')).toBeNull()
    expect(parseTerminalMessage('null')).toBeNull()
    expect(parseTerminalMessage('{"type":"unknown"}')).toBeNull()
    expect(parseTerminalMessage({ type: 'output' })).toBeNull()
  })

  it('builds a same-origin WebSocket URL with an encoded server id', () => {
    const url = terminalWebSocketURL('server id/with slash')
    expect(url).toContain('/api/v1/servers/server%20id%2Fwith%20slash/terminal')
    expect(url.startsWith('ws://') || url.startsWith('wss://')).toBe(true)
  })
})
