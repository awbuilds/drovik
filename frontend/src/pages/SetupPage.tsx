import { useMemo, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { KeyRound, LockKeyhole, ShieldCheck, UserRound } from 'lucide-react'
import { ApiError } from '@/api/client'
import { useAuth } from '@/auth/useAuth'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { AuthLayout } from '@/layout/AuthLayout'
import { cn } from '@/lib/cn'
import { userFacingError } from '@/lib/errors'
import { validateConfirmation, validatePassword, validateUsername } from '@/lib/validation'

interface FormErrors {
  username?: string | null
  password?: string | null
  confirmPassword?: string | null
}

function passwordScore(password: string) {
  let score = 0
  if (password.length >= 12) score += 1
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score += 1
  if (/\d/.test(password)) score += 1
  if (/[^A-Za-z0-9]/.test(password)) score += 1
  return score
}

export function SetupPage() {
  const { setup } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [errors, setErrors] = useState<FormErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [correlationId, setCorrelationId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const score = useMemo(() => passwordScore(password), [password])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError(null)
    setCorrelationId(null)

    const nextErrors: FormErrors = {
      username: validateUsername(username),
      password: validatePassword(password),
      confirmPassword: validateConfirmation(password, confirmPassword),
    }
    setErrors(nextErrors)
    if (Object.values(nextErrors).some(Boolean)) {
      return
    }

    setSubmitting(true)
    try {
      await setup(username.trim(), password)
      navigate('/', { replace: true })
    } catch (caught) {
      if (caught instanceof ApiError) {
        setCorrelationId(caught.correlationId ?? null)
      }
      setFormError(userFacingError(caught))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      eyebrow="First run"
      title="Create your administrator"
      description="This account controls this Drovik installation. It is stored locally and never sent to a third party."
      footer={
        <span>
          Already configured on another device?{' '}
          <Link
            to="/login"
            className="text-signal-300 hover:text-signal-200 underline underline-offset-2"
          >
            Sign in
          </Link>
        </span>
      }
    >
      <form className="space-y-5" onSubmit={handleSubmit} noValidate>
        {formError ? (
          <Alert variant="error" title="Setup could not be completed">
            {formError}
            {correlationId ? (
              <span className="mt-1 block font-mono text-[10px] opacity-70">
                Reference: {correlationId}
              </span>
            ) : null}
          </Alert>
        ) : null}

        <Field label="Username" htmlFor="username" error={errors.username}>
          <div className="relative">
            <UserRound
              className="text-ink-500 pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              aria-hidden="true"
            />
            <Input
              id="username"
              name="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              placeholder="admin"
              className="pl-9"
              invalid={Boolean(errors.username)}
              aria-describedby={errors.username ? 'username-error' : undefined}
            />
          </div>
        </Field>

        <Field
          label="Password"
          htmlFor="password"
          hint="At least 12 characters. A long, unique passphrase is best."
          error={errors.password}
        >
          <div className="relative">
            <LockKeyhole
              className="text-ink-500 pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              aria-hidden="true"
            />
            <Input
              id="password"
              name="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
              placeholder="At least 12 characters"
              className="pl-9"
              invalid={Boolean(errors.password)}
              aria-describedby={errors.password ? 'password-error' : undefined}
            />
          </div>
          {password.length > 0 ? (
            <div className="mt-2 flex items-center gap-2" aria-hidden="true">
              {[1, 2, 3, 4].map((level) => (
                <span
                  key={level}
                  className={cn(
                    'h-1 flex-1 rounded-full transition-colors',
                    score >= level ? 'bg-signal-400' : 'bg-ink-700',
                  )}
                />
              ))}
              <span className="text-ink-500 ml-1 w-14 font-mono text-[10px] uppercase">
                {score <= 1 ? 'weak' : score === 2 ? 'fair' : score === 3 ? 'good' : 'strong'}
              </span>
            </div>
          ) : null}
        </Field>

        <Field label="Confirm password" htmlFor="confirm-password" error={errors.confirmPassword}>
          <div className="relative">
            <KeyRound
              className="text-ink-500 pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              aria-hidden="true"
            />
            <Input
              id="confirm-password"
              name="confirm-password"
              type="password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              autoComplete="new-password"
              placeholder="Repeat the passphrase"
              className="pl-9"
              invalid={Boolean(errors.confirmPassword)}
              aria-describedby={errors.confirmPassword ? 'confirm-password-error' : undefined}
            />
          </div>
        </Field>

        <div className="border-ink-700 bg-ink-900/60 flex gap-3 rounded-md border p-3">
          <ShieldCheck className="text-mint-400 mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <p className="text-ink-400 text-xs leading-5">
            Passwords are hashed with Argon2id. Drovik stores only a hash and never logs your
            credentials.
          </p>
        </div>

        <Button type="submit" className="w-full" size="lg" loading={submitting}>
          Create administrator
        </Button>
      </form>
    </AuthLayout>
  )
}
