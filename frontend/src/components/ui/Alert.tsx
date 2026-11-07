import type { ReactNode } from 'react'
import { CheckCircle2, Info, TriangleAlert, XCircle } from 'lucide-react'
import { cn } from '@/lib/cn'

type AlertVariant = 'info' | 'success' | 'warning' | 'error'

const styles: Record<AlertVariant, string> = {
  info: 'border-ink-600 bg-ink-800/60 text-ink-200',
  success: 'border-mint-500/30 bg-mint-400/10 text-mint-100',
  warning: 'border-signal-500/30 bg-signal-400/10 text-signal-100',
  error: 'border-rose-500/30 bg-rose-500/10 text-rose-100',
}

const icons = {
  info: Info,
  success: CheckCircle2,
  warning: TriangleAlert,
  error: XCircle,
} as const

interface AlertProps {
  variant?: AlertVariant
  title?: string
  children: ReactNode
  className?: string
}

export function Alert({ variant = 'info', title, children, className }: AlertProps) {
  const Icon = icons[variant]
  return (
    <div
      role={variant === 'error' ? 'alert' : 'status'}
      className={cn('flex gap-3 rounded-md border p-3 text-sm', styles[variant], className)}
    >
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div className="space-y-1">
        {title ? <p className="font-medium">{title}</p> : null}
        <div className="text-sm text-current/85">{children}</div>
      </div>
    </div>
  )
}
