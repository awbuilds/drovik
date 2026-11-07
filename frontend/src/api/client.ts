import type { ApiErrorBody, AuthResponse, SetupStatus, VersionInfo } from './types'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

export class ApiError extends Error {
  readonly code: string
  readonly status: number
  readonly correlationId?: string

  constructor(code: string, status: number, message: string, correlationId?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.correlationId = correlationId
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  csrf?: string
  signal?: AbortSignal
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = options.method ?? 'GET'
  const headers = new Headers({ Accept: 'application/json' })
  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
  }
  if (options.csrf) {
    headers.set('X-CSRF-Token', options.csrf)
  }

  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
      credentials: 'include',
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal: options.signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw error
    }
    throw new ApiError(
      'NETWORK_ERROR',
      0,
      'Unable to reach the Drovik server. Check that it is running and try again.',
    )
  }

  const contentType = response.headers.get('Content-Type') ?? ''
  const isJSON = contentType.includes('application/json')
  const payload: unknown = isJSON ? await response.json().catch(() => null) : await response.text()

  if (!response.ok) {
    const body = payload as ApiErrorBody | null
    const apiError = body?.error
    throw new ApiError(
      apiError?.code ?? 'INTERNAL_ERROR',
      response.status,
      apiError?.message ?? 'The request could not be completed.',
      apiError?.correlation_id,
    )
  }

  return payload as T
}

export function getSetupStatus(signal?: AbortSignal): Promise<SetupStatus> {
  return request<SetupStatus>('/api/v1/setup/status', { signal })
}

export function setup(username: string, password: string): Promise<AuthResponse> {
  return request<AuthResponse>('/api/v1/setup', {
    method: 'POST',
    body: { username, password },
  })
}

export function login(username: string, password: string): Promise<AuthResponse> {
  return request<AuthResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: { username, password },
  })
}

export function logout(csrf: string): Promise<void> {
  return request<void>('/api/v1/auth/logout', { method: 'POST', csrf })
}

export function me(signal?: AbortSignal): Promise<AuthResponse> {
  return request<AuthResponse>('/api/v1/auth/me', { signal })
}

export function getVersion(signal?: AbortSignal): Promise<VersionInfo> {
  return request<VersionInfo>('/api/v1/version', { signal })
}
