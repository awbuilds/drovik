import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { LoginPage } from './LoginPage'

const login = vi.hoisted(() => vi.fn())

vi.mock('@/auth/useAuth', () => ({
  useAuth: () => ({ login }),
}))

vi.mock('@/api/client', () => ({
  ApiError: class ApiError extends Error {},
  getSetupStatus: vi.fn().mockResolvedValue({ required: false, oidc_enabled: true }),
  oidcStartURL: () => '/api/v1/auth/oidc/start',
}))

function renderLogin() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: 0 } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('LoginPage', () => {
  it('validates empty input before calling the API', async () => {
    const user = userEvent.setup()
    renderLogin()

    await user.click(screen.getByRole('button', { name: /^sign in$/i }))
    expect(await screen.findByText(/enter your username and password/i)).toBeInTheDocument()
    expect(login).not.toHaveBeenCalled()
  })

  it('shows the single sign-on entry point and explains a denied link', async () => {
    renderLogin()
    expect(await screen.findByRole('link', { name: /sign in with single sign-on/i })).toHaveAttribute(
      'href',
      '/api/v1/auth/oidc/start',
    )
  })

  it('explains when no local account is linked', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/login?oidc_error=no_account']}>
          <LoginPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(await screen.findByText(/no local drovik account is linked/i)).toBeInTheDocument()
  })

  it('submits the credentials to the auth provider', async () => {
    const user = userEvent.setup()
    login.mockResolvedValueOnce(undefined)
    renderLogin()

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: /^sign in$/i }))

    await waitFor(() => {
      expect(login).toHaveBeenCalledWith('admin', 'correct horse battery staple')
    })
  })
})
