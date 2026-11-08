import { describe, expect, it } from 'vitest'
import {
  baseName,
  breadcrumbs,
  formatBytes,
  isHidden,
  joinRemotePath,
  parentRemotePath,
  sortEntries,
} from './files'
import type { FileEntry } from '@/api/files'

const entry = (name: string, type: FileEntry['type'], size: number): FileEntry => ({
  name,
  path: `/home/${name}`,
  type,
  size,
  mode: type === 'directory' ? 'drwxr-xr-x' : '-rw-r--r--',
  modified_at: '2026-11-08T00:00:00Z',
})

describe('file path helpers', () => {
  it('joins and parents remote paths', () => {
    expect(joinRemotePath('/home/ops', 'file.txt')).toBe('/home/ops/file.txt')
    expect(joinRemotePath('/', 'etc')).toBe('/etc')
    expect(parentRemotePath('/home/ops/file.txt')).toBe('/home/ops')
    expect(parentRemotePath('/')).toBe('/')
    expect(baseName('/home/ops/file.txt')).toBe('file.txt')
  })

  it('builds breadcrumbs', () => {
    expect(breadcrumbs('/home/ops')).toEqual([
      { label: '/', path: '/' },
      { label: 'home', path: '/home' },
      { label: 'ops', path: '/home/ops' },
    ])
  })

  it('sorts directories first and respects direction', () => {
    const sorted = sortEntries(
      [entry('b.txt', 'file', 20), entry('a', 'directory', 0), entry('a.txt', 'file', 10)],
      'name',
      'asc',
    )
    expect(sorted.map((item) => item.name)).toEqual(['a', 'a.txt', 'b.txt'])
  })

  it('formats bytes and detects hidden files', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2.0 KiB')
    expect(isHidden('.env')).toBe(true)
    expect(isHidden('notes.txt')).toBe(false)
  })
})
