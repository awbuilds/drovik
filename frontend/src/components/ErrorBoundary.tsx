import { Component, type ErrorInfo, type ReactNode } from 'react'
import { RotateCcw, ShieldAlert } from 'lucide-react'
import { reportClientError } from '@/lib/errors'
import { Button } from './ui/Button'

interface Props {
  children: ReactNode
  title?: string
  compact?: boolean
}

interface State {
  hasError: boolean
  errorId: string | null
}

function errorId() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID().slice(0, 8)
  }
  return Math.random().toString(16).slice(2, 10)
}

export class ErrorBoundary extends Component<Props, State> {
  override state: State = { hasError: false, errorId: null }

  static getDerivedStateFromError(): State {
    return { hasError: true, errorId: errorId() }
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    reportClientError(error, {
      boundary: this.props.title ?? 'root',
      componentStack: info.componentStack ?? '',
    })
  }

  private reset = () => {
    this.setState({ hasError: false, errorId: null })
  }

  override render() {
    if (!this.state.hasError) {
      return this.props.children
    }

    if (this.props.compact) {
      return (
        <div className="border-ink-700 bg-ink-900 rounded-lg border p-5">
          <div className="flex items-start gap-3">
            <ShieldAlert className="mt-0.5 h-5 w-5 text-rose-400" aria-hidden="true" />
            <div className="space-y-2">
              <p className="text-ink-100 text-sm font-medium">
                {this.props.title ?? 'This panel could not be displayed.'}
              </p>
              <p className="text-ink-400 text-xs">
                The rest of Drovik is still available. Reference:{' '}
                <span className="font-mono">{this.state.errorId}</span>
              </p>
              <Button size="sm" variant="outline" onClick={this.reset}>
                <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
                Try again
              </Button>
            </div>
          </div>
        </div>
      )
    }

    return (
      <main className="flex min-h-screen items-center justify-center p-6">
        <div className="surface w-full max-w-md p-8 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-rose-500/10 text-rose-400">
            <ShieldAlert className="h-6 w-6" aria-hidden="true" />
          </div>
          <h1 className="font-display text-ink-50 mt-4 text-xl font-semibold">
            Something went wrong
          </h1>
          <p className="text-ink-400 mt-2 text-sm">
            Drovik hit an unexpected error. No server data or credentials were exposed. Reload the
            workspace to continue.
          </p>
          <p className="text-ink-500 mt-3 font-mono text-xs">
            Reference: {this.state.errorId ?? 'unknown'}
          </p>
          <Button className="mt-6 w-full" onClick={() => window.location.reload()}>
            <RotateCcw className="h-4 w-4" aria-hidden="true" />
            Reload Drovik
          </Button>
        </div>
      </main>
    )
  }
}
