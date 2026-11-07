import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Check, ExternalLink, LogOut, Moon, Sun } from 'lucide-react'
import { getVersion } from '@/api/client'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Separator } from '@/components/ui/Separator'
import { useToast } from '@/components/ui/useToast'
import { formatDateTime } from '@/lib/format'
import { useThemeStore } from '@/lib/theme'

export function SettingsPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { theme, setTheme } = useThemeStore()
  const versionQuery = useQuery({
    queryKey: ['version'],
    queryFn: ({ signal }) => getVersion(signal),
    staleTime: 60_000,
  })

  const handleLogout = async () => {
    await logout()
    toast({ title: 'Signed out', description: 'This session was revoked.', variant: 'success' })
    navigate('/login', { replace: true })
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="System"
        title="Settings"
        description="Your local account, session, appearance and build information."
        actions={<Badge variant="outline">Phase 1</Badge>}
      />

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Administrator</CardTitle>
            <CardDescription>The local account that controls this installation.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pt-5">
            <div className="flex items-center gap-3">
              <div className="bg-ink-800 text-signal-300 flex h-11 w-11 items-center justify-center rounded-full font-mono text-sm font-semibold uppercase">
                {user?.username.slice(0, 2) ?? 'DR'}
              </div>
              <div>
                <p className="text-ink-50 text-sm font-medium">{user?.username ?? 'Unknown'}</p>
                <p className="text-ink-500 font-mono text-[10px] uppercase">
                  {user?.role ?? 'admin'}
                </p>
              </div>
            </div>
            <Separator />
            <dl className="grid gap-3 text-xs sm:grid-cols-2">
              <div>
                <dt className="text-ink-500">Created</dt>
                <dd className="text-ink-200 mt-1">{formatDateTime(user?.created_at)}</dd>
              </div>
              <div>
                <dt className="text-ink-500">Last sign-in</dt>
                <dd className="text-ink-200 mt-1">{formatDateTime(user?.last_login_at)}</dd>
              </div>
            </dl>
            <p className="text-ink-500 text-[11px] leading-5">
              Drovik has no email recovery. The administrator password can only be reset through the
              documented administrative recovery procedure.
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Session</CardTitle>
            <CardDescription>Server-side session with automatic expiry.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pt-5">
            <div className="space-y-2 text-xs">
              <div className="flex items-center justify-between gap-4">
                <span className="text-ink-500">Storage</span>
                <span className="text-ink-200">Server-side · hashed token</span>
              </div>
              <div className="flex items-center justify-between gap-4">
                <span className="text-ink-500">Cookie</span>
                <span className="text-ink-200">HttpOnly · SameSite=Lax</span>
              </div>
              <div className="flex items-center justify-between gap-4">
                <span className="text-ink-500">CSRF</span>
                <span className="text-mint-300">Enabled</span>
              </div>
            </div>
            <Separator />
            <Button variant="outline" onClick={() => void handleLogout()} className="w-full">
              <LogOut className="h-4 w-4" aria-hidden="true" />
              Sign out of this session
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Appearance</CardTitle>
            <CardDescription>Dark is the default; light is fully supported.</CardDescription>
          </CardHeader>
          <CardContent className="pt-5">
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setTheme('dark')}
                className={
                  theme === 'dark'
                    ? 'border-signal-500/50 bg-signal-400/10 rounded-lg border p-3 text-left'
                    : 'border-ink-700 bg-ink-900 hover:border-ink-600 rounded-lg border p-3 text-left'
                }
                aria-pressed={theme === 'dark'}
              >
                <Moon className="text-ink-300 h-4 w-4" aria-hidden="true" />
                <p className="text-ink-100 mt-2 text-xs font-medium">Control room</p>
                <p className="text-ink-500 mt-0.5 text-[10px]">Deep ink, warm signal</p>
              </button>
              <button
                type="button"
                onClick={() => setTheme('light')}
                className={
                  theme === 'light'
                    ? 'border-signal-500/50 bg-signal-400/10 rounded-lg border p-3 text-left'
                    : 'border-ink-700 bg-ink-900 hover:border-ink-600 rounded-lg border p-3 text-left'
                }
                aria-pressed={theme === 'light'}
              >
                <Sun className="text-ink-300 h-4 w-4" aria-hidden="true" />
                <p className="text-ink-100 mt-2 text-xs font-medium">Paper terminal</p>
                <p className="text-ink-500 mt-0.5 text-[10px]">Warm paper, same signal</p>
              </button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>About Drovik</CardTitle>
            <CardDescription>Build identity and licensing.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 pt-5">
            <dl className="space-y-2 font-mono text-xs">
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">version</dt>
                <dd className="text-ink-200">{versionQuery.data?.version ?? 'dev'}</dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">commit</dt>
                <dd className="text-ink-200 max-w-[55%] truncate">
                  {versionQuery.data?.commit ?? 'unknown'}
                </dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">go</dt>
                <dd className="text-ink-200">{versionQuery.data?.go_version ?? 'unknown'}</dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-ink-500">schema</dt>
                <dd className="text-ink-200">
                  {versionQuery.data ? `v${versionQuery.data.schema_version}` : 'unknown'}
                </dd>
              </div>
            </dl>
            <Separator />
            <div className="flex flex-wrap gap-2">
              <Badge variant="mint">
                <Check className="h-3 w-3" aria-hidden="true" />
                AGPL-3.0-only
              </Badge>
              <Badge variant="outline">No telemetry</Badge>
            </div>
            <a
              href="https://github.com/awbuilds/drovik"
              target="_blank"
              rel="noreferrer"
              className="text-signal-300 hover:text-signal-200 inline-flex items-center gap-1 text-xs underline underline-offset-2"
            >
              Source repository
              <ExternalLink className="h-3 w-3" aria-hidden="true" />
            </a>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
