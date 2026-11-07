import * as React from 'react'
import { cn } from '@/lib/cn'

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, invalid, ...props }, ref) => {
    return (
      <input
        ref={ref}
        aria-invalid={invalid || undefined}
        className={cn(
          'border-ink-600 bg-ink-900 text-ink-50 placeholder:text-ink-400 h-10 w-full rounded-md border px-3 text-sm transition-colors',
          'hover:border-ink-500 focus:border-signal-400 focus:ring-signal-400/30 focus:ring-2 focus:outline-none',
          'disabled:bg-ink-850 disabled:text-ink-400 disabled:cursor-not-allowed',
          'aria-invalid:border-rose-500 aria-invalid:focus:border-rose-400 aria-invalid:focus:ring-rose-400/20',
          className,
        )}
        {...props}
      />
    )
  },
)
Input.displayName = 'Input'
