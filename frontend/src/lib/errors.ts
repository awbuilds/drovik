const SENSITIVE_KEYS = new Set([
  'password',
  'passphrase',
  'token',
  'csrf_token',
  'authorization',
  'cookie',
  'private_key',
  'secret',
])

export function sanitizeContext(context: Record<string, unknown>): Record<string, unknown> {
  const clean: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(context)) {
    if (SENSITIVE_KEYS.has(key.toLowerCase())) {
      clean[key] = '[REDACTED]'
    } else if (typeof value === 'string' && value.length > 300) {
      clean[key] = `${value.slice(0, 300)}…`
    } else {
      clean[key] = value
    }
  }
  return clean
}

export function reportClientError(error: unknown, context: Record<string, unknown> = {}) {
  const message =
    error instanceof Error ? error.message : typeof error === 'string' ? error : 'Unknown error'
  // Client logs are intentionally sanitized and local. Drovik sends no telemetry.
  console.error('[drovik]', message, sanitizeContext(context))
}

export function userFacingError(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message
  }
  return 'Something went wrong. Try again.'
}
