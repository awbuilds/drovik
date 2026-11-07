import type { HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

type BadgeVariant = 'neutral' | 'signal' | 'mint' | 'rose' | 'outline'

const variants: Record<BadgeVariant, string> = {
  neutral: 'border-ink-700 bg-ink-800 text-ink-300',
  signal: 'border-signal-500/30 bg-signal-400/10 text-signal-300',
  mint: 'border-mint-500/30 bg-mint-400/10 text-mint-300',
  rose: 'border-rose-500/30 bg-rose-400/10 text-rose-300',
  outline: 'border-ink-600 bg-transparent text-ink-300',
}

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant
}

export function Badge({ className, variant = 'neutral', ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 font-mono text-[10px] font-medium tracking-wide uppercase',
        variants[variant],
        className,
      )}
      {...props}
    />
  )
}
