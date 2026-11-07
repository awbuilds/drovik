import { cn } from '@/lib/cn'

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn('bg-ink-800 animate-pulse rounded-md', className)} aria-hidden="true" />
}
