import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, login } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('api client', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends credentials and maps the auth response', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        user: {
          id: 'user-1',
          username: 'admin',
          role: 'admin',
          created_at: '2026-11-07T12:00:00Z',
        },
        csrf_token: 'csrf-token',
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const result = await login('admin', 'correct horse battery staple')
    expect(result.user.username).toBe('admin')
    expect(result.csrf_token).toBe('csrf-token')

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/auth/login')
    expect(init.credentials).toBe('include')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(
      JSON.stringify({ username: 'admin', password: 'correct horse battery staple' }),
    )
  })

  it('maps a structured API error', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(
          {
            error: {
              code: 'AUTH_INVALID_CREDENTIALS',
              message: 'Invalid username or password.',
              correlation_id: 'req-1',
            },
          },
          401,
        ),
      ),
    )

    await expect(login('admin', 'wrong')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'AUTH_INVALID_CREDENTIALS',
      status: 401,
      correlationId: 'req-1',
    } satisfies Partial<ApiError>)
  })

  it('maps a network failure without leaking details', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(login('admin', 'password')).rejects.toMatchObject({
      code: 'NETWORK_ERROR',
      status: 0,
    })
  })
})
