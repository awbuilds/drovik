import { FolderTree, Pencil, Play, Server as ServerIcon, SquareTerminal, Trash2 } from 'lucide-react'
import type { Capabilities, Server } from '@/api/servers'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/Card'
import { StatusDot } from '@/components/ui/StatusDot'
import { Tooltip } from '@/components/ui/Tooltip'
import { formatRelativeTime } from '@/lib/format'

interface ServerCardProps {
  server: Server
  capabilities?: Capabilities
  testing: boolean
  onTest: () => void
  onTerminal: () => void
  onFiles: () => void
  onEdit: () => void
  onDelete: () => void
}

export function ServerCard({
  server,
  capabilities,
  testing,
  onTest,
  onTerminal,
  onFiles,
  onEdit,
  onDelete,
}: ServerCardProps) {
  const tags = server.tags
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)

  return (
    <Card className="flex flex-col">
      <CardHeader className="flex-row items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="bg-ink-800 text-signal-300 mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg">
            <ServerIcon className="h-4 w-4" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <CardTitle className="truncate">{server.display_name}</CardTitle>
            <p className="text-ink-400 mt-1 truncate font-mono text-xs">
              {server.username}@{server.host}:{server.port}
            </p>
          </div>
        </div>
        <Badge variant={server.auth_method === 'private_key' ? 'mint' : 'neutral'}>
          {server.auth_method === 'private_key' ? 'key' : 'password'}
        </Badge>
      </CardHeader>

      <CardContent className="flex-1 space-y-4 pt-4">
        <div className="flex flex-wrap items-center gap-2">
          <StatusDot
            status={server.last_connected_at ? 'online' : 'pending'}
            label={
              server.last_connected_at
                ? `Last verified ${formatRelativeTime(server.last_connected_at)}`
                : 'Not verified yet'
            }
          />
        </div>

        {capabilities ? (
          <div className="flex flex-wrap gap-1.5">
            <Badge variant="outline">
              {capabilities.distribution || capabilities.os || 'Linux'}
            </Badge>
            {capabilities.systemd ? <Badge variant="mint">systemd</Badge> : null}
            {capabilities.journalctl ? <Badge variant="mint">journalctl</Badge> : null}
            {capabilities.sftp ? (
              <Badge variant="mint">SFTP</Badge>
            ) : (
              <Badge variant="rose">no SFTP</Badge>
            )}
          </div>
        ) : (
          <p className="text-ink-500 text-xs">
            Run a connection test to detect the host's capabilities and verify its key.
          </p>
        )}

        {tags.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {tags.slice(0, 4).map((tag) => (
              <span key={tag} className="text-ink-500 font-mono text-[10px] uppercase">
                #{tag}
              </span>
            ))}
          </div>
        ) : null}
      </CardContent>

      <CardFooter className="justify-between">
        <Button size="sm" variant="outline" onClick={onTest} loading={testing}>
          <Play className="h-3.5 w-3.5" aria-hidden="true" />
          Test connection
        </Button>
        <div className="flex items-center gap-1">
          <Tooltip label="Open terminal">
            <Button
              variant="ghost"
              size="icon"
              onClick={onTerminal}
              aria-label={`Open terminal for ${server.display_name}`}
            >
              <SquareTerminal className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </Tooltip>
          <Tooltip label="Browse files">
            <Button
              variant="ghost"
              size="icon"
              onClick={onFiles}
              aria-label={`Browse files on ${server.display_name}`}
            >
              <FolderTree className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </Tooltip>
          <Tooltip label="Edit server">
            <Button
              variant="ghost"
              size="icon"
              onClick={onEdit}
              aria-label={`Edit ${server.display_name}`}
            >
              <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </Tooltip>
          <Tooltip label="Delete server">
            <Button
              variant="ghost"
              size="icon"
              onClick={onDelete}
              aria-label={`Delete ${server.display_name}`}
              className="hover:text-rose-400"
            >
              <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </Tooltip>
        </div>
      </CardFooter>
    </Card>
  )
}
