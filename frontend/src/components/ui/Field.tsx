import type { ReactNode } from 'react'
import * as Label from '@radix-ui/react-label'
import { cn } from '@/lib/cn'

interface FieldProps {
  label: string
  htmlFor: string
  hint?: string
  error?: string | null
  children: ReactNode
  className?: string
}

export function Field({ label, htmlFor, hint, error, children, className }: FieldProps) {
  const describedBy = error ? `${htmlFor}-error` : hint ? `${htmlFor}-hint` : undefined
  return (
    <div className={cn('space-y-1.5', className)}>
      <Label.Root htmlFor={htmlFor} className="text-ink-200 text-sm font-medium">
        {label}
      </Label.Root>
      {children}
      {error ? (
        <p id={`${htmlFor}-error`} className="text-xs text-rose-400" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p id={`${htmlFor}-hint`} className="text-ink-400 text-xs">
          {hint}
        </p>
      ) : null}
      {describedBy ? <span className="sr-only">{describedBy}</span> : null}
    </div>
  )
}
