import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/cn'

type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger'
type ButtonSize = 'sm' | 'md' | 'lg' | 'icon'

const variants: Record<ButtonVariant, string> = {
  primary:
    'bg-signal-400 text-ink-950 hover:bg-signal-300 active:bg-signal-500 shadow-[0_1px_0_rgba(255,255,255,0.25)_inset]',
  secondary: 'bg-ink-700 text-ink-50 hover:bg-ink-600 active:bg-ink-700',
  outline:
    'border border-ink-600 bg-transparent text-ink-100 hover:border-ink-500 hover:bg-ink-800',
  ghost: 'bg-transparent text-ink-200 hover:bg-ink-800 hover:text-ink-50',
  danger: 'bg-rose-500 text-white hover:bg-rose-400 active:bg-rose-500',
}

const sizes: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-xs',
  md: 'h-10 px-4 text-sm',
  lg: 'h-11 px-5 text-sm',
  icon: 'h-9 w-9',
}

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  asChild?: boolean
  loading?: boolean
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = 'primary',
      size = 'md',
      asChild = false,
      loading = false,
      disabled,
      children,
      ...props
    },
    ref,
  ) => {
    const Comp = asChild ? Slot : 'button'
    return (
      <Comp
        ref={ref}
        className={cn(
          'focus-visible:ring-signal-400 focus-visible:ring-offset-ink-950 inline-flex items-center justify-center gap-2 rounded-md font-medium transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-45',
          variants[variant],
          sizes[size],
          className,
        )}
        disabled={disabled || loading}
        aria-busy={loading || undefined}
        {...props}
      >
        {asChild ? (
          children
        ) : (
          <>
            {loading ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : null}
            {children}
          </>
        )}
      </Comp>
    )
  },
)
Button.displayName = 'Button'
