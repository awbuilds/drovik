import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { CheckCircle2, Info, X, XCircle } from 'lucide-react'
import { cn } from '@/lib/cn'
import {
  ToastContext,
  type ToastContextValue,
  type ToastItem,
  type ToastVariant,
} from './toast-context'

const icons = {
  info: Info,
  success: CheckCircle2,
  error: XCircle,
} as const

const styles: Record<ToastVariant, string> = {
  info: 'border-ink-600',
  success: 'border-mint-500/40',
  error: 'border-rose-500/40',
}

function newToastId() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  return Math.random().toString(16).slice(2)
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const dismiss = useCallback((id: string) => {
    setToasts((current) => current.filter((item) => item.id !== id))
  }, [])

  const toast = useCallback((item: Omit<ToastItem, 'id'>) => {
    setToasts((current) => [...current.slice(-3), { ...item, id: newToastId() }])
  }, [])

  const value = useMemo<ToastContextValue>(() => ({ toast }), [toast])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className="pointer-events-none fixed right-4 bottom-4 z-[100] flex w-full max-w-sm flex-col gap-2"
        aria-live="polite"
        aria-atomic="false"
      >
        {toasts.map((item) => (
          <ToastCard key={item.id} item={item} onDismiss={dismiss} />
        ))}
      </div>
    </ToastContext.Provider>
  )
}

function ToastCard({ item, onDismiss }: { item: ToastItem; onDismiss: (id: string) => void }) {
  const Icon = icons[item.variant]

  useEffect(() => {
    const timer = window.setTimeout(() => onDismiss(item.id), 5000)
    return () => window.clearTimeout(timer)
  }, [item.id, onDismiss])

  return (
    <div
      className={cn(
        'bg-ink-900/95 pointer-events-auto flex items-start gap-3 rounded-lg border p-3 shadow-2xl backdrop-blur',
        styles[item.variant],
      )}
      role={item.variant === 'error' ? 'alert' : 'status'}
    >
      <Icon
        className={cn(
          'mt-0.5 h-4 w-4 shrink-0',
          item.variant === 'error'
            ? 'text-rose-400'
            : item.variant === 'success'
              ? 'text-mint-400'
              : 'text-ink-300',
        )}
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1">
        <p className="text-ink-100 text-sm font-medium">{item.title}</p>
        {item.description ? (
          <p className="text-ink-400 mt-0.5 text-xs">{item.description}</p>
        ) : null}
      </div>
      <button
        type="button"
        onClick={() => onDismiss(item.id)}
        className="text-ink-500 hover:text-ink-200 rounded p-1"
        aria-label="Dismiss notification"
      >
        <X className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
    </div>
  )
}
