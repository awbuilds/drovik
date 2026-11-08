export interface TerminalMessage {
  type: 'output' | 'status' | 'error'
  data?: string
  state?: 'connected' | 'closed' | 'error'
  code?: string
  message?: string
}

export function terminalWebSocketURL(serverId: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/api/v1/servers/${encodeURIComponent(serverId)}/terminal`
}

export function parseTerminalMessage(raw: unknown): TerminalMessage | null {
  if (typeof raw !== 'string') {
    return null
  }
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') {
      return null
    }
    const candidate = parsed as Record<string, unknown>
    if (candidate.type !== 'output' && candidate.type !== 'status' && candidate.type !== 'error') {
      return null
    }
    return {
      type: candidate.type,
      data: typeof candidate.data === 'string' ? candidate.data : undefined,
      state:
        candidate.state === 'connected' || candidate.state === 'closed' || candidate.state === 'error'
          ? candidate.state
          : undefined,
      code: typeof candidate.code === 'string' ? candidate.code : undefined,
      message: typeof candidate.message === 'string' ? candidate.message : undefined,
    }
  } catch {
    return null
  }
}
