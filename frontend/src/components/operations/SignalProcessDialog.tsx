import { useEffect, useState } from 'react'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Select } from '@/components/ui/Select'
import type { ProcessInfo, ProcessSignal } from '@/api/operations'

interface SignalProcessDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  process: ProcessInfo | null
  busy: boolean
  error: string | null
  onConfirm: (signal: ProcessSignal) => void
}

export function SignalProcessDialog({
  open,
  onOpenChange,
  process,
  busy,
  error,
  onConfirm,
}: SignalProcessDialogProps) {
  const [signal, setSignal] = useState<ProcessSignal>('SIGTERM')
  const [confirmedKill, setConfirmedKill] = useState(false)

  useEffect(() => {
    if (open) {
      // oxlint-disable-next-line react/set-state-in-effect -- the dialog is initialized for the selected process
      setSignal('SIGTERM')
      // oxlint-disable-next-line react/set-state-in-effect -- the dialog is initialized for the selected process
      setConfirmedKill(false)
    }
  }, [open, process])

  const canConfirm = signal === 'SIGTERM' || confirmedKill

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Signal process"
      description={process ? `PID ${process.pid} · ${process.user}` : undefined}
      size="md"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button
            variant={signal === 'SIGKILL' ? 'danger' : 'primary'}
            loading={busy}
            disabled={!canConfirm}
            onClick={() => onConfirm(signal)}
          >
            Send {signal}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {error ? <Alert variant="error">{error}</Alert> : null}
        <Alert variant="warning" title="Terminating a process can cause data loss">
          The process may be holding files, database transactions or client connections. Drovik
          sends the signal as the connected SSH account; the operating system may deny it.
        </Alert>

        <div className="border-ink-700 bg-ink-950/50 rounded-lg border p-3">
          <p className="text-ink-500 font-mono text-[10px] tracking-[0.18em] uppercase">Command</p>
          <p className="text-ink-200 mt-2 break-all font-mono text-xs">{process?.command ?? '—'}</p>
        </div>

        <label className="block space-y-2">
          <span className="text-ink-300 text-xs font-medium">Signal</span>
          <Select
            value={signal}
            onChange={(event) => setSignal(event.target.value as ProcessSignal)}
            aria-label="Signal"
          >
            <option value="SIGTERM">SIGTERM · graceful shutdown</option>
            <option value="SIGKILL">SIGKILL · immediate, cannot be caught</option>
          </Select>
        </label>

        {signal === 'SIGKILL' ? (
          <div className="border-rose-500/30 bg-rose-500/10 rounded-md border p-3">
            <p className="text-rose-200 text-xs leading-5">
              SIGKILL cannot be handled by the process. Unsaved data will be lost and the service
              may not restart cleanly. Prefer SIGTERM first.
            </p>
            <label className="text-ink-200 mt-3 flex items-start gap-2 text-xs">
              <input
                type="checkbox"
                checked={confirmedKill}
                onChange={(event) => setConfirmedKill(event.target.checked)}
                className="accent-rose-400 mt-0.5 h-4 w-4"
              />
              I understand and want to send SIGKILL to PID {process?.pid}.
            </label>
          </div>
        ) : null}

        <p className="text-ink-500 text-[11px] leading-5">
          Drovik rechecks the process start time before signaling to reduce the risk of acting on a
          reused PID. The list refreshes after the action so you can confirm the result.
        </p>
      </div>
    </Dialog>
  )
}
