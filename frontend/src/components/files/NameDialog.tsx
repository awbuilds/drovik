import { useEffect, useState } from 'react'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'

interface NameDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  label: string
  initialValue?: string
  confirmLabel: string
  busy?: boolean
  error?: string | null
  onSubmit: (value: string) => void
}

export function NameDialog({
  open,
  onOpenChange,
  title,
  description,
  label,
  initialValue = '',
  confirmLabel,
  busy,
  error,
  onSubmit,
}: NameDialogProps) {
  const [value, setValue] = useState(initialValue)
  const [validation, setValidation] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      // oxlint-disable-next-line react/set-state-in-effect -- the dialog is initialized from the selected action
      setValue(initialValue)
      // oxlint-disable-next-line react/set-state-in-effect -- the dialog is initialized from the selected action
      setValidation(null)
    }
  }, [open, initialValue])

  const submit = () => {
    const name = value.trim()
    if (!name) {
      setValidation('Enter a name.')
      return
    }
    if (name.includes('/') || name.includes('\x00')) {
      setValidation('Names cannot contain slashes or NUL bytes.')
      return
    }
    setValidation(null)
    onSubmit(name)
  }

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
          <Button type="submit" form="name-dialog-form" loading={busy}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <form
        id="name-dialog-form"
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
      >
        {error ? <Alert variant="error" className="mb-4">{error}</Alert> : null}
        <Field label={label} htmlFor="name-dialog-input" error={validation}>
          <Input
            id="name-dialog-input"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            autoComplete="off"
            autoFocus
            invalid={Boolean(validation)}
          />
        </Field>
      </form>
    </Dialog>
  )
}
