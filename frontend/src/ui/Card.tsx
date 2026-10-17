import type { ReactNode, HTMLAttributes } from 'react'

export function Card({ children, className, ...rest }: { children?: ReactNode; className?: string } & HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={`rounded-[var(--radius)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-[var(--shadow-panel)] ${className ?? ''}`}
      {...rest}
    >
      {children}
    </div>
  )
}

export function CardHeader({ title, subtitle, right }: { title: string; subtitle?: string; right?: ReactNode }) {
  return (
    <div className="flex items-start justify-between px-5 pt-4 pb-3">
      <div>
        <h3 className="text-[13px] font-semibold text-[var(--text-strong)]">{title}</h3>
        {subtitle && <p className="mt-0.5 text-[11px] text-[var(--text-soft)]">{subtitle}</p>}
      </div>
      {right}
    </div>
  )
}

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'brand' | 'warn' | 'danger' }) {
  const tones: Record<string, string> = {
    neutral: 'bg-[var(--bg-subtle)] text-[var(--text-muted)] border-[var(--border)]',
    brand: 'bg-[var(--brand-surface)] text-[var(--brand-ink)] border-transparent',
    warn: 'bg-[var(--warn-soft)] text-[var(--warn-ink)] border-transparent',
    danger: 'bg-[var(--danger-soft)] text-[var(--danger-ink)] border-transparent',
  }
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium ${tones[tone]}`}>
      {children}
    </span>
  )
}

export function Empty({ label, hint }: { label: string; hint?: string }) {
  return (
    <div className="flex flex-col items-center gap-1 py-8 text-center">
      <div className="text-[12px] font-medium text-[var(--text-soft)]">{label}</div>
      {hint && <div className="text-[11px] text-[var(--text-faint)]">{hint}</div>}
    </div>
  )
}