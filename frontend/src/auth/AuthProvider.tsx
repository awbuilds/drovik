import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import {
  ApiError,
  getSetupStatus,
  login as apiLogin,
  logout as apiLogout,
  me,
  setup as apiSetup,
} from '@/api/client'
import type { User } from '@/api/types'
import { reportClientError, userFacingError } from '@/lib/errors'
import { AuthContext, type AuthContextValue, type AuthStatus } from './context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading')
  const [user, setUser] = useState<User | null>(null)
  const [csrfToken, setCSRFToken] = useState('')
  const [error, setError] = useState<string | null>(null)

  const bootstrap = useCallback(async (signal?: AbortSignal) => {
    try {
      const setupStatus = await getSetupStatus(signal)
      if (setupStatus.required) {
        setUser(null)
        setCSRFToken('')
        setStatus('setup-required')
        return
      }
      const response = await me(signal)
      setUser(response.user)
      setCSRFToken(response.csrf_token)
      setStatus('authenticated')
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') {
        return
      }
      if (caught instanceof ApiError && caught.status === 401) {
        setUser(null)
        setCSRFToken('')
        setStatus('unauthenticated')
        return
      }
      reportClientError(caught, { operation: 'auth.bootstrap' })
      setError(userFacingError(caught))
      setStatus('error')
    }
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    // oxlint-disable-next-line react/set-state-in-effect -- auth is resolved asynchronously from the server
    void bootstrap(controller.signal)
    return () => controller.abort()
  }, [bootstrap])

  const setup = useCallback(async (username: string, password: string) => {
    const response = await apiSetup(username, password)
    setUser(response.user)
    setCSRFToken(response.csrf_token)
    setStatus('authenticated')
    setError(null)
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const response = await apiLogin(username, password)
    setUser(response.user)
    setCSRFToken(response.csrf_token)
    setStatus('authenticated')
    setError(null)
  }, [])

  const logout = useCallback(async () => {
    try {
      if (csrfToken) {
        await apiLogout(csrfToken)
      }
    } catch (caught) {
      // The local session is cleared regardless; the server-side session expires
      // on its own if the revocation request could not be delivered.
      reportClientError(caught, { operation: 'auth.logout' })
    } finally {
      setUser(null)
      setCSRFToken('')
      setStatus('unauthenticated')
    }
  }, [csrfToken])

  const refresh = useCallback(async () => {
    setError(null)
    setStatus('loading')
    await bootstrap()
  }, [bootstrap])

  const value = useMemo<AuthContextValue>(
    () => ({ status, user, csrfToken, error, setup, login, logout, refresh }),
    [status, user, csrfToken, error, setup, login, logout, refresh],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
