import { apiRequest } from './client'

export type UserRole = 'admin' | 'operator' | 'read_only'

export interface ManagedUser {
  id: string
  username: string
  role: UserRole
  is_active: boolean
  created_at: string
  last_login_at?: string
}

export interface CreateUserInput {
  username: string
  password: string
  role: UserRole
}

export function listUsers(signal?: AbortSignal): Promise<{ users: ManagedUser[] }> {
  return apiRequest<{ users: ManagedUser[] }>('/api/v1/users', { signal })
}

export function createUser(input: CreateUserInput, csrf: string): Promise<ManagedUser> {
  return apiRequest<ManagedUser>('/api/v1/users', { method: 'POST', body: input, csrf })
}

export function updateUser(
  id: string,
  input: { role?: UserRole; is_active?: boolean },
  csrf: string,
): Promise<ManagedUser> {
  return apiRequest<ManagedUser>(`/api/v1/users/${id}`, { method: 'PATCH', body: input, csrf })
}

export function resetUserPassword(id: string, password: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/users/${id}/password`, {
    method: 'POST',
    body: { password },
    csrf,
  })
}

export function deleteUser(id: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/users/${id}`, { method: 'DELETE', csrf })
}
