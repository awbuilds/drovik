export type Role = 'admin' | 'operator' | 'read_only'

export interface User {
  id: string
  username: string
  role: Role
  created_at: string
  last_login_at?: string | null
}

export interface AuthResponse {
  user: User
  csrf_token: string
}

export interface SetupStatus {
  required: boolean
}

export interface VersionInfo {
  version: string
  commit: string
  build_date: string
  go_version: string
  schema_version: number
}

export interface ApiErrorBody {
  error: {
    code: string
    message: string
    correlation_id?: string
    details?: unknown
  }
}
