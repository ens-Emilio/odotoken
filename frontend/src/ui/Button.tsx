import type { ButtonHTMLAttributes, ReactNode } from 'react'

// Two variants only: the action the screen exists for, and everything else.
// A third weight would be a hierarchy the app does not have.
export function Button({
  children,
  variant = 'secondary',
  className,
  ...rest
}: {
  children?: ReactNode
  variant?: 'primary' | 'secondary'
} & ButtonHTMLAttributes<HTMLButtonElement>) {
  const base =
    'inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[12px] font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-[var(--brand)]'
  const tone =
    variant === 'primary'
      ? 'bg-[var(--text-strong)] text-[var(--bg)] hover:opacity-90'
      : 'border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-strong)] hover:bg-[var(--bg-fill-secondary)]'
  return (
    <button type="button" className={`${base} ${tone} ${className ?? ''}`} {...rest}>
      {children}
    </button>
  )
}
