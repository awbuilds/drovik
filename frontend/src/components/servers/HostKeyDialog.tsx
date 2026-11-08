import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import type { HostKeyInfo, Server } from '@/api/servers'

interface HostKeyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
  hostKey: HostKeyInfo | null
  previous?: HostKeyInfo
  changed: boolean
  busy: boolean
  onTrust: (replace: boolean) => void
}

export function HostKeyDialog({
  open,
  onOpenChange,
  server,
  hostKey,
  previous,
  changed,
  busy,
  onTrust,
}: HostKeyDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={changed ? 'Host key changed' : 'Verify host key'}
      description={
        changed
          ? 'The presented key differs from the one previously approved for this server.'
          : 'Drovik has not connected to this server before. Verify the fingerprint out of band.'
      }
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button
            variant={changed ? 'danger' : 'primary'}
            loading={busy}
            onClick={() => onTrust(changed)}
          >
            {changed ? 'Replace trusted key' : 'Trust this host key'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {changed ? (
          <Alert
            variant="warning"
            title="This may indicate a rebuild or a man-in-the-middle attack"
          >
            Do not continue unless you can explain the change. Compare the new fingerprint with the
            server console or your infrastructure records before replacing the trusted key.
          </Alert>
        ) : (
          <Alert variant="info" title="Compare this fingerprint out of band">
            Run <code className="font-mono">ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub</code>{' '}
            on the server (or the appropriate host key) and confirm it matches exactly.
          </Alert>
        )}

        <div className="border-ink-700 bg-ink-950/50 rounded-lg border p-4">
          <div className="flex items-center justify-between gap-3">
            <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">
              Presented fingerprint
            </p>
            <Badge variant={changed ? 'rose' : 'signal'}>{hostKey?.key_type ?? 'unknown'}</Badge>
          </div>
          <p className="text-ink-100 mt-2 font-mono text-sm break-all">
            {hostKey?.fingerprint ?? '—'}
          </p>
          <p className="text-ink-500 mt-2 text-xs">
            {server?.display_name ?? 'Server'} · {server?.host}:{server?.port}
          </p>
        </div>

        {changed && previous ? (
          <div className="border-ink-700 bg-ink-900 rounded-lg border p-4">
            <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">
              Previously approved
            </p>
            <p className="text-ink-400 mt-2 font-mono text-xs break-all">{previous.fingerprint}</p>
          </div>
        ) : null}
      </div>
    </Dialog>
  )
}
