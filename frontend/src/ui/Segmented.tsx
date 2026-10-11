import type { ReactNode } from 'react'

export function Segmented<T extends string>({ value, onChange, options }: {
  value: T
  onChange: (v: T) => void
  options: { value: T; label: ReactNode }[]
}) {
  return (
    <div className="flex items-center gap-0.5 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-0.5">
      {options.map((opt) => (
        <button
          key={opt.value}
          className={`rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
            value === opt.value
              ? 'bg-[var(--accent)] text-[var(--accent-contrast)]'
              : 'text-[var(--text-muted)] hover:bg-[var(--bg-fill-secondary)] hover:text-[var(--text-strong)]'
          }`}
          onClick={() => onChange(opt.value)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  )
}