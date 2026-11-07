import { cn } from '@/lib/cn'

type Status = 'online' | 'offline' | 'warning' | 'pending'

const colors: Record<Status, string> = {
  online: 'bg-mint-400',
  offline: 'bg-ink-500',
  warning: 'bg-signal-400',
  pending: 'bg-ink-400',
}

interface StatusDotProps {
  status: Status
  label: string
  className?: string
  pulse?: boolean
}

export function StatusDot({ status, label, className, pulse }: StatusDotProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <span className="relative flex h-2 w-2">
        {pulse ? (
          <span
            className={cn(
              'absolute inline-flex h-full w-full animate-ping rounded-full opacity-60',
              colors[status],
            )}
          />
        ) : null}
        <span className={cn('relative inline-flex h-2 w-2 rounded-full', colors[status])} />
      </span>
      <span className="text-ink-300 text-xs">{label}</span>
    </span>
  )
}
