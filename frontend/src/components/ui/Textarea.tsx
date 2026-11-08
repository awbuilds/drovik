import * as React from 'react'
import { cn } from '@/lib/cn'

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    className={cn(
      'border-ink-600 bg-ink-900 text-ink-50 placeholder:text-ink-400 min-h-24 w-full rounded-md border px-3 py-2 font-mono text-xs leading-5 transition-colors',
      'hover:border-ink-500 focus:border-signal-400 focus:ring-signal-400/30 focus:ring-2 focus:outline-none',
      className,
    )}
    {...props}
  />
))
Textarea.displayName = 'Textarea'
