import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Plus, Server as ServerIcon, SquareTerminal } from 'lucide-react'
import { listServers } from '@/api/servers'
import { PageHeader } from '@/components/PageHeader'
import { TerminalPanel } from '@/components/terminal/TerminalPanel'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Select } from '@/components/ui/Select'
import { Skeleton } from '@/components/ui/Skeleton'
import { userFacingError } from '@/lib/errors'

export function TerminalPage() {
  const { id } = useParams<{ id?: string }>()
  const navigate = useNavigate()
  const serversQuery = useQuery({
    queryKey: ['servers'],
    queryFn: ({ signal }) => listServers(signal),
  })
  const servers = serversQuery.data?.servers ?? []
  const current = id ? servers.find((server) => server.id === id) : undefined

  if (serversQuery.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-[62vh] w-full" />
      </div>
    )
  }

  if (serversQuery.isError) {
    return (
      <Alert variant="error" title="Servers could not be loaded">
        {userFacingError(serversQuery.error)}
      </Alert>
    )
  }

  if (servers.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Card className="max-w-md p-8 text-center">
          <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
            <ServerIcon className="h-6 w-6" aria-hidden="true" />
          </div>
          <h1 className="font-display text-ink-50 mt-4 text-xl font-semibold">Add a server first</h1>
          <p className="text-ink-400 mt-2 text-sm">
            The terminal opens an SSH PTY on a saved host. Save and verify a server before opening a
            session.
          </p>
          <Button asChild className="mt-5">
            <Link to="/servers">
              <Plus className="h-4 w-4" aria-hidden="true" />
              Go to servers
            </Link>
          </Button>
        </Card>
      </div>
    )
  }

  if (id && !current) {
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
        title="Terminal"
        description="An interactive SSH PTY. Commands execute on the remote host as the connected SSH user; Drovik does not rewrite them."
        actions={
          <div className="flex items-center gap-2">
            <Badge variant="mint">Phase 3</Badge>
            <Select
              aria-label="Select server"
              value={id ?? ''}
              onChange={(event) => {
                if (event.target.value) navigate(`/terminal/${event.target.value}`)
              }}
              className="w-56"
            >
              <option value="">Select a server…</option>
              {servers.map((server) => (
                <option key={server.id} value={server.id}>
                  {server.display_name} · {server.host}
                </option>
              ))}
            </Select>
          </div>
        }
      />

      {current ? (
        <TerminalPanel server={current} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {servers.map((server) => (
            <Card key={server.id}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <SquareTerminal className="text-signal-400 h-4 w-4" aria-hidden="true" />
                  {server.display_name}
                </CardTitle>
                <CardDescription>
                  {server.username}@{server.host}:{server.port}
                </CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                <Button
                  variant="outline"
                  className="w-full"
                  onClick={() => navigate(`/terminal/${server.id}`)}
                >
                  Open terminal
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <div className="border-ink-700 bg-ink-900/60 flex gap-3 rounded-lg border p-4">
        <SquareTerminal className="text-mint-400 mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <p className="text-ink-400 text-xs leading-5">
          Terminal content is not recorded by default. Closing the tab, signing out or losing the
          network closes the session; Drovik releases the SSH channel and PTY.
        </p>
      </div>
    </div>
  )
}
