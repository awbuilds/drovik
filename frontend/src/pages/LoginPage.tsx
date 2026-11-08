import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { LockKeyhole, ShieldCheck, UserRound } from 'lucide-react'
import { ApiError, getSetupStatus, oidcStartURL } from '@/api/client'
import { useAuth } from '@/auth/useAuth'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { AuthLayout } from '@/layout/AuthLayout'
import { userFacingError } from '@/lib/errors'

const oidcErrors: Record<string, string> = {
  no_account: 'No local Drovik account is linked to that identity. Ask an administrator to create your account first.',
  link: 'The local account could not be linked to this identity. Ask an administrator to check the linking.',
  invalid: 'The sign-in request expired or was already used. Start again.',
  failed: 'The identity provider could not complete sign-in. Try again or contact your administrator.',
  disabled: 'Single sign-on is not enabled on this instance.',
}

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const oidcError = searchParams.get('oidc_error')
  const setupQuery = useQuery({
    queryKey: ['setup-status'],
    queryFn: ({ signal }) => getSetupStatus(signal),
    staleTime: 60_000,
    retry: 0,
  })
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [mfaRequired, setMfaRequired] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [correlationId, setCorrelationId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError(null)
    setCorrelationId(null)
    if (!username.trim() || !password) {
      setError('Enter your username and password.')
      return
    }
    if (mfaRequired && !totp.trim()) {
      setError('Enter your authenticator code.')
      return
    }

    setSubmitting(true)
    try {
      const code = totp.trim()
      if (code) {
        await login(username.trim(), password, code)
      } else {
        await login(username.trim(), password)
      }
      navigate('/', { replace: true })
    } catch (caught) {
      if (caught instanceof ApiError) {
        setCorrelationId(caught.correlationId ?? null)
        if (caught.code === 'AUTH_MFA_REQUIRED') {
          setMfaRequired(true)
          setError(null)
          return
        }
        if (caught.code === 'AUTH_MFA_INVALID') {
          setMfaRequired(true)
        }
      }
      setError(userFacingError(caught))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      eyebrow="Operator sign in"
      title="Return to the control surface"
      description="Use the administrator account created during first-run setup. Sessions are server-side and expire automatically."
      footer={
        <span>
          Setting up Drovik for the first time?{' '}
          <Link
            to="/setup"
            className="text-signal-300 hover:text-signal-200 underline underline-offset-2"
          >
            Create the administrator
          </Link>
        </span>
      }
    >
      <form className="space-y-5" onSubmit={handleSubmit} noValidate>
        {oidcError ? (
          <Alert variant="error" title="Single sign-on failed">
            {oidcErrors[oidcError] ?? oidcErrors.failed}
          </Alert>
        ) : null}
        {error ? (
          <Alert variant="error" title="Sign in failed">
            {error}
            {correlationId ? (
              <span className="mt-1 block font-mono text-[10px] opacity-70">
                Reference: {correlationId}
              </span>
            ) : null}
          </Alert>
        ) : null}

        <Field label="Username" htmlFor="login-username">
          <div className="relative">
            <UserRound
              className="text-ink-500 pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              aria-hidden="true"
            />
            <Input
              id="login-username"
              name="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              placeholder="admin"
              className="pl-9"
            />
          </div>
        </Field>

        <Field label="Password" htmlFor="login-password">
          <div className="relative">
            <LockKeyhole
              className="text-ink-500 pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              aria-hidden="true"
            />
            <Input
              id="login-password"
              name="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              placeholder="Enter your password"
              className="pl-9"
            />
          </div>
        </Field>

        {mfaRequired ? (
          <Field label="Authenticator code" htmlFor="login-totp" hint="Six digits, or a recovery code.">
            <Input
              id="login-totp"
              name="totp"
              value={totp}
              onChange={(event) => setTotp(event.target.value)}
              autoComplete="one-time-code"
              inputMode="numeric"
              placeholder="123456"
              className="font-mono"
            />
          </Field>
        ) : null}

        <Button type="submit" className="w-full" size="lg" loading={submitting}>
          Sign in
        </Button>

        {setupQuery.data?.oidc_enabled ? (
          <Button asChild variant="outline" className="w-full" size="lg">
            <a href={oidcStartURL()}>
              <ShieldCheck className="h-4 w-4" aria-hidden="true" />
              Sign in with single sign-on
            </a>
          </Button>
        ) : null}

        <p className="text-ink-500 text-center text-[11px] leading-5">
          Drovik never sends your password to the identity provider. Single sign-on links to a
          pre-existing local account.
        </p>
      </form>
    </AuthLayout>
  )
}
