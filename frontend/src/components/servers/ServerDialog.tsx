import { useEffect, useState } from 'react'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Textarea } from '@/components/ui/Textarea'
import { useToast } from '@/components/ui/useToast'
import {
  createServer,
  updateServer,
  type AuthMethod,
  type Server,
  type UpdateServerInput,
} from '@/api/servers'
import { userFacingError } from '@/lib/errors'

interface ServerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
  csrfToken: string
  onSaved: () => void
}

interface FormState {
  displayName: string
  host: string
  port: string
  username: string
  authMethod: AuthMethod
  password: string
  privateKey: string
  passphrase: string
  tags: string
  notes: string
}

const emptyForm: FormState = {
  displayName: '',
  host: '',
  port: '22',
  username: '',
  authMethod: 'password',
  password: '',
  privateKey: '',
  passphrase: '',
  tags: '',
  notes: '',
}

function fromServer(server: Server | null): FormState {
  if (!server) {
    return emptyForm
  }
  return {
    displayName: server.display_name,
    host: server.host,
    port: String(server.port),
    username: server.username,
    authMethod: server.auth_method,
    password: '',
    privateKey: '',
    passphrase: '',
    tags: server.tags ?? '',
    notes: server.notes ?? '',
  }
}

export function ServerDialog({
  open,
  onOpenChange,
  server,
  csrfToken,
  onSaved,
}: ServerDialogProps) {
  const [form, setForm] = useState<FormState>(emptyForm)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const { toast } = useToast()

  useEffect(() => {
    if (open) {
      // oxlint-disable-next-line react/set-state-in-effect -- the form is initialized from the selected server when the dialog opens
      setForm(fromServer(server))
      setErrors({})
      setFormError(null)
    }
  }, [open, server])

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((current) => ({ ...current, [key]: value }))
  }

  const validate = (): boolean => {
    const next: Record<string, string> = {}
    if (!form.displayName.trim()) next.displayName = 'Enter a display name.'
    if (!form.host.trim()) next.host = 'Enter a hostname or IP address.'
    const port = Number(form.port)
    if (!Number.isInteger(port) || port < 1 || port > 65535)
      next.port = 'Use a port between 1 and 65535.'
    if (!form.username.trim()) next.username = 'Enter the SSH username.'
    if (form.authMethod === 'password') {
      if (!server && !form.password) next.password = 'Enter the SSH password.'
      if (server && server.auth_method !== 'password' && !form.password) {
        next.password = 'Enter a password for the new authentication method.'
      }
    } else {
      if (!server && !form.privateKey) next.privateKey = 'Paste the private key.'
      if (server && server.auth_method !== 'private_key' && !form.privateKey) {
        next.privateKey = 'Paste a private key for the new authentication method.'
      }
    }
    setErrors(next)
    return Object.keys(next).length === 0
  }

  const handleSubmit = async () => {
    if (!validate()) return
    setSubmitting(true)
    setFormError(null)
    try {
      const base = {
        display_name: form.displayName.trim(),
        host: form.host.trim(),
        port: Number(form.port),
        username: form.username.trim(),
        auth_method: form.authMethod,
        tags: form.tags.trim(),
        notes: form.notes.trim(),
      }
      if (server) {
        const input: UpdateServerInput = base
        if (form.password) input.password = form.password
        if (form.privateKey) input.private_key = form.privateKey
        if (form.passphrase) input.passphrase = form.passphrase
        await updateServer(server.id, input, csrfToken)
        toast({ title: 'Server updated', variant: 'success' })
      } else {
        await createServer(
          {
            ...base,
            password: form.authMethod === 'password' ? form.password : undefined,
            private_key: form.authMethod === 'private_key' ? form.privateKey : undefined,
            passphrase: form.passphrase || undefined,
          },
          csrfToken,
        )
        toast({
          title: 'Server saved',
          description: 'Test the connection to verify the host key.',
          variant: 'success',
        })
      }
      onSaved()
      onOpenChange(false)
    } catch (error) {
      setFormError(userFacingError(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={server ? 'Edit server' : 'Add server'}
      description={
        server
          ? 'Update connection metadata or replace the credential. Secrets are encrypted at rest.'
          : 'Drovik connects as this SSH user. The host key is verified before the server is trusted.'
      }
      size="lg"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="server-form" loading={submitting}>
            {server ? 'Save changes' : 'Save server'}
          </Button>
        </>
      }
    >
      <form
        id="server-form"
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault()
          void handleSubmit()
        }}
      >
        {formError ? <Alert variant="error">{formError}</Alert> : null}

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Display name" htmlFor="server-name" error={errors.displayName}>
            <Input
              id="server-name"
              value={form.displayName}
              onChange={(event) => update('displayName', event.target.value)}
              placeholder="Production web"
              invalid={Boolean(errors.displayName)}
            />
          </Field>
          <Field label="Tags" htmlFor="server-tags" hint="Comma-separated, optional.">
            <Input
              id="server-tags"
              value={form.tags}
              onChange={(event) => update('tags', event.target.value)}
              placeholder="prod, web"
            />
          </Field>
        </div>

        <div className="grid gap-4 sm:grid-cols-[1fr_7rem]">
          <Field label="Hostname or IP" htmlFor="server-host" error={errors.host}>
            <Input
              id="server-host"
              value={form.host}
              onChange={(event) => update('host', event.target.value)}
              placeholder="server.example.com"
              autoCapitalize="none"
              spellCheck={false}
              invalid={Boolean(errors.host)}
            />
          </Field>
          <Field label="Port" htmlFor="server-port" error={errors.port}>
            <Input
              id="server-port"
              type="number"
              min={1}
              max={65535}
              value={form.port}
              onChange={(event) => update('port', event.target.value)}
              invalid={Boolean(errors.port)}
            />
          </Field>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="SSH username" htmlFor="server-username" error={errors.username}>
            <Input
              id="server-username"
              value={form.username}
              onChange={(event) => update('username', event.target.value)}
              placeholder="deploy"
              autoCapitalize="none"
              spellCheck={false}
              invalid={Boolean(errors.username)}
            />
          </Field>
          <Field label="Authentication" htmlFor="server-auth">
            <Select
              id="server-auth"
              value={form.authMethod}
              onChange={(event) => update('authMethod', event.target.value as AuthMethod)}
            >
              <option value="password">Password</option>
              <option value="private_key">Private key</option>
            </Select>
          </Field>
        </div>

        {form.authMethod === 'password' ? (
          <Field
            label="SSH password"
            htmlFor="server-password"
            error={errors.password}
            hint={server ? 'Leave blank to keep the current password.' : undefined}
          >
            <Input
              id="server-password"
              type="password"
              value={form.password}
              onChange={(event) => update('password', event.target.value)}
              autoComplete="new-password"
              placeholder={server ? '••••••••' : 'Password for the SSH user'}
              invalid={Boolean(errors.password)}
            />
          </Field>
        ) : (
          <>
            <Field
              label="Private key"
              htmlFor="server-private-key"
              error={errors.privateKey}
              hint={
                server
                  ? 'Leave blank to keep the current key. Supported: OpenSSH and PEM private keys.'
                  : 'Supported: OpenSSH and PEM private keys. The key is encrypted before storage.'
              }
            >
              <Textarea
                id="server-private-key"
                value={form.privateKey}
                onChange={(event) => update('privateKey', event.target.value)}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                spellCheck={false}
                autoComplete="off"
                className="min-h-32"
              />
            </Field>
            <Field
              label="Key passphrase"
              htmlFor="server-passphrase"
              hint="Optional. Leave blank if the key is not encrypted."
            >
              <Input
                id="server-passphrase"
                type="password"
                value={form.passphrase}
                onChange={(event) => update('passphrase', event.target.value)}
                autoComplete="new-password"
                placeholder="Passphrase"
              />
            </Field>
          </>
        )}

        <Field label="Notes" htmlFor="server-notes" hint="Optional. Do not store secrets here.">
          <Textarea
            id="server-notes"
            value={form.notes}
            onChange={(event) => update('notes', event.target.value)}
            className="min-h-20 font-sans text-sm"
            placeholder="Role, owner or maintenance notes"
          />
        </Field>

        <div className="border-ink-700 bg-ink-850/60 rounded-md border p-3">
          <p className="text-ink-400 text-xs leading-5">
            Drovik never returns stored credentials to the browser. The first connection asks you to
            verify the server's host-key fingerprint; a changed key is blocked until you re-verify
            it.
          </p>
        </div>
      </form>
    </Dialog>
  )
}
