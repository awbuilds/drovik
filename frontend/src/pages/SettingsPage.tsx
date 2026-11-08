import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Check, ExternalLink, KeyRound, LogOut, Moon, ShieldCheck, Sun } from 'lucide-react'
import {
  disableMFA,
  enableMFA,
  getMFAStatus,
  getOIDCLinkStatus,
  getSetupStatus,
  getVersion,
  setupMFA,
  startOIDCLink,
  unlinkOIDC,
} from '@/api/client'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { Separator } from '@/components/ui/Separator'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'
import { useThemeStore } from '@/lib/theme'
import { ApiError } from '@/api/client'

export function SettingsPage() {
  const { user, logout, csrfToken } = useAuth()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { theme, setTheme } = useThemeStore()
  const [searchParams] = useSearchParams()
  const versionQuery = useQuery({
    queryKey: ['version'],
    queryFn: ({ signal }) => getVersion(signal),
    staleTime: 60_000,
  })
  const setupQuery = useQuery({
    queryKey: ['setup-status'],
    queryFn: ({ signal }) => getSetupStatus(signal),
    staleTime: 60_000,
    retry: 0,
  })
  const oidcEnabled = setupQuery.data?.oidc_enabled === true
  const oidcLinkQuery = useQuery({
    queryKey: ['oidc-link'],
    queryFn: ({ signal }) => getOIDCLinkStatus(signal),
    enabled: oidcEnabled,
    retry: 0,
  })
  const [ssoError, setSsoError] = useState<string | null>(null)
  const [ssoBusy, setSsoBusy] = useState(false)
  const [unlinkOpen, setUnlinkOpen] = useState(false)
  const [unlinkPassword, setUnlinkPassword] = useState('')
  const mfaQuery = useQuery({
    queryKey: ['mfa-status'],
    queryFn: ({ signal }) => getMFAStatus(signal),
    staleTime: 10_000,
  })
  const [enrollment, setEnrollment] = useState<{ secret: string; uri: string } | null>(null)
  const [enrollmentCode, setEnrollmentCode] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [mfaBusy, setMfaBusy] = useState(false)
  const [mfaError, setMfaError] = useState<string | null>(null)
  const [disableOpen, setDisableOpen] = useState(false)
  const [disablePassword, setDisablePassword] = useState('')
  const [mfaSetupOpen, setMfaSetupOpen] = useState(false)
  const [mfaSetupPassword, setMfaSetupPassword] = useState('')
  const [mfaSetupCode, setMfaSetupCode] = useState('')
  const [linkOpen, setLinkOpen] = useState(false)
  const [linkPassword, setLinkPassword] = useState('')
  const [linkCode, setLinkCode] = useState('')

  const beginMFA = () => {
    setMfaSetupPassword('')
    setMfaSetupCode('')
    setMfaError(null)
    setMfaSetupOpen(true)
  }

  const submitMFASetup = async () => {
    setMfaBusy(true)
    setMfaError(null)
    try {
      const result = await setupMFA(
        {
          password: mfaSetupPassword,
          ...(mfaQuery.data?.enabled && mfaSetupCode.trim() ? { code: mfaSetupCode.trim() } : {}),
        },
        csrfToken,
      )
      setEnrollment(result)
      setEnrollmentCode('')
      setRecoveryCodes([])
      setMfaSetupOpen(false)
      setMfaSetupPassword('')
      setMfaSetupCode('')
    } catch (caught) {
      setMfaError(userFacingError(caught))
    } finally {
      setMfaBusy(false)
    }
  }

  const confirmMFA = async () => {
    setMfaBusy(true)
    setMfaError(null)
    try {
      const result = await enableMFA(enrollmentCode.trim(), csrfToken)
      setRecoveryCodes(result.recovery_codes)
      setEnrollment(null)
      toast({ title: 'Two-factor authentication enabled', description: 'Store the recovery codes now.', variant: 'success' })
      await mfaQuery.refetch()
    } catch (caught) {
      setMfaError(caught instanceof ApiError ? caught.message : userFacingError(caught))
    } finally {
      setMfaBusy(false)
    }
  }

  const confirmDisableMFA = async () => {
    setMfaBusy(true)
    setMfaError(null)
    try {
      await disableMFA(disablePassword, csrfToken)
      setDisableOpen(false)
      setDisablePassword('')
      setRecoveryCodes([])
      toast({ title: 'Two-factor authentication disabled', variant: 'success' })
      await mfaQuery.refetch()
    } catch (caught) {
      setMfaError(userFacingError(caught))
    } finally {
      setMfaBusy(false)
    }
  }

  const beginOIDCLink = () => {
    setLinkPassword('')
    setLinkCode('')
    setSsoError(null)
    setLinkOpen(true)
  }

  const submitOIDCLink = async () => {
    setSsoBusy(true)
    setSsoError(null)
    try {
      const result = await startOIDCLink(
        {
          password: linkPassword,
          ...(mfaQuery.data?.enabled && linkCode.trim() ? { code: linkCode.trim() } : {}),
        },
        csrfToken,
      )
      window.location.assign(result.url)
    } catch (caught) {
      setSsoError(userFacingError(caught))
      setSsoBusy(false)
    }
  }

  const confirmUnlinkOIDC = async () => {
    setSsoBusy(true)
    setSsoError(null)
    try {
      await unlinkOIDC(unlinkPassword, csrfToken)
      setUnlinkOpen(false)
      setUnlinkPassword('')
      toast({ title: 'Single sign-on unlinked', variant: 'success' })
      await oidcLinkQuery.refetch()
    } catch (caught) {
      setSsoError(userFacingError(caught))
    } finally {
      setSsoBusy(false)
    }
  }

  const ssoNotice = searchParams.get('oidc_linked')
    ? 'Single sign-on is now linked to this account.'
    : searchParams.get('oidc_error') === 'already_linked'
      ? 'This local account already has a different single sign-on identity.'
      : searchParams.get('oidc_error')
        ? 'Single sign-on linking failed. Try again or contact an administrator.'
        : null

  const handleLogout = async () => {
    try {
      await logout()
      toast({ title: 'Signed out', description: 'This session was revoked.', variant: 'success' })
      navigate('/login', { replace: true })
    } catch (caught) {
      toast({
        title: 'Sign out failed',
        description: `${userFacingError(caught)} The session is still active.`,
        variant: 'error',
      })
    }
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
            <CardTitle className="flex items-center gap-2"><ShieldCheck className="text-signal-400 h-4 w-4" aria-hidden="true" />Two-factor authentication</CardTitle>
            <CardDescription>TOTP authenticator codes plus single-use recovery codes.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pt-5">
            {mfaError ? <Alert variant="error">{mfaError}</Alert> : null}
            {recoveryCodes.length > 0 ? (
              <Alert variant="warning" title="Store these recovery codes now">
                <div className="mt-2 grid grid-cols-2 gap-1 font-mono text-[11px]">
                  {recoveryCodes.map((code) => <span key={code}>{code}</span>)}
                </div>
                <p className="mt-2 text-[11px]">Each code works once and cannot be shown again.</p>
              </Alert>
            ) : null}
            {enrollment ? (
              <div className="space-y-3">
                <p className="text-ink-400 text-xs">Add this secret to your authenticator, then confirm a code.</p>
                <pre className="bg-ink-950 text-ink-200 overflow-auto rounded-md p-3 font-mono text-[11px] whitespace-pre-wrap">{enrollment.secret}</pre>
                <p className="text-ink-500 font-mono text-[10px] break-all">{enrollment.uri}</p>
                <Field label="Authenticator code" htmlFor="mfa-enroll-code">
                  <Input id="mfa-enroll-code" value={enrollmentCode} onChange={(event) => setEnrollmentCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" className="max-w-40 font-mono" />
                </Field>
                <div className="flex gap-2">
                  <Button onClick={() => void confirmMFA()} loading={mfaBusy} disabled={!enrollmentCode.trim()}>Confirm and enable</Button>
                  <Button variant="ghost" onClick={() => { setEnrollment(null); setMfaError(null) }} disabled={mfaBusy}>Cancel</Button>
                </div>
              </div>
            ) : (
              <div className="flex flex-wrap items-center gap-3">
                <Badge variant={mfaQuery.data?.enabled ? 'mint' : 'outline'}>{mfaQuery.data?.enabled ? 'enabled' : 'disabled'}</Badge>
                {mfaQuery.data?.enabled ? (
                  <Button variant="outline" size="sm" onClick={() => { setDisableOpen(true); setMfaError(null) }}>Disable MFA</Button>
                ) : (
                  <Button variant="outline" size="sm" onClick={beginMFA} loading={mfaBusy}>{mfaQuery.data?.enabled ? 'Replace MFA' : 'Enable MFA'}</Button>
                )}
              </div>
            )}
            <p className="text-ink-500 text-[11px] leading-5">If you lose the authenticator and recovery codes, the account cannot sign in. There is no email recovery.</p>
          </CardContent>
        </Card>

        {oidcEnabled ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <KeyRound className="text-signal-400 h-4 w-4" aria-hidden="true" />
                Single sign-on
              </CardTitle>
              <CardDescription>Link this local account to the configured OpenID Connect identity.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 pt-5">
              {ssoNotice ? <Alert variant={searchParams.get('oidc_linked') ? 'success' : 'error'}>{ssoNotice}</Alert> : null}
              {ssoError ? <Alert variant="error">{ssoError}</Alert> : null}
              {oidcLinkQuery.isError ? (
                <Alert variant="warning">The single sign-on link status could not be read.</Alert>
              ) : (
                <div className="flex flex-wrap items-center gap-3">
                  <Badge variant={oidcLinkQuery.data?.linked ? 'mint' : 'outline'}>
                    {oidcLinkQuery.data?.linked ? 'linked' : 'not linked'}
                  </Badge>
                  {oidcLinkQuery.data?.linked ? (
                    <Button variant="outline" size="sm" onClick={() => { setUnlinkOpen(true); setSsoError(null) }}>
                      Unlink identity
                    </Button>
                  ) : (
                    <Button variant="outline" size="sm" onClick={beginOIDCLink} loading={ssoBusy}>
                      Link identity
                    </Button>
                  )}
                </div>
              )}
              <p className="text-ink-500 text-[11px] leading-5">
                Single sign-on never creates an account. An administrator must create the local
                account first, then you link it here. The identity provider's own policy is the
                authentication boundary for SSO sign-in.
              </p>
            </CardContent>
          </Card>
        ) : null}

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

      <Dialog
        open={mfaSetupOpen}
        onOpenChange={setMfaSetupOpen}
        title={mfaQuery.data?.enabled ? 'Replace two-factor authentication' : 'Enable two-factor authentication'}
        description="Confirm your password before generating a new authenticator secret."
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setMfaSetupOpen(false)} disabled={mfaBusy}>Cancel</Button>
            <Button onClick={() => void submitMFASetup()} loading={mfaBusy} disabled={!mfaSetupPassword}>Continue</Button>
          </>
        }
      >
        <div className="space-y-4">
          {mfaError ? <Alert variant="error">{mfaError}</Alert> : null}
          <Field label="Password" htmlFor="mfa-setup-password">
            <Input id="mfa-setup-password" type="password" value={mfaSetupPassword} onChange={(event) => setMfaSetupPassword(event.target.value)} autoComplete="current-password" />
          </Field>
          {mfaQuery.data?.enabled ? (
            <Field label="Current authenticator code" htmlFor="mfa-setup-code" hint="Required to replace an active factor.">
              <Input id="mfa-setup-code" value={mfaSetupCode} onChange={(event) => setMfaSetupCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" className="font-mono" />
            </Field>
          ) : null}
        </div>
      </Dialog>

      <Dialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        title="Link single sign-on"
        description="Confirm your local password before adding a federated login to this account."
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setLinkOpen(false)} disabled={ssoBusy}>Cancel</Button>
            <Button onClick={() => void submitOIDCLink()} loading={ssoBusy} disabled={!linkPassword}>Continue to provider</Button>
          </>
        }
      >
        <div className="space-y-4">
          {ssoError ? <Alert variant="error">{ssoError}</Alert> : null}
          <Field label="Password" htmlFor="oidc-link-password">
            <Input id="oidc-link-password" type="password" value={linkPassword} onChange={(event) => setLinkPassword(event.target.value)} autoComplete="current-password" />
          </Field>
          {mfaQuery.data?.enabled ? (
            <Field label="Authenticator code" htmlFor="oidc-link-code" hint="Required because MFA is enabled.">
              <Input id="oidc-link-code" value={linkCode} onChange={(event) => setLinkCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" className="font-mono" />
            </Field>
          ) : null}
        </div>
      </Dialog>

      <Dialog
        open={unlinkOpen}
        onOpenChange={setUnlinkOpen}
        title="Unlink single sign-on"
        description="Confirm your password to remove the linked identity. Password sign-in keeps working."
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setUnlinkOpen(false)} disabled={ssoBusy}>Cancel</Button>
            <Button variant="danger" onClick={() => void confirmUnlinkOIDC()} loading={ssoBusy} disabled={!unlinkPassword}>Unlink</Button>
          </>
        }
      >
        <div className="space-y-4">
          {ssoError ? <Alert variant="error">{ssoError}</Alert> : null}
          <Field label="Password" htmlFor="oidc-unlink-password">
            <Input id="oidc-unlink-password" type="password" value={unlinkPassword} onChange={(event) => setUnlinkPassword(event.target.value)} autoComplete="current-password" />
          </Field>
        </div>
      </Dialog>

      <Dialog
        open={disableOpen}
        onOpenChange={setDisableOpen}
        title="Disable two-factor authentication"
        description="Confirm your password to remove the authenticator and recovery codes."
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setDisableOpen(false)} disabled={mfaBusy}>Cancel</Button>
            <Button variant="danger" onClick={() => void confirmDisableMFA()} loading={mfaBusy} disabled={!disablePassword}>Disable MFA</Button>
          </>
        }
      >
        <div className="space-y-4">
          {mfaError ? <Alert variant="error">{mfaError}</Alert> : null}
          <Field label="Password" htmlFor="mfa-disable-password">
            <Input id="mfa-disable-password" type="password" value={disablePassword} onChange={(event) => setDisablePassword(event.target.value)} autoComplete="current-password" />
          </Field>
        </div>
      </Dialog>
    </div>
  )
}
