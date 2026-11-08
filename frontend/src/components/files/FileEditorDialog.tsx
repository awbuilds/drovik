import { useEffect, useState } from 'react'
import { FileText, RotateCcw } from 'lucide-react'
import { ApiError } from '@/api/client'
import { readFile, writeFile } from '@/api/files'
import { ConfirmDialog } from '@/components/servers/ConfirmDialog'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Skeleton } from '@/components/ui/Skeleton'
import { Textarea } from '@/components/ui/Textarea'
import { useToast } from '@/components/ui/useToast'
import { userFacingError } from '@/lib/errors'
import { formatBytes } from '@/lib/files'

interface FileEditorDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  serverId: string
  path: string | null
  csrfToken: string
  onSaved: () => void
}

export function FileEditorDialog({
  open,
  onOpenChange,
  serverId,
  path,
  csrfToken,
  onSaved,
}: FileEditorDialogProps) {
  const { toast } = useToast()
  const [content, setContent] = useState('')
  const [original, setOriginal] = useState('')
  const [modifiedAt, setModifiedAt] = useState<string | undefined>()
  const [size, setSize] = useState<number | undefined>()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [stale, setStale] = useState(false)
  const [confirmDiscard, setConfirmDiscard] = useState(false)

  const dirty = content !== original

  const load = async () => {
    if (!path) return
    setLoading(true)
    setError(null)
    setStale(false)
    try {
      const file = await readFile(serverId, path)
      setContent(file.content)
      setOriginal(file.content)
      setModifiedAt(file.modified_at)
      setSize(file.size)
    } catch (caught) {
      if (caught instanceof ApiError && caught.code === 'FILE_STALE') {
        setStale(true)
      }
      setError(userFacingError(caught))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open && path) {
      // oxlint-disable-next-line react/set-state-in-effect -- loading a remote file is an external synchronization
      void load()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, path, serverId])

  const requestClose = () => {
    if (dirty) {
      setConfirmDiscard(true)
      return
    }
    onOpenChange(false)
  }

  const save = async () => {
    if (!path) return
    setSaving(true)
    setError(null)
    try {
      await writeFile(serverId, path, content, { modifiedAt, size }, csrfToken)
      toast({ title: 'File saved', description: path, variant: 'success' })
      onSaved()
      onOpenChange(false)
    } catch (caught) {
      if (caught instanceof ApiError && caught.code === 'FILE_STALE') {
        setStale(true)
      }
      setError(userFacingError(caught))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!next) {
            requestClose()
          } else {
            onOpenChange(true)
          }
        }}
        title="Edit remote file"
        description={path ?? undefined}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={requestClose} disabled={saving}>
              Cancel
            </Button>
            <Button onClick={() => void save()} loading={saving} disabled={loading || Boolean(error && !content)}>
              Save file
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {error ? (
            <Alert variant="error" title="File could not be opened or saved">
              {error}
              {stale ? (
                <div className="mt-2">
                  <Button size="sm" variant="outline" onClick={() => void load()}>
                    <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
                    Reload remote file
                  </Button>
                </div>
              ) : null}
            </Alert>
          ) : null}

          <div className="text-ink-500 flex flex-wrap items-center gap-3 font-mono text-[10px] uppercase">
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" aria-hidden="true" />
              {path ? path.split('/').pop() : 'file'}
            </span>
            {size !== undefined ? <span>{formatBytes(size)}</span> : null}
            {modifiedAt ? <span>{new Date(modifiedAt).toLocaleString()}</span> : null}
            {dirty ? <span className="text-signal-400">Unsaved changes</span> : null}
          </div>

          {loading ? (
            <Skeleton className="h-[50vh] w-full" />
          ) : (
            <Textarea
              value={content}
              onChange={(event) => setContent(event.target.value)}
              spellCheck={false}
              autoComplete="off"
              className="h-[50vh] min-h-80 resize-y font-mono text-xs leading-5"
              aria-label="File content"
            />
          )}

          <p className="text-ink-500 text-[11px] leading-5">
            The file is written only when you choose Save. Drovik checks the remote modification
            time and size before overwriting.
          </p>
        </div>
      </Dialog>

      <ConfirmDialog
        open={confirmDiscard}
        onOpenChange={setConfirmDiscard}
        title="Discard unsaved changes?"
        description="The editor has changes that have not been saved to the remote server."
        confirmLabel="Discard changes"
        destructive
        onConfirm={() => {
          setConfirmDiscard(false)
          onOpenChange(false)
        }}
      />
    </>
  )
}
