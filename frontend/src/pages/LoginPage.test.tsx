import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { LoginPage } from './LoginPage'

const login = vi.hoisted(() => vi.fn())

vi.mock('@/auth/useAuth', () => ({
  useAuth: () => ({ login }),
}))

describe('LoginPage', () => {
  it('validates empty input before calling the API', async () => {
    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    )

    await user.click(screen.getByRole('button', { name: /^sign in$/i }))
    expect(await screen.findByText(/enter your username and password/i)).toBeInTheDocument()
    expect(login).not.toHaveBeenCalled()
  })

  it('submits the credentials to the auth provider', async () => {
    const user = userEvent.setup()
    login.mockResolvedValueOnce(undefined)
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    )

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: /^sign in$/i }))

    await waitFor(() => {
      expect(login).toHaveBeenCalledWith('admin', 'correct horse battery staple')
    })
  })
})
