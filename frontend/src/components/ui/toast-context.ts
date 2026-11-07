import { createContext } from 'react'

export type ToastVariant = 'info' | 'success' | 'error'

export interface ToastItem {
  id: string
  title: string
  description?: string
  variant: ToastVariant
}

export interface ToastContextValue {
  toast: (item: Omit<ToastItem, 'id'>) => void
}

export const ToastContext = createContext<ToastContextValue | null>(null)
