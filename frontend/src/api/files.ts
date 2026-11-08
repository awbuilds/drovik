import { ApiError, apiRequest } from './client'

export type FileType = 'file' | 'directory' | 'symlink' | 'other'

export interface FileEntry {
  name: string
  path: string
  type: FileType
  size: number
  mode: string
  modified_at: string
  uid?: number
  gid?: number
}

export interface FileListing {
  path: string
  entries: FileEntry[]
  truncated: boolean
}

export interface FileContent {
  path: string
  content: string
  size: number
  mode: string
  modified_at: string
}

export function listFiles(
  serverId: string,
  path: string,
  signal?: AbortSignal,
): Promise<FileListing> {
  const query = new URLSearchParams()
  if (path) query.set('path', path)
  return apiRequest<FileListing>(`/api/v1/servers/${serverId}/files?${query.toString()}`, { signal })
}

export function statFile(serverId: string, path: string): Promise<FileEntry> {
  return apiRequest<FileEntry>(
    `/api/v1/servers/${serverId}/files/stat?path=${encodeURIComponent(path)}`,
  )
}

export function readFile(serverId: string, path: string): Promise<FileContent> {
  return apiRequest<FileContent>(
    `/api/v1/servers/${serverId}/files/content?path=${encodeURIComponent(path)}`,
  )
}

export function writeFile(
  serverId: string,
  path: string,
  content: string,
  expected: { modifiedAt?: string; size?: number },
  csrf: string,
): Promise<void> {
  return apiRequest<void>(`/api/v1/servers/${serverId}/files/content`, {
    method: 'PUT',
    csrf,
    body: {
      path,
      content,
      expected_modified_at: expected.modifiedAt,
      expected_size: expected.size,
    },
  })
}

export function mkdir(serverId: string, path: string, csrf: string): Promise<{ path: string }> {
  return apiRequest<{ path: string }>(`/api/v1/servers/${serverId}/files/mkdir`, {
    method: 'POST',
    csrf,
    body: { path },
  })
}

export function touch(serverId: string, path: string, csrf: string): Promise<{ path: string }> {
  return apiRequest<{ path: string }>(`/api/v1/servers/${serverId}/files/touch`, {
    method: 'POST',
    csrf,
    body: { path },
  })
}

export function renamePath(
  serverId: string,
  from: string,
  to: string,
  overwrite: boolean,
  csrf: string,
): Promise<void> {
  return apiRequest<void>(`/api/v1/servers/${serverId}/files/rename`, {
    method: 'PATCH',
    csrf,
    body: { from, to, overwrite },
  })
}

export function deletePath(
  serverId: string,
  path: string,
  recursive: boolean,
  csrf: string,
): Promise<void> {
  const query = new URLSearchParams({ path, recursive: String(recursive) })
  return apiRequest<void>(`/api/v1/servers/${serverId}/files?${query.toString()}`, {
    method: 'DELETE',
    csrf,
  })
}

export function downloadFileURL(serverId: string, path: string): string {
  return `/api/v1/servers/${serverId}/files/download?path=${encodeURIComponent(path)}`
}

export function uploadFile(
  serverId: string,
  path: string,
  file: File,
  overwrite: boolean,
  csrf: string,
  onProgress?: (loaded: number, total: number) => void,
  signal?: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    try {
      const query = new URLSearchParams({ path, overwrite: String(overwrite) })
      const request = new XMLHttpRequest()
      request.open('POST', `/api/v1/servers/${serverId}/files/upload?${query.toString()}`)
      request.withCredentials = true
      request.setRequestHeader('X-CSRF-Token', csrf)
      request.upload.onprogress = (event) => {
        if (event.lengthComputable) onProgress?.(event.loaded, event.total)
      }
      request.onerror = () => reject(new Error('The upload failed.'))
      request.onabort = () => reject(new DOMException('Upload cancelled', 'AbortError'))
      request.onload = () => {
        if (request.status >= 200 && request.status < 300) {
          resolve()
          return
        }
        try {
          const body = JSON.parse(request.responseText) as {
            error?: {
              code?: string
              message?: string
              correlation_id?: string
              details?: unknown
            }
          }
          reject(
            new ApiError(
              body.error?.code ?? 'UPLOAD_FAILED',
              request.status,
              body.error?.message ?? 'The upload failed.',
              body.error?.correlation_id,
              body.error?.details,
            ),
          )
        } catch (error) {
          if (error instanceof ApiError) {
            reject(error)
          } else {
            reject(new Error('The upload failed.'))
          }
        }
      }
      if (signal) {
        signal.addEventListener('abort', () => request.abort(), { once: true })
      }
      const form = new FormData()
      form.append('file', file, file.name)
      request.send(form)
    } catch (error) {
      reject(error)
    }
  })
}
