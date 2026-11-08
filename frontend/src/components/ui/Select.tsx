import * as React from 'react'
import { cn } from '@/lib/cn'

export const Select = React.forwardRef<
  HTMLSelectElement,
  React.SelectHTMLAttributes<HTMLSelectElement>
>(({ className, children, ...props }, ref) => (
  <select
    ref={ref}
    className={cn(
      'border-ink-600 bg-ink-900 text-ink-50 h-10 w-full rounded-md border px-3 text-sm transition-colors',
      'hover:border-ink-500 focus:border-signal-400 focus:ring-signal-400/30 focus:ring-2 focus:outline-none',
      className,
    )}
    {...props}
  >
    {children}
  </select>
))
Select.displayName = 'Select'
