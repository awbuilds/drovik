import { createContext } from 'react'
import type { User } from '@/api/types'

export type AuthStatus =
  'loading' | 'setup-required' | 'authenticated' | 'unauthenticated' | 'error'

export interface AuthContextValue {
  status: AuthStatus
  user: User | null
  csrfToken: string
  error: string | null
  setup: (username: string, password: string) => Promise<void>
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)
