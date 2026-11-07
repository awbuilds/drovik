import { describe, expect, it } from 'vitest'
import { validateConfirmation, validatePassword, validateUsername } from './validation'

describe('validateUsername', () => {
  it('accepts a normal operator name', () => {
    expect(validateUsername('admin')).toBeNull()
    expect(validateUsername('ops.team-1')).toBeNull()
  })

  it('rejects empty, short and unsafe names', () => {
    expect(validateUsername('')).not.toBeNull()
    expect(validateUsername('ab')).not.toBeNull()
    expect(validateUsername('-admin')).not.toBeNull()
    expect(validateUsername('admin space')).not.toBeNull()
    expect(validateUsername('admin@example.com')).not.toBeNull()
  })
})

describe('validatePassword', () => {
  it('enforces the minimum length', () => {
    expect(validatePassword('short')).not.toBeNull()
    expect(validatePassword('correct horse battery staple')).toBeNull()
  })

  it('rejects whitespace-only passwords', () => {
    expect(validatePassword('            ')).not.toBeNull()
  })
})

describe('validateConfirmation', () => {
  it('requires a matching confirmation', () => {
    expect(validateConfirmation('password', 'password')).toBeNull()
    expect(validateConfirmation('password', 'different')).not.toBeNull()
    expect(validateConfirmation('password', '')).not.toBeNull()
  })
})
