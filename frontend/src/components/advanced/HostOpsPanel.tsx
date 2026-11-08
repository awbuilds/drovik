import { useState } from 'react'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { useAuth } from '@/auth/useAuth'
import { networkOp, storageOp, type HostOpResult, type NetworkAction, type StorageAction } from '@/api/advanced'
import { userFacingError } from '@/lib/errors'

type Kind = 'storage' | 'network'
type Action = StorageAction | NetworkAction

const labels: Record<Action, string> = {
  mount: 'Mount filesystem',
  unmount: 'Unmount filesystem',
  format: 'Format device',
  partition: 'Create partition table',
  link: 'Set link state',
  address: 'Add/remove address',
  route: 'Add/remove route',
  firewall: 'Firewall rule',
}

export function HostOpsPanel({ serverId, kind, enabled }: { serverId: string; kind: Kind; enabled: boolean }) {
  const { csrfToken } = useAuth()
  const [target, setTarget] = useState<Action | null>(null)
  const [device, setDevice] = useState('/dev/sdb')
  const [mountpoint, setMountpoint] = useState('/mnt/data')
  const [filesystem, setFilesystem] = useState('ext4')
  const [iface, setIface] = useState('eth0')
  const [state, setState] = useState('up')
  const [cidr, setCidr] = useState('192.0.2.10/24')
  const [destination, setDestination] = useState('default')
  const [gateway, setGateway] = useState('192.0.2.1')
  const [addressAction, setAddressAction] = useState('add')
  const [routeAction, setRouteAction] = useState('add')
  const [port, setPort] = useState('22')
  const [protocol, setProtocol] = useState('tcp')
  const [firewallAction, setFirewallAction] = useState('allow')
  const [confirmDevice, setConfirmDevice] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<HostOpResult | null>(null)

  const actions: Action[] = kind === 'storage'
    ? ['mount', 'unmount', 'format', 'partition']
    : ['link', 'address', 'route', 'firewall']

  const open = (action: Action) => {
    setTarget(action)
    setConfirmed(false)
    setConfirmDevice('')
    setError(null)
    setResult(null)
  }

  const submit = async () => {
    if (!target || !confirmed) return
    setBusy(true)
    setError(null)
    setResult(null)
    try {
      let response: HostOpResult
      if (kind === 'storage') {
        const body: Record<string, string | boolean> = { confirm: true }
        if (target === 'unmount') body.mountpoint = mountpoint
        if (target === 'mount') {
          body.device = device
          body.mountpoint = mountpoint
          body.filesystem = filesystem
        }
        if (target === 'format' || target === 'partition') {
          body.device = device
          body.filesystem = filesystem
          body.confirm_device = confirmDevice
        }
        response = await storageOp(serverId, target as StorageAction, body, csrfToken)
      } else {
        const body: Record<string, string | number | boolean> = { confirm: true, interface: iface }
        if (target === 'link') body.state = state
        if (target === 'address') {
          body.cidr = cidr
          body.action = addressAction
        }
        if (target === 'route') {
          body.destination = destination
          body.gateway = gateway
          body.action = routeAction
        }
        if (target === 'firewall') {
          body.port = Number(port)
          body.protocol = protocol
          body.action = firewallAction
        }
        response = await networkOp(serverId, target as NetworkAction, body, csrfToken)
      }
      setResult(response)
      if (!response.success) setError('The remote command reported a failure. Review the output.')
    } catch (caught) {
      setError(userFacingError(caught))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-3">
        <div>
          <CardTitle>{kind === 'storage' ? 'Destructive storage operations' : 'Destructive network operations'}</CardTitle>
          <CardDescription>
            {enabled
              ? 'Administrator-only, confirmed and audited. These commands can destroy data or disconnect the host.'
              : 'Disabled by configuration (DROVIK_ENABLE_DESTRUCTIVE_HOST_OPS).'}
          </CardDescription>
        </div>
        <Badge variant={enabled ? 'rose' : 'outline'}>{enabled ? 'enabled' : 'disabled'}</Badge>
      </CardHeader>
      <CardContent className="space-y-3 pt-4">
        <div className="flex flex-wrap gap-2">
          {actions.map((action) => (
            <Button key={action} size="sm" variant="outline" disabled={!enabled} onClick={() => open(action)}>
              {labels[action]}
            </Button>
          ))}
        </div>
        {enabled ? (
          <Alert variant="warning" title="No undo">
            Formatting and partitioning erase data. Network changes can cut the SSH connection Drovik
            depends on. Use a disposable host for tests.
          </Alert>
        ) : null}
      </CardContent>

      <Dialog
        open={target !== null}
        onOpenChange={(open) => { if (!open) setTarget(null) }}
        title={target ? labels[target] : 'Host operation'}
        description="Runs one enumerated command as the connected SSH account, after explicit confirmation."
        size="md"
        footer={
          <>
            <Button variant="ghost" onClick={() => setTarget(null)} disabled={busy}>Close</Button>
            <Button
              variant={target === 'format' || target === 'partition' || target === 'unmount' ? 'danger' : 'primary'}
              onClick={() => void submit()}
              loading={busy}
              disabled={!confirmed || ((target === 'format' || target === 'partition') && confirmDevice !== device)}
            >
              Run command
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {error ? <Alert variant="error">{error}</Alert> : null}
          {target === 'mount' || target === 'format' || target === 'partition' ? (
            <Field label="Block device" htmlFor="op-device" hint="Example: /dev/sdb or /dev/sdb1">
              <Input id="op-device" value={device} onChange={(event) => setDevice(event.target.value)} className="font-mono" />
            </Field>
          ) : null}
          {target === 'mount' || target === 'unmount' ? (
            <Field label="Mountpoint" htmlFor="op-mountpoint">
              <Input id="op-mountpoint" value={mountpoint} onChange={(event) => setMountpoint(event.target.value)} className="font-mono" />
            </Field>
          ) : null}
          {target === 'mount' || target === 'format' || target === 'partition' ? (
            <Field label="Filesystem" htmlFor="op-filesystem">
              <Select id="op-filesystem" value={filesystem} onChange={(event) => setFilesystem(event.target.value)}>
                <option value="ext4">ext4</option>
                <option value="xfs">xfs</option>
                <option value="btrfs">btrfs</option>
                <option value="vfat">vfat</option>
              </Select>
            </Field>
          ) : null}
          {target === 'format' || target === 'partition' ? (
            <Field label="Type the device to confirm" htmlFor="op-confirm-device">
              <Input id="op-confirm-device" value={confirmDevice} onChange={(event) => setConfirmDevice(event.target.value)} className="font-mono" />
            </Field>
          ) : null}
          {kind === 'network' ? (
            <Field label="Interface" htmlFor="op-interface">
              <Input id="op-interface" value={iface} onChange={(event) => setIface(event.target.value)} className="font-mono" />
            </Field>
          ) : null}
          {target === 'link' ? (
            <Field label="State" htmlFor="op-state">
              <Select id="op-state" value={state} onChange={(event) => setState(event.target.value)}>
                <option value="up">up</option>
                <option value="down">down</option>
              </Select>
            </Field>
          ) : null}
          {target === 'address' ? (
            <>
              <Field label="CIDR" htmlFor="op-cidr"><Input id="op-cidr" value={cidr} onChange={(event) => setCidr(event.target.value)} className="font-mono" /></Field>
              <Field label="Action" htmlFor="op-address-action">
                <Select id="op-address-action" value={addressAction} onChange={(event) => setAddressAction(event.target.value)}>
                  <option value="add">add</option>
                  <option value="del">delete</option>
                </Select>
              </Field>
            </>
          ) : null}
          {target === 'route' ? (
            <>
              <Field label="Destination" htmlFor="op-destination"><Input id="op-destination" value={destination} onChange={(event) => setDestination(event.target.value)} className="font-mono" /></Field>
              <Field label="Gateway" htmlFor="op-gateway"><Input id="op-gateway" value={gateway} onChange={(event) => setGateway(event.target.value)} className="font-mono" /></Field>
              <Field label="Action" htmlFor="op-route-action">
                <Select id="op-route-action" value={routeAction} onChange={(event) => setRouteAction(event.target.value)}>
                  <option value="add">add</option>
                  <option value="del">delete</option>
                </Select>
              </Field>
            </>
          ) : null}
          {target === 'firewall' ? (
            <>
              <Field label="Port" htmlFor="op-port"><Input id="op-port" value={port} onChange={(event) => setPort(event.target.value)} className="font-mono" /></Field>
              <Field label="Protocol" htmlFor="op-protocol">
                <Select id="op-protocol" value={protocol} onChange={(event) => setProtocol(event.target.value)}>
                  <option value="tcp">tcp</option>
                  <option value="udp">udp</option>
                </Select>
              </Field>
              <Field label="Action" htmlFor="op-firewall-action">
                <Select id="op-firewall-action" value={firewallAction} onChange={(event) => setFirewallAction(event.target.value)}>
                  <option value="allow">allow</option>
                  <option value="deny">deny</option>
                </Select>
              </Field>
            </>
          ) : null}
          <label className="text-ink-300 flex items-start gap-2 text-xs">
            <input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} className="accent-signal-400 mt-0.5 h-4 w-4" />
            I understand this command changes the managed host and cannot be undone.
          </label>
          {result ? (
            <pre className="bg-ink-950 text-ink-200 max-h-64 overflow-auto rounded-md p-3 font-mono text-[10px] whitespace-pre-wrap">{result.output || '(no output)'}</pre>
          ) : null}
        </div>
      </Dialog>
    </Card>
  )
}
