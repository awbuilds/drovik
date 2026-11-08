import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  ChevronRight,
  Download,
  Eye,
  EyeOff,
  File as FileIcon,
  FileQuestion,
  FileText,
  Folder,
  FolderPlus,
  Link2,
  Pencil,
  Plus,
  RefreshCw,
  Server as ServerIcon,
  Trash2,
  Upload,
} from 'lucide-react'
import { ApiError } from '@/api/client'
import {
  deletePath,
  downloadFileURL,
  listFiles,
  mkdir,
  renamePath,
  touch,
  uploadFile,
  type FileEntry,
} from '@/api/files'
import { listServers } from '@/api/servers'
import { useAuth } from '@/auth/useAuth'
import { PageHeader } from '@/components/PageHeader'
import { ConfirmDialog } from '@/components/servers/ConfirmDialog'
import { FileEditorDialog } from '@/components/files/FileEditorDialog'
import { DeleteFileDialog } from '@/components/files/DeleteFileDialog'
import { NameDialog } from '@/components/files/NameDialog'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import {
  baseName,
  breadcrumbs,
  formatBytes,
  isHidden,
  joinRemotePath,
  parentRemotePath,
  sortEntries,
  type SortDirection,
  type SortKey,
} from '@/lib/files'
import { formatDateTime } from '@/lib/format'

interface UploadState {
  name: string
  loaded: number
  total: number
}

interface ConflictState {
  file: File
  target: string
}

type NameAction =
  | { type: 'folder' }
  | { type: 'file' }
  | { type: 'rename'; entry: FileEntry }

export function FilesPage() {
  const { serverId } = useParams<{ serverId?: string }>()
  const navigate = useNavigate()
  const { csrfToken } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const uploadInputRef = useRef<HTMLInputElement | null>(null)

  const [path, setPath] = useState('')
  const [hidden, setHidden] = useState(false)
  const [filter, setFilter] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')
  const [upload, setUpload] = useState<UploadState | null>(null)
  const [conflict, setConflict] = useState<ConflictState | null>(null)
  const [nameAction, setNameAction] = useState<NameAction | null>(null)
  const [nameBusy, setNameBusy] = useState(false)
  const [nameError, setNameError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<FileEntry | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [editorPath, setEditorPath] = useState<string | null>(null)

  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const servers = serversQuery.data?.servers ?? []
  const currentServer = servers.find((server) => server.id === serverId)

  const filesQuery = useQuery({
    queryKey: ['files', serverId, path],
    queryFn: ({ signal }) => listFiles(serverId ?? '', path, signal),
    enabled: Boolean(serverId),
  })

  useEffect(() => {
    // oxlint-disable-next-line react/set-state-in-effect -- reset the browser when the server changes
    setPath('')
    // oxlint-disable-next-line react/set-state-in-effect -- reset the browser when the server changes
    setFilter('')
    // oxlint-disable-next-line react/set-state-in-effect -- reset the browser when the server changes
    setEditorPath(null)
  }, [serverId])

  const currentPath = filesQuery.data?.path ?? path
  const entries = sortEntries(
    (filesQuery.data?.entries ?? [])
      .filter((entry) => hidden || !isHidden(entry.name))
      .filter((entry) => entry.name.toLowerCase().includes(filter.toLowerCase())),
    sortKey,
    sortDirection,
  )

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ['files', serverId, path] })
  }

  const uploadOne = async (file: File, overwrite: boolean) => {
    if (!serverId || !currentPath) return
    const target = joinRemotePath(currentPath, file.name)
    setUpload({ name: file.name, loaded: 0, total: file.size })
    try {
      await uploadFile(serverId, target, file, overwrite, csrfToken, (loaded, total) => {
        setUpload({ name: file.name, loaded, total })
      })
      toast({ title: 'Upload complete', description: file.name, variant: 'success' })
      await refresh()
    } catch (caught) {
      if (caught instanceof ApiError && caught.code === 'FILE_CONFLICT') {
        setConflict({ file, target })
        return
      }
      toast({ title: 'Upload failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setUpload(null)
    }
  }

  const handleUploadSelection = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    for (const file of files) {
      await uploadOne(file, false)
    }
  }

  const handleNameSubmit = async (name: string) => {
    if (!serverId || !nameAction) return
    setNameBusy(true)
    setNameError(null)
    try {
      if (nameAction.type === 'folder') {
        await mkdir(serverId, joinRemotePath(currentPath, name), csrfToken)
        toast({ title: 'Folder created', description: name, variant: 'success' })
      } else if (nameAction.type === 'file') {
        await touch(serverId, joinRemotePath(currentPath, name), csrfToken)
        toast({ title: 'File created', description: name, variant: 'success' })
      } else {
        await renamePath(
          serverId,
          nameAction.entry.path,
          joinRemotePath(parentRemotePath(nameAction.entry.path), name),
          false,
          csrfToken,
        )
        toast({ title: 'Renamed', description: name, variant: 'success' })
      }
      setNameAction(null)
      await refresh()
    } catch (caught) {
      setNameError(userFacingError(caught))
    } finally {
      setNameBusy(false)
    }
  }

  const handleDelete = async (recursive: boolean) => {
    if (!serverId || !deleteTarget) return
    setDeleting(true)
    try {
      await deletePath(serverId, deleteTarget.path, recursive, csrfToken)
      toast({ title: 'Deleted', description: deleteTarget.name, variant: 'success' })
      setDeleteTarget(null)
      await refresh()
    } catch (caught) {
      toast({ title: 'Delete failed', description: userFacingError(caught), variant: 'error' })
    } finally {
      setDeleting(false)
    }
  }

  const openEntry = (entry: FileEntry) => {
    if (entry.type === 'directory') {
      setPath(entry.path)
      return
    }
    if (entry.type === 'symlink') {
      toast({
        title: 'Symlink selected',
        description: 'Symlinks are not opened in the editor or followed by recursive actions.',
        variant: 'info',
      })
      return
    }
    setEditorPath(entry.path)
  }

  const download = (entry: FileEntry) => {
    if (!serverId) return
    try {
      const anchor = document.createElement('a')
      anchor.href = downloadFileURL(serverId, entry.path)
      anchor.download = entry.name
      anchor.click()
    } catch (caught) {
      toast({ title: 'Download failed', description: userFacingError(caught), variant: 'error' })
    }
  }

  if (serversQuery.isLoading) {
    return <Skeleton className="h-[60vh] w-full" />
  }

  if (serversQuery.isError) {
    return (
      <Alert variant="error" title="Servers could not be loaded">
        {userFacingError(serversQuery.error)}
      </Alert>
    )
  }

  if (!serverId) {
    return (
      <div className="space-y-5">
        <PageHeader
          eyebrow="Workspace"
          title="Files"
          description="Browse and edit files over SFTP. Select a server to open its filesystem as the connected SSH user."
        />
        {servers.length === 0 ? (
          <Card className="p-8 text-center">
            <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
              <ServerIcon className="h-6 w-6" aria-hidden="true" />
            </div>
            <h2 className="font-display text-ink-50 mt-4 text-xl font-semibold">Add a server first</h2>
            <p className="text-ink-400 mt-2 text-sm">
              SFTP uses the same saved SSH connection and host-key trust as the terminal.
            </p>
            <Button asChild className="mt-5">
              <Link to="/servers">
                <Plus className="h-4 w-4" aria-hidden="true" />
                Go to servers
              </Link>
            </Button>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {servers.map((server) => (
              <Card key={server.id}>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Folder className="text-signal-400 h-4 w-4" aria-hidden="true" />
                    {server.display_name}
                  </CardTitle>
                  <CardDescription>
                    {server.username}@{server.host}:{server.port}
                  </CardDescription>
                </CardHeader>
                <CardContent className="pt-4">
                  <Button variant="outline" className="w-full" onClick={() => navigate(`/files/${server.id}`)}>
                    Browse files
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    )
  }

  if (!currentServer) {
    return (
      <Alert variant="error" title="Server not found">
        This server does not exist or is not assigned to your account.
      </Alert>
    )
  }

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Workspace"
        title="Files"
        description={`SFTP as ${currentServer.username}@${currentServer.host}. Drovik never follows symlinks for recursive operations.`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Select
              aria-label="Select server"
              value={serverId}
              onChange={(event) => navigate(`/files/${event.target.value}`)}
              className="w-56"
            >
              {servers.map((server) => (
                <option key={server.id} value={server.id}>
                  {server.display_name} · {server.host}
                </option>
              ))}
            </Select>
            <Badge variant="mint">Phase 4</Badge>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => void refresh()} loading={filesQuery.isFetching}>
          <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
          Refresh
        </Button>
        <Button size="sm" onClick={() => uploadInputRef.current?.click()} disabled={Boolean(upload)}>
          <Upload className="h-3.5 w-3.5" aria-hidden="true" />
          Upload
        </Button>
        <input ref={uploadInputRef} type="file" multiple className="hidden" onChange={(event) => void handleUploadSelection(event)} />
        <Button size="sm" variant="outline" onClick={() => setNameAction({ type: 'folder' })}>
          <FolderPlus className="h-3.5 w-3.5" aria-hidden="true" />
          New folder
        </Button>
        <Button size="sm" variant="outline" onClick={() => setNameAction({ type: 'file' })}>
          <Plus className="h-3.5 w-3.5" aria-hidden="true" />
          New file
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => setHidden((value) => !value)}
          aria-pressed={hidden}
        >
          {hidden ? <EyeOff className="h-3.5 w-3.5" aria-hidden="true" /> : <Eye className="h-3.5 w-3.5" aria-hidden="true" />}
          {hidden ? 'Hide dotfiles' : 'Show dotfiles'}
        </Button>
      </div>

      {upload ? (
        <div className="border-ink-700 bg-ink-900 rounded-lg border p-3">
          <div className="flex items-center justify-between gap-3 text-xs">
            <span className="text-ink-200 truncate">Uploading {upload.name}</span>
            <span className="text-ink-500 font-mono">
              {upload.total > 0 ? `${Math.round((upload.loaded / upload.total) * 100)}%` : '…'}
            </span>
          </div>
          <div className="bg-ink-700 mt-2 h-1 overflow-hidden rounded-full">
            <div
              className="bg-signal-400 h-full transition-[width]"
              style={{ width: upload.total > 0 ? `${Math.min(100, (upload.loaded / upload.total) * 100)}%` : '10%' }}
            />
          </div>
        </div>
      ) : null}

      <Card>
        <div className="border-ink-700 flex flex-wrap items-center gap-2 border-b px-4 py-3">
          <nav aria-label="Breadcrumb" className="flex min-w-0 flex-1 flex-wrap items-center gap-1">
            {breadcrumbs(currentPath).map((crumb, index, list) => (
              <span key={crumb.path} className="flex items-center gap-1">
                <button
                  type="button"
                  className="text-ink-300 hover:text-signal-300 max-w-40 truncate rounded px-1 py-0.5 font-mono text-[11px]"
                  onClick={() => setPath(crumb.path)}
                >
                  {crumb.label}
                </button>
                {index < list.length - 1 ? (
                  <ChevronRight className="text-ink-600 h-3 w-3" aria-hidden="true" />
                ) : null}
              </span>
            ))}
          </nav>
          <div className="flex items-center gap-2">
            <Input
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
              placeholder="Filter this directory"
              className="h-8 w-44 text-xs"
              aria-label="Filter files"
            />
            <Select
              value={sortKey}
              onChange={(event) => setSortKey(event.target.value as SortKey)}
              className="h-8 w-28 text-xs"
              aria-label="Sort by"
            >
              <option value="name">Name</option>
              <option value="type">Type</option>
              <option value="size">Size</option>
              <option value="modified">Modified</option>
            </Select>
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8"
              onClick={() => setSortDirection((value) => (value === 'asc' ? 'desc' : 'asc'))}
              aria-label={`Sort ${sortDirection === 'asc' ? 'descending' : 'ascending'}`}
            >
              {sortDirection === 'asc' ? '↑' : '↓'}
            </Button>
          </div>
        </div>

        {filesQuery.isError ? (
          <div className="p-4">
            <Alert variant="error" title="Directory could not be listed">
              {userFacingError(filesQuery.error)}
            </Alert>
          </div>
        ) : filesQuery.isLoading ? (
          <div className="space-y-2 p-4">
            {[1, 2, 3, 4].map((item) => (
              <Skeleton key={item} className="h-10 w-full" />
            ))}
          </div>
        ) : entries.length === 0 ? (
          <div className="p-10 text-center">
            <FileQuestion className="text-ink-500 mx-auto h-8 w-8" aria-hidden="true" />
            <p className="text-ink-300 mt-3 text-sm">This directory is empty.</p>
            <p className="text-ink-500 mt-1 text-xs">Upload a file or create a folder to get started.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-ink-500 border-ink-700 border-b font-mono text-[10px] uppercase">
                <tr>
                  <th className="px-4 py-2 font-medium">Name</th>
                  <th className="hidden px-4 py-2 text-right font-medium sm:table-cell">Size</th>
                  <th className="hidden px-4 py-2 font-medium md:table-cell">Mode</th>
                  <th className="hidden px-4 py-2 font-medium lg:table-cell">Modified</th>
                  <th className="px-4 py-2 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.path} className="border-ink-800 hover:bg-ink-850/60 border-b last:border-0">
                    <td className="px-4 py-2.5">
                      <button
                        type="button"
                        onClick={() => openEntry(entry)}
                        className="focus-visible:ring-signal-400 flex max-w-full items-center gap-2 rounded text-left focus-visible:ring-2 focus-visible:outline-none"
                      >
                        {entry.type === 'directory' ? (
                          <Folder className="text-signal-400 h-4 w-4 shrink-0" aria-hidden="true" />
                        ) : entry.type === 'symlink' ? (
                          <Link2 className="text-mint-400 h-4 w-4 shrink-0" aria-hidden="true" />
                        ) : entry.type === 'file' ? (
                          <FileText className="text-ink-400 h-4 w-4 shrink-0" aria-hidden="true" />
                        ) : (
                          <FileIcon className="text-ink-500 h-4 w-4 shrink-0" aria-hidden="true" />
                        )}
                        <span className="text-ink-100 truncate">{entry.name}</span>
                        {entry.type === 'symlink' ? <Badge variant="outline">link</Badge> : null}
                      </button>
                    </td>
                    <td className="text-ink-400 hidden px-4 py-2.5 text-right font-mono text-[11px] sm:table-cell">
                      {entry.type === 'directory' ? '—' : formatBytes(entry.size)}
                    </td>
                    <td className="text-ink-500 hidden px-4 py-2.5 font-mono text-[11px] md:table-cell">
                      {entry.mode}
                    </td>
                    <td className="text-ink-500 hidden px-4 py-2.5 text-xs lg:table-cell">
                      {formatDateTime(entry.modified_at)}
                    </td>
                    <td className="px-4 py-2.5">
                      <div className="flex items-center justify-end gap-1">
                        {entry.type === 'file' ? (
                          <>
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7"
                              onClick={() => download(entry)}
                              aria-label={`Download ${entry.name}`}
                            >
                              <Download className="h-3.5 w-3.5" aria-hidden="true" />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7"
                              onClick={() => setEditorPath(entry.path)}
                              aria-label={`Edit ${entry.name}`}
                            >
                              <FileText className="h-3.5 w-3.5" aria-hidden="true" />
                            </Button>
                          </>
                        ) : null}
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7"
                          onClick={() => {
                            setNameError(null)
                            setNameAction({ type: 'rename', entry })
                          }}
                          aria-label={`Rename ${entry.name}`}
                        >
                          <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="hover:text-rose-400 h-7 w-7"
                          onClick={() => setDeleteTarget(entry)}
                          aria-label={`Delete ${entry.name}`}
                        >
                          <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {filesQuery.data?.truncated ? (
        <Alert variant="warning" title="Directory listing was truncated">
          This directory contains more entries than the configured listing limit. Use a more specific
          directory or the search filter.
        </Alert>
      ) : null}

      <div className="border-ink-700 bg-ink-900/60 flex gap-3 rounded-lg border p-4">
        <Folder className="text-mint-400 mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <p className="text-ink-400 text-xs leading-5">
          Transfers stream over SFTP. Editor content is limited to the configured size and is only
          written after Save. Symlinks are shown but never followed by recursive operations.
        </p>
      </div>

      <NameDialog
        open={nameAction !== null}
        onOpenChange={(open) => {
          if (!open) {
            setNameAction(null)
            setNameError(null)
          }
        }}
        title={
          nameAction?.type === 'folder'
            ? 'New folder'
            : nameAction?.type === 'file'
              ? 'New file'
              : 'Rename item'
        }
        description={
          nameAction?.type === 'rename'
            ? `Rename ${nameAction.entry.name} in ${parentRemotePath(nameAction.entry.path)}.`
            : `Create an item in ${currentPath}.`
        }
        label="Name"
        initialValue={nameAction?.type === 'rename' ? baseName(nameAction.entry.path) : ''}
        confirmLabel={nameAction?.type === 'rename' ? 'Rename' : 'Create'}
        busy={nameBusy}
        error={nameError}
        onSubmit={(name) => void handleNameSubmit(name)}
      />

      <DeleteFileDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        entry={deleteTarget}
        busy={deleting}
        onConfirm={(recursive) => void handleDelete(recursive)}
      />

      <ConfirmDialog
        open={conflict !== null}
        onOpenChange={(open) => {
          if (!open) setConflict(null)
        }}
        title="Overwrite existing file?"
        description={`${conflict?.target ?? 'The target'} already exists.`}
        confirmLabel="Overwrite"
        destructive
        onConfirm={() => {
          const pending = conflict
          setConflict(null)
          if (pending) void uploadOne(pending.file, true)
        }}
      />

      {editorPath && serverId ? (
        <FileEditorDialog
          open={editorPath !== null}
          onOpenChange={(open) => {
            if (!open) setEditorPath(null)
          }}
          serverId={serverId}
          path={editorPath}
          csrfToken={csrfToken}
          onSaved={() => void refresh()}
        />
      ) : null}
    </div>
  )
}
