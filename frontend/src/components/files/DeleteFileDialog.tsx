import { useEffect, useState } from 'react'
import type { FileEntry } from '@/api/files'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'

interface DeleteFileDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  entry: FileEntry | null
  busy: boolean
  onConfirm: (recursive: boolean) => void
}

export function DeleteFileDialog({
  open,
  onOpenChange,
  entry,
  busy,
  onConfirm,
}: DeleteFileDialogProps) {
  const [recursive, setRecursive] = useState(false)

  useEffect(() => {
    if (open) {
      // oxlint-disable-next-line react/set-state-in-effect -- reset the confirmation when the target changes
      setRecursive(false)
    }
  }, [open, entry])

  const isDirectory = entry?.type === 'directory'

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Delete remote item?"
      description={entry?.path}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button variant="danger" loading={busy} onClick={() => onConfirm(recursive)}>
            Delete permanently
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Alert variant="warning" title="This action cannot be undone">
          Deleting removes the item from the remote server as the connected SSH user. Drovik does
          not move it to a recycle bin.
        </Alert>
        {isDirectory ? (
          <label className="border-ink-700 bg-ink-850 flex cursor-pointer items-start gap-3 rounded-md border p-3">
            <input
              type="checkbox"
              checked={recursive}
              onChange={(event) => setRecursive(event.target.checked)}
              className="accent-signal-400 mt-0.5 h-4 w-4"
            />
            <span className="text-ink-300 text-xs leading-5">
              Delete this directory and everything inside it recursively. Symlinks are removed, not
              followed. This is required if the directory is not empty.
            </span>
          </label>
        ) : null}
      </div>
    </Dialog>
  )
}
