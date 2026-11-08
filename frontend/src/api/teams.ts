import { apiRequest } from './client'

export interface Team {
  id: string
  name: string
  description: string
  member_count: number
  server_count: number
  created_at: string
  updated_at: string
}

export interface TeamMember {
  user_id: string
  username: string
  role: string
  created_at: string
}

export interface TeamServer {
  server_id: string
  display_name: string
  host: string
  access_level: 'read' | 'write'
  created_at: string
}

export interface TeamDetail {
  team: Team
  members: TeamMember[]
  servers: TeamServer[]
}

export function listTeams(signal?: AbortSignal): Promise<{ teams: Team[] }> {
  return apiRequest<{ teams: Team[] }>('/api/v1/teams', { signal })
}

export function getTeam(id: string, signal?: AbortSignal): Promise<TeamDetail> {
  return apiRequest<TeamDetail>(`/api/v1/teams/${id}`, { signal })
}

export function createTeam(
  input: { name: string; description: string },
  csrf: string,
): Promise<Team> {
  return apiRequest<Team>('/api/v1/teams', { method: 'POST', csrf, body: input })
}

export function updateTeam(
  id: string,
  input: { name: string; description: string },
  csrf: string,
): Promise<Team> {
  return apiRequest<Team>(`/api/v1/teams/${id}`, { method: 'PATCH', csrf, body: input })
}

export function deleteTeam(id: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/teams/${id}`, { method: 'DELETE', csrf })
}

export function addTeamMember(teamId: string, userId: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/teams/${teamId}/members/${userId}`, { method: 'PUT', csrf })
}

export function removeTeamMember(teamId: string, userId: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/teams/${teamId}/members/${userId}`, { method: 'DELETE', csrf })
}

export function setTeamServer(
  teamId: string,
  serverId: string,
  accessLevel: 'read' | 'write',
  csrf: string,
): Promise<void> {
  return apiRequest<void>(`/api/v1/teams/${teamId}/servers/${serverId}`, {
    method: 'PUT',
    csrf,
    body: { access_level: accessLevel },
  })
}

export function removeTeamServer(teamId: string, serverId: string, csrf: string): Promise<void> {
  return apiRequest<void>(`/api/v1/teams/${teamId}/servers/${serverId}`, {
    method: 'DELETE',
    csrf,
  })
}
