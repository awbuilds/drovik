import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

function Broken(): never {
  throw new Error('render failed')
}

describe('ErrorBoundary', () => {
  it('renders a recoverable fallback instead of a white screen', () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <ErrorBoundary>
        <Broken />
      </ErrorBoundary>,
    )

    expect(screen.getByRole('heading', { name: /something went wrong/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /reload drovik/i })).toBeInTheDocument()
    consoleError.mockRestore()
  })
})
