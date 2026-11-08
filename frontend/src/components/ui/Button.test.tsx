import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Button } from './Button'

describe('Button', () => {
  it('slots a single child correctly with asChild', () => {
    render(
      <Button asChild>
        <a href="/servers">Manage servers</a>
      </Button>,
    )
    expect(screen.getByRole('link', { name: /manage servers/i })).toBeInTheDocument()
  })

  it('renders a loading state without dropping the label', () => {
    render(<Button loading>Save</Button>)
    expect(screen.getByRole('button', { name: /save/i })).toBeInTheDocument()
  })
})
