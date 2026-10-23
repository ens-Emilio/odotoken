import { useState, useRef, useEffect, useCallback, type KeyboardEvent } from 'react'

export interface SelectOption {
  value: string
  label: string
}

export interface SelectGroup {
  label: string
  options: SelectOption[]
}

interface SelectProps {
  value: string
  onChange: (value: string) => void
  options: SelectOption[] | SelectGroup[]
  placeholder?: string
  disabled?: boolean
  id?: string
  className?: string
  dropdownClassName?: string
}

function isGrouped(options: SelectOption[] | SelectGroup[]): options is SelectGroup[] {
  return options.length > 0 && 'options' in options[0]
}

function flattenOptions(options: SelectOption[] | SelectGroup[]): SelectOption[] {
  if (isGrouped(options)) {
    return options.flatMap(g => g.options)
  }
  return options as SelectOption[]
}

export function Select({ value, onChange, options, placeholder = 'Select…', disabled, id, className, dropdownClassName }: SelectProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const flat = flattenOptions(options)
  const selectedLabel = flat.find(o => o.value === value)?.label

  const close = useCallback(() => setOpen(false), [])

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) close()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open, close])

  useEffect(() => {
    if (open && listRef.current) {
      const active = listRef.current.querySelector('[aria-selected="true"]') as HTMLElement | null
      active?.scrollIntoView({ block: 'nearest' })
    }
  }, [open])

  const handleKey = (e: KeyboardEvent) => {
    if (disabled) return
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      setOpen(v => !v)
      return
    }
    if (e.key === 'Escape') {
      setOpen(false)
      return
    }
    if (!open) return
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const idx = flat.findIndex(o => o.value === value)
      const next = e.key === 'ArrowDown'
        ? Math.min(idx + 1, flat.length - 1)
        : Math.max(idx - 1, 0)
      onChange(flat[next].value)
    }
  }

  const grouped = isGrouped(options)

  return (
 <div ref={containerRef} className={`relative w-full ${className ?? ''}`} onKeyDown={handleKey}>
      <button
        type="button"
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
        disabled={disabled}
        onClick={() => !disabled && setOpen(v => !v)}
        className="flex w-full items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2 text-left text-[11px] font-medium text-[var(--text-strong)] outline-none focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50"
      >
        <span className={`block max-w-[180px] truncate ${selectedLabel ? '' : 'text-[var(--text-muted)]'}`}>
          {selectedLabel ?? placeholder}
        </span>
        <svg className={`ml-2 h-3 w-3 shrink-0 text-[var(--text-muted)] transition-transform ${open ? 'rotate-180' : ''}`} viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 4.5L6 7.5L9 4.5" />
        </svg>
      </button>

      {open && (
        <ul
          ref={listRef}
          role="listbox"
          className={`select-dropdown-list absolute left-0 z-50 mt-1 max-h-60 overflow-auto rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] py-1 shadow-[var(--shadow-pop)] ${dropdownClassName ?? ''}`}
 style={{ maxWidth: '256px', width: '100%' }}
        >
          {grouped
            ? (options as SelectGroup[]).map(group => (
                <li key={group.label} role="presentation">
                  <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                    {group.label}
                  </div>
                  {group.options.map(opt => (
                    <li
                      key={opt.value}
                      role="option"
                      aria-selected={opt.value === value}
                      onClick={() => { onChange(opt.value); close() }}
                      className={`select-option ${opt.value === value ? 'select-option-active' : ''}`}
 style={opt.value === value ? { backgroundColor: '#3a3a3a', color: '#f2f2f2', fontWeight: 500, maxWidth: '256px' } : { maxWidth: '256px' }}
                    >
                      {opt.label}
                    </li>
                  ))}
                </li>
              ))
            : (options as SelectOption[]).map(opt => (
                <li
                  key={opt.value}
                  role="option"
                  aria-selected={opt.value === value}
                  onClick={() => { onChange(opt.value); close() }}
                  className={`select-option ${opt.value === value ? 'select-option-active' : ''}`}
 style={opt.value === value ? { backgroundColor: '#3a3a3a', color: '#f2f2f2', fontWeight: 500, maxWidth: '256px' } : { maxWidth: '256px' }}
                >
                  {opt.label}
                </li>
              ))
          }
        </ul>
      )}
    </div>
  )
}