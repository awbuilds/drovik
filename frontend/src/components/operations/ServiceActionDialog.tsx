import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import type { ServiceAction, ServiceUnit } from '@/api/operations'

const copy: Record<ServiceAction, { title: string; confirm: string; warning: string; destructive: boolean }> = {
  start: {
    title: 'Start service',
    confirm: 'Start service',
    warning: 'Starting a service may expose network ports or run scheduled work immediately.',
    destructive: false,
  },
  stop: {
    title: 'Stop service',
    confirm: 'Stop service',
    warning: 'Stopping a service interrupts its work and may disconnect clients. Dependants are not changed by Drovik.',
    destructive: true,
  },
  restart: {
    title: 'Restart service',
    confirm: 'Restart service',
    warning: 'A restart briefly interrupts the service and may terminate active sessions or jobs.',
    destructive: true,
  },
  reload: {
    title: 'Reload service',
    confirm: 'Reload service',
    warning: 'Reload asks the service to re-read its configuration. Not every unit supports reload.',
    destructive: false,
  },
  enable: {
    title: 'Enable at boot',
    confirm: 'Enable unit',
    warning: 'Enabling changes what starts automatically at boot. The unit is not necessarily started now.',
    destructive: false,
  },
  disable: {
    title: 'Disable at boot',
    confirm: 'Disable unit',
    warning: 'Disabling prevents the unit from starting automatically at boot. It does not stop a running instance.',
    destructive: true,
  },
}

interface ServiceActionDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  unit: ServiceUnit | null
  action: ServiceAction
  busy: boolean
  error: string | null
  onConfirm: () => void
}

export function ServiceActionDialog({
  open,
  onOpenChange,
  unit,
  action,
  busy,
  error,
  onConfirm,
}: ServiceActionDialogProps) {
  const entry = copy[action]
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={entry.title}
      description={unit ? `${unit.name} · ${unit.active}/${unit.sub}` : undefined}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button variant={entry.destructive ? 'danger' : 'primary'} loading={busy} onClick={onConfirm}>
            {entry.confirm}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {error ? <Alert variant="error">{error}</Alert> : null}
        <Alert variant={entry.destructive ? 'warning' : 'info'} title={entry.title}>
          {entry.warning}
        </Alert>
        <p className="text-ink-400 text-xs leading-5">
          The action runs as the connected SSH user through <span className="font-mono">systemctl</span>.
          Drovik does not use sudo or escalate privileges, and re-reads the unit state after the action.
        </p>
      </div>
    </Dialog>
  )
}
