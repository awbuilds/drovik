import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { Loader2, RefreshCw, TriangleAlert } from 'lucide-react'
import { AuthProvider } from '@/auth/AuthProvider'
import { useAuth } from '@/auth/useAuth'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Button } from '@/components/ui/Button'
import { Logo } from '@/components/ui/Logo'
import { ToastProvider } from '@/components/ui/Toast'
import { TooltipProvider } from '@/components/ui/Tooltip'
import { AppShell } from '@/layout/AppShell'
import { LoginPage } from '@/pages/LoginPage'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { SettingsPage } from '@/pages/SettingsPage'
import { SetupPage } from '@/pages/SetupPage'
import { WorkspacePage } from '@/pages/WorkspacePage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 30_000,
    },
  },
})

export default function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <BrowserRouter>
            <TooltipProvider delayDuration={250}>
              <ToastProvider>
                <AppRoutes />
              </ToastProvider>
            </TooltipProvider>
          </BrowserRouter>
        </AuthProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}

function AppRoutes() {
  const { status } = useAuth()
  const location = useLocation()

  if (status === 'loading') {
    return <LoadingScreen />
  }

  if (status === 'error') {
    return <BootErrorScreen />
  }

  if (status === 'setup-required') {
    if (location.pathname === '/setup') {
      return (
        <Routes>
          <Route path="/setup" element={<SetupPage />} />
          <Route path="*" element={<Navigate to="/setup" replace />} />
        </Routes>
      )
    }
    return <Navigate to="/setup" replace />
  }

  if (status === 'unauthenticated') {
    if (location.pathname === '/login') {
      return (
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      )
    }
    return <Navigate to="/login" replace />
  }

  if (location.pathname === '/login' || location.pathname === '/setup') {
    return <Navigate to="/" replace />
  }

  return (
    <Routes>
      <Route
        path="/"
        element={
          <AppShell>
            <WorkspacePage />
          </AppShell>
        }
      />
      <Route
        path="/settings"
        element={
          <AppShell>
            <SettingsPage />
          </AppShell>
        }
      />
      <Route
        path="*"
        element={
          <AppShell>
            <NotFoundPage />
          </AppShell>
        }
      />
    </Routes>
  )
}

function LoadingScreen() {
  return (
    <main className="bg-ink-950 flex min-h-screen flex-col items-center justify-center p-6">
      <Logo size="lg" />
      <div className="text-ink-400 mt-8 flex items-center gap-2 font-mono text-[11px] tracking-[0.18em] uppercase">
        <Loader2 className="text-signal-400 h-3.5 w-3.5 animate-spin" aria-hidden="true" />
        Establishing control surface
      </div>
      <div className="mt-6 w-full max-w-xs space-y-2" aria-hidden="true">
        <div className="bg-ink-800 h-1.5 animate-pulse rounded-full" />
        <div className="bg-ink-800 mx-auto h-1.5 w-2/3 animate-pulse rounded-full" />
      </div>
    </main>
  )
}

function BootErrorScreen() {
  const { error, refresh } = useAuth()
  return (
    <main className="bg-ink-950 flex min-h-screen items-center justify-center p-6">
      <div className="surface w-full max-w-md p-8 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-rose-500/10 text-rose-400">
          <TriangleAlert className="h-6 w-6" aria-hidden="true" />
        </div>
        <h1 className="font-display text-ink-50 mt-4 text-xl font-semibold">Cannot reach Drovik</h1>
        <p className="text-ink-400 mt-2 text-sm">
          {error ??
            'The backend did not respond. Check that the Drovik service is running and that the API URL is correct.'}
        </p>
        <Button className="mt-6 w-full" onClick={() => void refresh()}>
          <RefreshCw className="h-4 w-4" aria-hidden="true" />
          Retry connection
        </Button>
        <p className="text-ink-600 mt-4 font-mono text-[10px] uppercase">
          No credentials are stored in the browser
        </p>
      </div>
    </main>
  )
}
