import { useEffect, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { NavLink, useNavigate } from 'react-router-dom'
import {
  Activity,
  Boxes,
  ChevronRight,
  Code2,
  FolderTree,
  Gauge,
  LayoutDashboard,
  ListTree,
  ScrollText,
  Settings2,
  LogOut,
  Menu,
  Moon,
  Server,
  Settings,
  SquareTerminal,
  Sun,
  Users,
  X,
  type LucideIcon,
} from 'lucide-react'
import { useAuth } from '@/auth/useAuth'
import { reportClientError } from '@/lib/errors'
import { listServers } from '@/api/servers'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Logo } from '@/components/ui/Logo'
import { StatusDot } from '@/components/ui/StatusDot'
import { Tooltip } from '@/components/ui/Tooltip'
import { cn } from '@/lib/cn'
import { useThemeStore } from '@/lib/theme'

interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  enabled: boolean
  phase?: string
  adminOnly?: boolean
}

const navigation: Array<{ label: string; items: NavItem[] }> = [
  {
    label: 'Workspace',
    items: [
      { to: '/', label: 'Overview', icon: LayoutDashboard, enabled: true },
      { to: '/servers', label: 'Servers', icon: Server, enabled: true },
      { to: '/terminal', label: 'Terminal', icon: SquareTerminal, enabled: true },
      { to: '/files', label: 'Files', icon: FolderTree, enabled: true },
    ],
  },
  {
    label: 'Observe',
    items: [
      { to: '/monitoring', label: 'Monitoring', icon: Gauge, enabled: true },
      { to: '/logs', label: 'Logs', icon: ScrollText, enabled: true },
      { to: '/activity', label: 'Activity', icon: Activity, enabled: true },
    ],
  },
  {
    label: 'Operate',
    items: [
      { to: '/processes', label: 'Processes', icon: ListTree, enabled: true },
      { to: '/services', label: 'Services', icon: Settings2, enabled: true },
      { to: '/advanced', label: 'Advanced', icon: Boxes, enabled: true },
      { to: '/snippets', label: 'Snippets', icon: Code2, enabled: true },
    ],
  },
  {
    label: 'System',
    items: [
      { to: '/users', label: 'Users', icon: Users, enabled: true, adminOnly: true },
      { to: '/teams', label: 'Teams', icon: Users, enabled: true, adminOnly: true },
      { to: '/settings', label: 'Settings', icon: Settings, enabled: true },
    ],
  },
]

export function AppShell({ children }: { children: ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false)
  const { user, logout } = useAuth()
  const { theme, toggleTheme } = useThemeStore()
  const navigate = useNavigate()
  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
    staleTime: 30_000,
  })
  const serverCount = serversQuery.data?.servers.length ?? 0

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setMobileOpen(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const handleLogout = async () => {
    try {
      await logout()
      navigate('/login', { replace: true })
    } catch (caught) {
      // Keep the authenticated shell; the server-side session was not revoked.
      reportClientError(caught, { operation: 'auth.logout' })
    }
  }

  return (
    <div className="bg-ink-950 min-h-screen lg:grid lg:grid-cols-[264px_minmax(0,1fr)]">
      <aside className="border-ink-800 bg-ink-900/70 hidden min-h-screen flex-col border-r lg:flex">
        <SidebarContent user={user} onLogout={handleLogout} />
      </aside>

      {mobileOpen ? (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            className="bg-ink-950/80 absolute inset-0 backdrop-blur-sm"
            aria-label="Close navigation"
            onClick={() => setMobileOpen(false)}
          />
          <aside className="border-ink-800 bg-ink-900 absolute inset-y-0 left-0 flex w-72 flex-col border-r shadow-2xl">
            <div className="flex items-center justify-between px-5 pt-5">
              <Logo size="sm" />
              <Button variant="ghost" size="icon" onClick={() => setMobileOpen(false)}>
                <X className="h-4 w-4" aria-hidden="true" />
                <span className="sr-only">Close navigation</span>
              </Button>
            </div>
            <SidebarContent
              user={user}
              onLogout={handleLogout}
              onNavigate={() => setMobileOpen(false)}
            />
          </aside>
        </div>
      ) : null}

      <div className="flex min-h-screen min-w-0 flex-col">
        <header className="border-ink-800 bg-ink-950/85 sticky top-0 z-30 flex h-16 items-center gap-3 border-b px-4 backdrop-blur sm:px-6">
          <Button
            variant="ghost"
            size="icon"
            className="lg:hidden"
            onClick={() => setMobileOpen(true)}
          >
            <Menu className="h-4 w-4" aria-hidden="true" />
            <span className="sr-only">Open navigation</span>
          </Button>

          <div className="hidden items-center gap-2 sm:flex">
            <StatusDot
              status={serverCount > 0 ? 'online' : 'offline'}
              label={
                serverCount > 0
                  ? `${serverCount} server${serverCount === 1 ? '' : 's'} saved`
                  : 'No server connected'
              }
            />
            <Badge variant="outline">SSH</Badge>
          </div>

          <div className="ml-auto flex items-center gap-2">
            <Tooltip label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}>
              <Button variant="ghost" size="icon" onClick={toggleTheme} aria-label="Toggle theme">
                {theme === 'dark' ? (
                  <Sun className="h-4 w-4" aria-hidden="true" />
                ) : (
                  <Moon className="h-4 w-4" aria-hidden="true" />
                )}
              </Button>
            </Tooltip>
            <div className="border-ink-700 hidden items-center gap-2 border-l pl-3 sm:flex">
              <div className="bg-ink-800 text-signal-300 flex h-8 w-8 items-center justify-center rounded-full font-mono text-xs font-semibold uppercase">
                {user?.username.slice(0, 2) ?? 'DR'}
              </div>
              <div className="leading-tight">
                <p className="text-ink-100 text-xs font-medium">{user?.username ?? 'Operator'}</p>
                <p className="text-ink-500 font-mono text-[10px] uppercase">
                  {user?.role ?? 'admin'}
                </p>
              </div>
            </div>
          </div>
        </header>

        <main id="main-content" className="mx-auto w-full max-w-[1400px] flex-1 px-4 py-6 sm:px-6 lg:px-8" tabIndex={-1}>
          {children}
        </main>
      </div>
    </div>
  )
}

function SidebarContent({
  user,
  onLogout,
  onNavigate,
}: {
  user: { username: string; role: string } | null
  onLogout: () => Promise<void>
  onNavigate?: () => void
}) {
  return (
    <>
      <div className="hidden items-center px-5 pt-6 lg:flex">
        <Logo />
      </div>

      <nav className="flex-1 scrollbar-thin overflow-y-auto px-3 py-6" aria-label="Primary">
        {navigation.map((section) => (
          <div key={section.label} className="mb-6">
            <p className="text-ink-500 mb-2 px-2 font-mono text-[10px] tracking-[0.2em] uppercase">
              {section.label}
            </p>
            <ul className="space-y-1">
              {section.items
                .filter((item) => !item.adminOnly || user?.role === 'admin')
                .map((item) => {
                const Icon = item.icon
                if (!item.enabled) {
                  return (
                    <li key={item.to}>
                      <div
                        className="text-ink-500 flex cursor-not-allowed items-center gap-2.5 rounded-md px-2 py-2 text-sm"
                        aria-disabled="true"
                        title={`${item.label} is planned for ${item.phase}`}
                      >
                        <Icon className="h-4 w-4" aria-hidden="true" />
                        <span className="flex-1">{item.label}</span>
                        <span className="border-ink-700 text-ink-600 rounded border px-1.5 py-0.5 font-mono text-[9px] uppercase">
                          {item.phase}
                        </span>
                      </div>
                    </li>
                  )
                }
                return (
                  <li key={item.to}>
                    <NavLink
                      to={item.to}
                      end={item.to === '/'}
                      onClick={onNavigate}
                      className={({ isActive }) =>
                        cn(
                          'group flex items-center gap-2.5 rounded-md px-2 py-2 text-sm transition-colors',
                          isActive
                            ? 'bg-ink-800 text-ink-50 shadow-inset-line'
                            : 'text-ink-300 hover:bg-ink-800/60 hover:text-ink-100',
                        )
                      }
                    >
                      {({ isActive }) => (
                        <>
                          <Icon
                            className={cn('h-4 w-4', isActive ? 'text-signal-400' : 'text-ink-500')}
                            aria-hidden="true"
                          />
                          <span className="flex-1">{item.label}</span>
                          {isActive ? (
                            <ChevronRight className="text-ink-500 h-3.5 w-3.5" aria-hidden="true" />
                          ) : null}
                        </>
                      )}
                    </NavLink>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </nav>

      <div className="border-ink-800 border-t p-3">
        <div className="border-ink-700 bg-ink-850 flex items-center gap-2 rounded-md border px-2.5 py-2">
          <div className="bg-ink-800 text-signal-300 flex h-7 w-7 items-center justify-center rounded-full font-mono text-[10px] font-semibold uppercase">
            {user?.username.slice(0, 2) ?? 'DR'}
          </div>
          <div className="min-w-0 flex-1 leading-tight">
            <p className="text-ink-100 truncate text-xs font-medium">
              {user?.username ?? 'Operator'}
            </p>
            <p className="text-ink-500 font-mono text-[9px] uppercase">{user?.role ?? 'admin'}</p>
          </div>
          <Tooltip label="Sign out">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => {
                onNavigate?.()
                void onLogout()
              }}
              aria-label="Sign out"
            >
              <LogOut className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </Tooltip>
        </div>
        <p className="text-ink-600 mt-2 px-1 font-mono text-[9px] tracking-wider uppercase">
          Phase 9 · Advanced operations
        </p>
      </div>
    </>
  )
}
