import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { PlugZap, RotateCcw, X } from 'lucide-react'
import type { Server } from '@/api/servers'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { StatusDot } from '@/components/ui/StatusDot'
import { reportClientError } from '@/lib/errors'
import { parseTerminalMessage, terminalWebSocketURL } from '@/lib/terminal'
import { useThemeStore } from '@/lib/theme'

type ConnectionState = 'connecting' | 'connected' | 'closed' | 'error'

const statusMap: Record<ConnectionState, { label: string; status: 'online' | 'offline' | 'warning' | 'pending' }> = {
  connecting: { label: 'Connecting', status: 'pending' },
  connected: { label: 'Connected', status: 'online' },
  closed: { label: 'Disconnected', status: 'offline' },
  error: { label: 'Connection error', status: 'warning' },
}

function cssColor(name: string, fallback: string): string {
  try {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
  } catch {
    return fallback
  }
}

function terminalTheme() {
  return {
    background: cssColor('--color-ink-950', '#07090c'),
    foreground: cssColor('--color-ink-100', '#dfe5ec'),
    cursor: cssColor('--color-signal-400', '#f7bd57'),
    selectionBackground: cssColor('--color-signal-500', '#e9a536'),
    black: cssColor('--color-ink-950', '#07090c'),
    brightBlack: cssColor('--color-ink-500', '#354254'),
    red: cssColor('--color-rose-400', '#fb7185'),
    brightRed: cssColor('--color-rose-300', '#fda4af'),
    green: cssColor('--color-mint-400', '#5eead4'),
    brightGreen: cssColor('--color-mint-300', '#99f6e4'),
    yellow: cssColor('--color-signal-400', '#f7bd57'),
    brightYellow: cssColor('--color-signal-300', '#ffd98a'),
    blue: '#7dd3fc',
    brightBlue: '#bae6fd',
    magenta: '#d8b4fe',
    brightMagenta: '#e9d5ff',
    cyan: '#67e8f9',
    brightCyan: '#a5f3fc',
    white: cssColor('--color-ink-100', '#dfe5ec'),
    brightWhite: cssColor('--color-ink-50', '#f4f6f9'),
  }
}

export function TerminalPanel({ server }: { server: Server }) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<Terminal | null>(null)
  const closeRef = useRef<() => void>(() => {})
  const { theme } = useThemeStore()
  const [state, setState] = useState<ConnectionState>('connecting')
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    let disposed = false
    let socket: WebSocket | null = null
    let resizeObserver: ResizeObserver | null = null
    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: "'JetBrains Mono Variable', ui-monospace, monospace",
      fontSize: 13,
      lineHeight: 1.35,
      scrollback: 5000,
      convertEol: false,
      theme: terminalTheme(),
    })
    terminalRef.current = terminal
    const fit = new FitAddon()
    try {
      terminal.loadAddon(fit)
      terminal.open(container)
    } catch (caught) {
      reportClientError(caught, { operation: 'terminal.open' })
      terminal.dispose()
      // oxlint-disable-next-line react/set-state-in-effect -- terminal initialization failure must surface in the UI
      setError('The terminal could not be initialized.')
      // oxlint-disable-next-line react/set-state-in-effect -- terminal initialization failure must surface in the UI
      setState('error')
      return
    }

    const fitSafely = () => {
      try {
        fit.fit()
      } catch (caught) {
        reportClientError(caught, { operation: 'terminal.fit' })
      }
    }
    requestAnimationFrame(fitSafely)

    const send = (value: unknown) => {
      try {
        if (socket?.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify(value))
        }
      } catch (caught) {
        reportClientError(caught, { operation: 'terminal.send' })
      }
    }

    try {
      socket = new WebSocket(terminalWebSocketURL(server.id))
      socket.onopen = () => {
        if (disposed) {
          try {
            socket?.close()
          } catch {
            // The socket is already gone.
          }
          return
        }
        try {
          setState('connected')
          setError(null)
          send({ type: 'resize', cols: terminal.cols, rows: terminal.rows })
          terminal.focus()
        } catch (caught) {
          reportClientError(caught, { operation: 'terminal.open-handler' })
          setError('The terminal could not be focused.')
        }
      }
      socket.onmessage = (event) => {
        if (disposed) return
        try {
          const message = parseTerminalMessage(event.data)
          if (!message) return
          if (message.type === 'output' && message.data) {
            terminal.write(message.data)
          } else if (message.type === 'status') {
            if (message.state === 'connected') setState('connected')
            if (message.state === 'closed') setState('closed')
          } else if (message.type === 'error') {
            setError(message.message ?? 'The terminal session failed.')
            setState('error')
          }
        } catch (caught) {
          reportClientError(caught, { operation: 'terminal.message' })
        }
      }
      closeRef.current = () => {
        try {
          if (socket?.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: 'close' }))
          }
        } catch {
          // Closing is best-effort.
        }
      }
      socket.onerror = () => {
        if (!disposed) setState('error')
      }
      socket.onclose = () => {
        if (!disposed) setState((current) => (current === 'error' ? 'error' : 'closed'))
      }
    } catch (caught) {
      reportClientError(caught, { operation: 'terminal.connect' })
      // oxlint-disable-next-line react/set-state-in-effect -- connection setup failure must surface in the UI
      setError('The terminal could not be opened.')
      // oxlint-disable-next-line react/set-state-in-effect -- connection setup failure must surface in the UI
      setState('error')
    }

    const dataDisposable = terminal.onData((data) => {
      send({ type: 'input', data })
    })
    const resizeDisposable = terminal.onResize(({ cols, rows }) => {
      send({ type: 'resize', cols, rows })
    })

    try {
      resizeObserver = new ResizeObserver(fitSafely)
      resizeObserver.observe(container)
    } catch (caught) {
      reportClientError(caught, { operation: 'terminal.resize-observer' })
    }
    window.addEventListener('resize', fitSafely)

    return () => {
      disposed = true
      closeRef.current = () => {}
      try {
        if (socket?.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify({ type: 'close' }))
          socket.close()
        } else if (socket?.readyState === WebSocket.CONNECTING) {
          // Closing while CONNECTING produces a browser warning. Let onopen
          // observe the disposed flag and close immediately instead.
          socket.onopen = () => {
            try {
              socket?.close()
            } catch {
              // The socket is already gone.
            }
          }
        }
      } catch {
        // The socket is already gone; cleanup is best-effort.
      }
      dataDisposable.dispose()
      resizeDisposable.dispose()
      resizeObserver?.disconnect()
      window.removeEventListener('resize', fitSafely)
      terminal.dispose()
      terminalRef.current = null
    }
  }, [server.id, attempt])

  useEffect(() => {
    try {
      if (terminalRef.current) {
        terminalRef.current.options.theme = terminalTheme()
      }
    } catch {
      // A theme update must never crash the terminal.
    }
  }, [theme])

  const status = statusMap[state]

  return (
    <div className="space-y-3">
      <div className="border-ink-700 bg-ink-900 flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3">
        <StatusDot status={status.status} label={status.label} pulse={state === 'connecting'} />
        <Badge variant="outline">
          {server.username}@{server.host}:{server.port}
        </Badge>
        <span className="text-ink-500 ml-auto font-mono text-[10px] uppercase">
          Commands run on the remote host
        </span>
        {state !== 'connected' && state !== 'connecting' ? (
          <Button size="sm" variant="outline" onClick={() => setAttempt((value) => value + 1)}>
            <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
            Reconnect
          </Button>
        ) : state === 'connected' ? (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              closeRef.current()
              setState('closed')
            }}
          >
            <X className="h-3.5 w-3.5" aria-hidden="true" />
            Close session
          </Button>
        ) : null}
      </div>

      {error ? (
        <div className="border-rose-500/30 bg-rose-500/10 text-rose-200 rounded-md border px-3 py-2 text-xs">
          {error}
        </div>
      ) : null}

      <div className="border-ink-700 bg-ink-950 relative overflow-hidden rounded-lg border">
        {state === 'connecting' ? (
          <div className="text-ink-500 pointer-events-none absolute top-3 left-4 z-10 flex items-center gap-2 font-mono text-[10px] uppercase">
            <PlugZap className="text-signal-400 h-3 w-3 animate-pulse" aria-hidden="true" />
            Opening SSH PTY
          </div>
        ) : null}
        <div ref={containerRef} className="scrollbar-thin h-[62vh] min-h-96 w-full p-2" />
      </div>
    </div>
  )
}
