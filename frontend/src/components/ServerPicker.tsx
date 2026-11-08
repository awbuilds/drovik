import type { Server } from '@/api/servers'
import { Select } from '@/components/ui/Select'

interface ServerPickerProps {
  servers: Server[]
  serverId?: string
  onChange: (serverId: string) => void
  label?: string
}

export function ServerPicker({ servers, serverId, onChange, label = 'Select server' }: ServerPickerProps) {
  return (
    <Select
      aria-label={label}
      value={serverId ?? ''}
      onChange={(event) => {
        if (event.target.value) onChange(event.target.value)
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
  )
}
