import { cn } from '@/lib/cn'

interface LogoProps {
  className?: string
  showWordmark?: boolean
  size?: 'sm' | 'md' | 'lg'
}

const markSizes = {
  sm: 'h-7 w-7',
  md: 'h-9 w-9',
  lg: 'h-12 w-12',
}

const textSizes = {
  sm: 'text-sm',
  md: 'text-base',
  lg: 'text-xl',
}

export function Logo({ className, showWordmark = true, size = 'md' }: LogoProps) {
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <svg
        viewBox="0 0 64 64"
        role="img"
        aria-label="Drovik"
        className={cn('shrink-0', markSizes[size])}
      >
        <rect width="64" height="64" rx="14" className="fill-ink-950" />
        <rect x="14" y="14" width="15" height="15" rx="3" className="fill-signal-400" />
        <rect x="35" y="14" width="15" height="15" rx="3" className="fill-ink-700" />
        <rect x="14" y="35" width="15" height="15" rx="3" className="fill-ink-700" />
        <rect x="35" y="35" width="15" height="15" rx="3" className="fill-signal-400" />
        <path d="M14 46 L50 18" className="stroke-ink-950" strokeWidth="4" strokeLinecap="round" />
      </svg>
      {showWordmark ? (
        <span
          className={cn(
            'font-display text-ink-50 font-semibold tracking-[0.18em] uppercase',
            textSizes[size],
          )}
        >
          Drovik
        </span>
      ) : null}
    </span>
  )
}

export function DrovikMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" aria-hidden="true" className={cn('h-6 w-6', className)}>
      <rect width="64" height="64" rx="14" className="fill-ink-950" />
      <rect x="14" y="14" width="15" height="15" rx="3" className="fill-signal-400" />
      <rect x="35" y="14" width="15" height="15" rx="3" className="fill-ink-700" />
      <rect x="14" y="35" width="15" height="15" rx="3" className="fill-ink-700" />
      <rect x="35" y="35" width="15" height="15" rx="3" className="fill-signal-400" />
      <path d="M14 46 L50 18" className="stroke-ink-950" strokeWidth="4" strokeLinecap="round" />
    </svg>
  )
}
