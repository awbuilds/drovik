import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'

interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  confirmLabel: string
  busy?: boolean
  destructive?: boolean
  onConfirm: () => void
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  busy,
  destructive,
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button variant={destructive ? 'danger' : 'primary'} loading={busy} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <p className="text-ink-300 text-sm leading-6">
        {destructive
          ? 'This action cannot be undone. The saved connection and its stored credential will be removed from Drovik. The remote server is not changed.'
          : 'Confirm this action.'}
      </p>
    </Dialog>
  )
}
