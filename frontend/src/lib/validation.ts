export interface FieldError {
  field: 'username' | 'password' | 'confirmPassword'
  message: string
}

export function validateUsername(value: string): string | null {
  const username = value.trim()
  if (username.length === 0) {
    return 'Enter a username.'
  }
  if (username.length < 3 || username.length > 64) {
    return 'Username must be 3 to 64 characters.'
  }
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(username)) {
    return 'Use letters, numbers, dots, underscores or hyphens, starting with a letter or number.'
  }
  return null
}

export function validatePassword(value: string): string | null {
  if (value.length === 0) {
    return 'Enter a password.'
  }
  if (value.length < 12) {
    return 'Use at least 12 characters.'
  }
  if (value.length > 256) {
    return 'Use at most 256 characters.'
  }
  if (value.trim().length === 0) {
    return 'Password cannot be only whitespace.'
  }
  return null
}

export function validateConfirmation(password: string, confirmation: string): string | null {
  if (confirmation.length === 0) {
    return 'Confirm your password.'
  }
  if (password !== confirmation) {
    return 'Passwords do not match.'
  }
  return null
}
