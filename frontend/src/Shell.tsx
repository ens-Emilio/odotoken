import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import type { ViewId } from './views'
import { APP_TAGLINE, APP_VERSION } from './version'
import { useLocale } from './i18n/useLocale'

// Keys, not prose: the label is resolved at render so a language change
// repaints the sidebar without rebuilding this table.
const VIEW_LABEL_KEYS: Record<ViewId, string> = {
  overview: 'nav.overview',
  models: 'nav.models',
  clients: 'nav.clients',
  events: 'nav.events',
  throughput: 'nav.throughput',
  guard: 'nav.guard',
  settings: 'nav.settings',
  import: 'nav.import',
  sessions: 'nav.sessions',
  cache: 'nav.cache',
  simulate: 'nav.simulate',
  prices: 'nav.prices',
  anomalies: 'nav.anomalies',
  clientKeys: 'nav.clientKeys',
}
const VIEW_GROUPS: { labelKey: string; ids: ViewId[] }[] = [
  { labelKey: 'nav.monitor', ids: ['overview', 'models', 'clients', 'events', 'throughput', 'sessions', 'cache', 'anomalies'] },
  { labelKey: 'nav.control', ids: ['guard', 'settings', 'import', 'simulate', 'prices', 'clientKeys'] },
]

export function Sidebar({ view, onView, guardBreached, open, onToggle }: { view: ViewId; onView: (v: ViewId) => void; guardBreached: boolean; open: boolean; onToggle: () => void }) {
  const { t } = useLocale()
  const nav = (
    <>
      <div className="flex items-center gap-2 px-3 py-4">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--text-strong)] text-[15px] font-bold text-[var(--bg)]">
          ω
        </div>
        {open && (
          <div className="flex flex-col leading-none">
            <span className="text-[14px] font-semibold tracking-tight text-[var(--text-strong)]">odotoken</span>
            <span className="text-[10px] text-[var(--text-faint)]">v{APP_VERSION} · {APP_TAGLINE}</span>
          </div>
        )}
      </div>

      <nav className="flex-1 space-y-4 px-2 pt-2">
        {VIEW_GROUPS.map((group) => (
          <div key={t(group.labelKey)}>
            {open && <div className="mb-1 px-2.5 text-[10px] font-semibold uppercase tracking-wider text-[var(--text-faint)]">{t(group.labelKey)}</div>}
            <div className="space-y-0.5">
              {group.ids.map((v) => (
                <button
                  key={v}
                  aria-current={view === v ? 'page' : undefined}
                  aria-label={t(VIEW_LABEL_KEYS[v])}
                  onClick={() => {
                    onView(v)
                    if (window.innerWidth < 768 && open) onToggle()
                  }}
                  title={t(VIEW_LABEL_KEYS[v])}
                  className={`relative flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-[12px] font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--brand)] focus:ring-offset-0 ${
                    view === v
                      ? 'bg-[var(--bg-fill-secondary)] text-[var(--text-strong)]'
                      : 'text-[var(--text-muted)] hover:bg-[var(--bg-fill-tertiary)] hover:text-[var(--text-strong)]'
                  }`}
                >
                  <ViewIcon id={v} active={view === v} />
                  {open && <span className="truncate">{t(VIEW_LABEL_KEYS[v])}</span>}
                  {v === 'guard' && guardBreached && (
                    <span className={`h-1.5 w-1.5 rounded-full bg-[var(--danger)] ${open ? 'ms-auto' : 'absolute end-1 top-1'}`} aria-hidden />
                  )}
                </button>
              ))}
            </div>
          </div>
        ))}
      </nav>

      <button
        className="flex items-center gap-2 rounded-md px-3 py-3 text-[var(--text-soft)] hover:bg-[var(--bg-fill-tertiary)] hover:text-[var(--text-strong)] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
        onClick={onToggle}
        aria-label={open ? t('shell.collapseSidebar') : t('shell.expandSidebar')}
        aria-expanded={open}
      >
        {open ? <PanelLeftClose className="h-4 w-4" /> : <PanelLeftOpen className="h-4 w-4" />}
        {open && <span className="text-[11px]">{t('shell.collapse')}</span>}
      </button>
    </>
  )

  return (
    <>
      {/* Desktop */}
      <aside className={`glass sticky top-0 hidden h-screen shrink-0 flex-col border-r transition-[width] duration-200 md:flex ${open ? 'w-52' : 'w-14'}`}>
        {nav}
      </aside>

      {/* Mobile overlay */}
      {open && (
        <div className="fixed inset-0 z-40 flex md:hidden" aria-modal="true" role="dialog">
          <button type="button" aria-label={t('shell.closeSidebar')} className="absolute inset-0 bg-black/30 backdrop-blur-sm" onClick={onToggle} />
          <aside className="glass relative flex h-full w-64 shrink-0 flex-col border-r shadow-[var(--shadow-pop)]">
            {nav}
          </aside>
        </div>
      )}
    </>
  )
}

function ViewIcon({ id, active }: { id: ViewId; active: boolean }) {
  const cls = `h-4 w-4 shrink-0 ${active ? 'text-[var(--text-strong)]' : ''}`
  switch (id) {
    case 'overview': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/></svg>
    case 'models': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>
    case 'clients': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="4" width="20" height="14" rx="2"/><path d="M6 16v4M18 16v4"/></svg>
    case 'events': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
    case 'throughput': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 17l6-6 4 4 8-8"/><path d="M14 7h7v7"/></svg>
    case 'guard': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></svg>
    case 'import': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 3v12m0 0 4-4m-4 4-4-4"/><path d="M5 20h14"/></svg>
    case 'sessions': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 15a2 2 0 0 1-2 2H8l-5 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
    case 'cache': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.66 3.58 3 8 3s8-1.34 8-3V5"/><path d="M4 11v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6"/></svg>
    case 'simulate': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/></svg>
    case 'prices': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20.59 13.41 13.42 20.58a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><circle cx="7" cy="7" r="1.2"/></svg>
    case 'clientKeys': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M15.5 7.5a4.5 4.5 0 1 1-6.36 0"/><circle cx="12" cy="12" r="3"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2"/></svg>
    case 'anomalies': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><path d="M12 9v4M12 17h.01"/></svg>
    case 'settings': return <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0 1.51 1H21z"/></svg>
  }
}
