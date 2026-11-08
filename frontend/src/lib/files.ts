import type { FileEntry } from '@/api/files'

export type SortKey = 'name' | 'size' | 'modified' | 'type'
export type SortDirection = 'asc' | 'desc'

export function joinRemotePath(base: string, name: string): string {
  const cleanBase = base.replace(/\/+$/, '')
  const cleanName = name.replace(/^\/+/, '')
  if (!cleanName) return cleanBase || '/'
  return `${cleanBase}/${cleanName}`
}

export function parentRemotePath(path: string): string {
  if (path === '/' || path === '') return '/'
  const parts = path.replace(/\/+$/, '').split('/')
  parts.pop()
  return parts.join('/') || '/'
}

export function baseName(path: string): string {
  const parts = path.replace(/\/+$/, '').split('/')
  return parts[parts.length - 1] || '/'
}

export function breadcrumbs(path: string): Array<{ label: string; path: string }> {
  if (path === '/') return [{ label: '/', path: '/' }]
  const parts = path.split('/').filter(Boolean)
  const result: Array<{ label: string; path: string }> = [{ label: '/', path: '/' }]
  let current = ''
  for (const part of parts) {
    current += `/${part}`
    result.push({ label: part, path: current })
  }
  return result
}

export function sortEntries(
  entries: FileEntry[],
  key: SortKey,
  direction: SortDirection,
): FileEntry[] {
  const factor = direction === 'asc' ? 1 : -1
  return [...entries].sort((left, right) => {
    if (left.type === 'directory' && right.type !== 'directory') return -1
    if (right.type === 'directory' && left.type !== 'directory') return 1
    switch (key) {
      case 'size':
        return (left.size - right.size) * factor
      case 'modified':
        return (new Date(left.modified_at).getTime() - new Date(right.modified_at).getTime()) * factor
      case 'type':
        return left.type.localeCompare(right.type) * factor
      default:
        return left.name.localeCompare(right.name, undefined, { sensitivity: 'base' }) * factor
    }
  })
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes / 1024
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[index]}`
}

export function isHidden(name: string): boolean {
  return name.startsWith('.')
}
