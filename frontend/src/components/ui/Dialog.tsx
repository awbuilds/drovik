import type { ReactNode } from 'react'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { Button } from './Button'
import { cn } from '@/lib/cn'

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg'
}

const sizes = {
  sm: 'max-w-sm',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
}

export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  size = 'md',
}: DialogProps) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="bg-ink-950/80 data-[state=open]:animate-fade-in fixed inset-0 z-50 backdrop-blur-sm" />
        <DialogPrimitive.Content
          className={cn(
            'border-ink-700 bg-ink-900 fixed top-1/2 left-1/2 z-50 max-h-[88vh] w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border shadow-2xl',
            sizes[size],
          )}
        >
          <div className="border-ink-700 flex items-start justify-between gap-4 border-b px-5 py-4">
            <div className="space-y-1">
              <DialogPrimitive.Title className="font-display text-ink-50 text-base font-semibold">
                {title}
              </DialogPrimitive.Title>
              {description ? (
                <DialogPrimitive.Description className="text-ink-400 text-xs leading-5">
                  {description}
                </DialogPrimitive.Description>
              ) : null}
            </div>
            <DialogPrimitive.Close asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0">
                <X className="h-4 w-4" aria-hidden="true" />
                <span className="sr-only">Close</span>
              </Button>
            </DialogPrimitive.Close>
          </div>
          <div className="px-5 py-5">{children}</div>
          {footer ? (
            <div className="border-ink-700 bg-ink-850/50 flex items-center justify-end gap-2 border-t px-5 py-3">
              {footer}
            </div>
          ) : null}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}
