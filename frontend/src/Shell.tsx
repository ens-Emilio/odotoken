import { useEffect, useState } from 'react'
import { PanelLeftClose, PanelLeftOpen, Moon, Sun, Monitor } from 'lucide-react'
import { Segmented } from './ui/Segmented'
import type { ViewId } from './views'

type ThemeMode = 'light' | 'dark' | 'system'

const VIEW_META: Record<ViewId, { label: string }> = {
  overview: { label: 'Overview' },
  models: { label: 'Models' },
  clients: { label: 'Client Apps' },
  events: { label: 'Events' },
  guard: { label: 'Guard' },
  settings: { label: 'Settings' },
}

const VIEW_ORDER: ViewId[] = ['overview', 'models', 'clients', 'events', 'guard', 'settings']

function applyTheme(mode: ThemeMode) {
  const dark = mode === 'dark' || (mode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(() => (localStorage.getItem('odt-theme') as ThemeMode) || 'system')
  useEffect(() => {
    applyTheme(mode)
    localStorage.setItem('odt-theme', mode)
    if (mode === 'system') {
      const mq = window.matchMedia('(prefers-color-scheme: dark)')
      const onChange = () => applyTheme('system')
      mq.addEventListener('change', onChange)
      return () => mq.removeEventListener('change', onChange)
    }
  }, [mode])
  return { mode, setMode }
}

export function Sidebar({ view, onView, guardBreached }: { view: ViewId; onView: (v: ViewId) => void; guardBreached: boolean }) {
  const [open, setOpen] = useState(true)

  return (
    <aside
      className={`glass sticky top-0 flex h-screen shrink-0 flex-col border-r transition-[width] duration-200 ${open ? 'w-52' : 'w-14'}`}
    >
      <div className="flex items-center gap-2 px-3 py-4">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--text-strong)] text-[15px] font-bold text-[var(--bg)]">
          ω
        </div>
        {open && (
          <span className="text-[14px] font-semibold tracking-tight text-[var(--text-strong)]">odotoken</span>
        )}
      </div>

      <nav className="flex-1 space-y-0.5 px-2">
        {VIEW_ORDER.map((v) => (
          <button
            key={v}
            onClick={() => onView(v)}
            title={VIEW_META[v].label}
            className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-[12px] font-medium transition-colors ${
              view === v
                ? 'bg-[var(--bg-fill-secondary)] text-[var(--text-strong)]'
                : 'text-[var(--text-muted)] hover:bg-[var(--bg-fill-tertiary)] hover:text-[var(--text-strong)]'
            }`}
          >
            <ViewIcon id={v} active={view === v} />
            {open && <span className="truncate">{VIEW_META[v].label}</span>}
            {open && v === 'guard' && guardBreached && (
              <span className="ml-auto h-1.5 w-1.5 rounded-full bg-[var(--danger)]" />
            )}
          </button>
        ))}
      </nav>

      <button
        className="flex items-center gap-2 rounded-md px-3 py-3 text-[var(--text-soft)] hover:bg-[var(--bg-fill-tertiary)] hover:text-[var(--text-strong)]"
        onClick={() => setOpen((o) => !o)}
      >
        {open ? <PanelLeftClose className="h-4 w-4" /> : <PanelLeftOpen className="h-4 w-4" />}
        {open && <span className="text-[11px]">Collapse</span>}
      </button>
    </aside>
  )
}

function ViewIcon({ id, active }: { id: ViewId; active: boolean }) {
  const cls = `h-4 w-4 shrink-0 ${active ? 'text-[var(--text-strong)]' : ''}`
  switch (id) {
    case 'overview': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/></svg>
    case 'models': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>
    case 'clients': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="4" width="20" height="14" rx="2"/><path d="M6 16v4M18 16v4"/></svg>
    case 'events': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
    case 'guard': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></svg>
    case 'settings': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
  }
}

export function Topbar({ timeFilter, onTimeFilter, theme }: {
  timeFilter: string
  onTimeFilter: (v: string) => void
  theme: { mode: ThemeMode; setMode: (m: ThemeMode) => void }
}) {
  const ThemeIcon = theme.mode === 'dark' ? Moon : theme.mode === 'light' ? Sun : Monitor

  return (
    <header className="sticky top-0 z-30 flex items-center justify-between gap-3 border-b border-[var(--border)] bg-[var(--glass-regular)] px-5 py-3 backdrop-blur-xl">
      <h2 className="text-[13px] font-semibold text-[var(--text-strong)]">
        {VIEW_META[timeFilter as ViewId]?.label ?? ''}
      </h2>
      <div className="flex items-center gap-3">
        <Segmented
          value={timeFilter}
          onChange={onTimeFilter}
          options={[
            { value: 'today', label: 'Today' },
            { value: '7d', label: '7 days' },
            { value: '30d', label: '30 days' },
            { value: 'all', label: 'All' },
          ]}
        />
        <button
          className="rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-2 text-[var(--text-muted)] hover:text-[var(--text-strong)]"
          title={`Theme: ${theme.mode}`}
          onClick={() => theme.setMode(theme.mode === 'dark' ? 'light' : theme.mode === 'light' ? 'system' : 'dark')}
        >
          <ThemeIcon className="h-3.5 w-3.5" />
        </button>
      </div>
    </header>
  )
}