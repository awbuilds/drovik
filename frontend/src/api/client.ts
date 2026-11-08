import type { ApiErrorBody, AuthResponse, SetupStatus, VersionInfo } from './types'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

export function apiURL(path: string): string {
  return `${API_BASE}${path}`
}

export class ApiError extends Error {
  readonly code: string
  readonly status: number
  readonly correlationId?: string
  readonly details?: unknown

  constructor(
    code: string,
    status: number,
    message: string,
    correlationId?: string,
    details?: unknown,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.correlationId = correlationId
    this.details = details
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  csrf?: string
  signal?: AbortSignal
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
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
      apiError?.details,
    )
  }

  return payload as T
}

export function getSetupStatus(signal?: AbortSignal): Promise<SetupStatus> {
  return apiRequest<SetupStatus>('/api/v1/setup/status', { signal })
}

export function oidcStartURL(): string {
  return `${API_BASE}/api/v1/auth/oidc/start`
}

export function setup(username: string, password: string): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/api/v1/setup', {
    method: 'POST',
    body: { username, password },
  })
}

export function login(username: string, password: string, totp?: string): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: { username, password, ...(totp ? { totp } : {}) },
  })
}

export interface MFAStatus {
  enabled: boolean
}

export function getMFAStatus(signal?: AbortSignal): Promise<MFAStatus> {
  return apiRequest<MFAStatus>('/api/v1/auth/mfa', { signal })
}

export function setupMFA(
  input: { password: string; code?: string },
  csrf: string,
): Promise<{ secret: string; uri: string }> {
  return apiRequest<{ secret: string; uri: string }>('/api/v1/auth/mfa/setup', {
    method: 'POST',
    csrf,
    body: input,
  })
}

export function enableMFA(code: string, csrf: string): Promise<{ recovery_codes: string[] }> {
  return apiRequest<{ recovery_codes: string[] }>('/api/v1/auth/mfa/enable', {
    method: 'POST',
    csrf,
    body: { code },
  })
}

export interface OIDCLinkStatus {
  linked: boolean
}

export function getOIDCLinkStatus(signal?: AbortSignal): Promise<OIDCLinkStatus> {
  return apiRequest<OIDCLinkStatus>('/api/v1/auth/oidc/link', { signal })
}

export function startOIDCLink(
  input: { password: string; code?: string },
  csrf: string,
): Promise<{ url: string }> {
  return apiRequest<{ url: string }>('/api/v1/auth/oidc/link', {
    method: 'POST',
    csrf,
    body: input,
  })
}

export function unlinkOIDC(password: string, csrf: string): Promise<void> {
  return apiRequest<void>('/api/v1/auth/oidc/unlink', {
    method: 'POST',
    csrf,
    body: { password },
  })
}

export function disableMFA(password: string, csrf: string): Promise<void> {
  return apiRequest<void>('/api/v1/auth/mfa/disable', {
    method: 'POST',
    csrf,
    body: { password },
  })
}

export function logout(csrf: string): Promise<void> {
  return apiRequest<void>('/api/v1/auth/logout', { method: 'POST', csrf })
}

export function me(signal?: AbortSignal): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/api/v1/auth/me', { signal })
}

export function getVersion(signal?: AbortSignal): Promise<VersionInfo> {
  return apiRequest<VersionInfo>('/api/v1/version', { signal })
}


export async function apiDownload(path: string, signal?: AbortSignal): Promise<Blob> {
  let response: Response
  try {
    response = await fetch(apiURL(path), { credentials: 'include', signal })
  } catch (caught) {
    throw new ApiError('NETWORK_ERROR', 0, 'The download could not be started.', undefined, caught)
  }
  if (!response.ok) {
    let code = 'INTERNAL_ERROR'
    let message = 'The download failed.'
    try {
      const body = (await response.json()) as ApiErrorBody
      code = body.error?.code ?? code
      message = body.error?.message ?? message
    } catch {
      // Keep the generic message when the body is not JSON.
    }
    throw new ApiError(code, response.status, message)
  }
  return response.blob()
}
