import { useState, useRef, useEffect, Component, type ReactNode } from 'react'
import { useLocale } from './i18n/useLocale'

class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) { return { error } }
  render() {
    if (this.state.error) return <div className="p-5 text-[12px] text-[var(--danger-ink)]">Render error: {this.state.error.message}</div>
    return this.props.children
  }
}

import { AnimatePresence, motion } from 'motion/react'
import { Sidebar } from './Shell'
import { useTheme } from './theme'
import { PALETTES, type PaletteId } from './themes'
import { ToastHost } from './ui/Toast'
import { toast } from './ui/toast-store'
import { Select } from './ui/Select'
import { useFilterOptions, useGuardStatus } from './api'
import { guardStatusReason } from './guard-reason'
import type { ViewId } from './views'
import { OverviewView, ModelsView, ClientsView, EventsView, ThroughputView, GuardView, SettingsView, ImportView, SessionsView, CacheView, SimulateView, PricesView, AnomaliesView, ClientKeysView } from './views'

function View({ id, timeFilter, project, source, tag }: { id: ViewId; timeFilter: string; project: string; source: string; tag: string }) {
  switch (id) {
    case 'overview': return <OverviewView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'models': return <ModelsView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'clients': return <ClientsView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'events': return <EventsView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'throughput': return <ThroughputView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'sessions': return <SessionsView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'cache': return <CacheView timeFilter={timeFilter} project={project} source={source} tag={tag} />
    case 'anomalies': return <AnomaliesView project={project} source={source} tag={tag} />
    case 'simulate': return <SimulateView />
    case 'prices': return <PricesView />
    case 'clientKeys': return <ClientKeysView />
    case 'guard': return <GuardView />
    case 'settings': return <SettingsView />
    case 'import': return <ImportView />
  }
}

export default function App() {
  const [view, setView] = useState<ViewId>(() => {
    const p = new URLSearchParams(location.search)
    return (p.get('view') as ViewId) || (location.hash ? (location.hash.slice(1) as ViewId) : 'overview')
  })
  const initParams = new URLSearchParams(location.search)
  const [timeFilter, setTimeFilter] = useState(() => {
    const value = initParams.get('since')
    return value === 'all' ? 'all' : value || 'today'
  })
  const [project, setProject] = useState(() => initParams.get('project') || 'all')
  const [tag, setTag] = useState(() => initParams.get('tag') || 'all')
  const [source, setSource] = useState(() => initParams.get('source') || 'all')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const theme = useTheme()
  const { t } = useLocale()

 const { data: guardData, error } = useGuardStatus()
 const guardStatus = guardData?.status ?? 'ok'
 const prevStatus = useRef(guardStatus)
 if (error) console.error('Guard status fetch failed:', error)
 const { data: filterOptions } = useFilterOptions(project, source)

  // Toast when guard status transitions into warning/breached
  useEffect(() => {
    if (guardStatus !== prevStatus.current) {
      const reason = guardStatusReason(t, guardData) || t('app.thresholdReached')
      if (guardStatus === 'warning') toast('warn', t('app.guardWarning', { reason }))
      if (guardStatus === 'breached') toast('error', t('app.guardBreached', { reason }))
      prevStatus.current = guardStatus
    }
  }, [guardStatus, guardData, t])

  const changeView = (v: ViewId) => {
    setView(v)
    location.hash = v
    const p = new URLSearchParams(location.search)
    p.set('view', v)
    history.replaceState(null, '', `${location.pathname}?${p.toString()}${location.hash}`)
  }

  // keep URL in sync with filters for deep-links / share
  useEffect(() => {
    const p = new URLSearchParams(location.search)
    if (view !== 'overview') p.set('view', view)
    else p.delete('view')
    if (timeFilter !== 'today') p.set('since', timeFilter)
    else p.delete('since')
    if (project !== 'all') p.set('project', project)
    else p.delete('project')
    if (tag !== 'all') p.set('tag', tag)
    else p.delete('tag')
    if (source !== 'all') p.set('source', source)
    else p.delete('source')
    const qs = p.toString()
    history.replaceState(null, '', `${location.pathname}${qs ? '?' + qs : ''}${location.hash}`)
  }, [view, timeFilter, project, source, tag])

  const viewTitle: Record<ViewId, string> = {
    overview: t('nav.overview'), models: t('nav.models'), clients: t('nav.clients'),
    events: t('nav.events'), throughput: t('nav.throughput'), guard: t('nav.guard'),
    settings: t('nav.settings'), import: t('nav.import'), sessions: t('nav.sessions'),
    cache: t('nav.cache'), simulate: t('nav.simulate'), prices: t('nav.prices'), anomalies: t('nav.anomalies'), clientKeys: t('nav.clientKeys'),
  }

  return (
    <ErrorBoundary>
    <div className="flex min-h-screen">
      <Sidebar view={view} onView={changeView} guardBreached={guardStatus === 'breached' || guardStatus === 'warning'} open={sidebarOpen} onToggle={() => setSidebarOpen((o) => !o)} />
      <div className="min-w-0 flex-1">
        <header className="sticky top-0 z-30 flex items-center justify-between gap-3 border-b border-[var(--border)] bg-[var(--glass-regular)] px-3 py-3 backdrop-blur-xl sm:px-5">
          <div className="flex items-center gap-2">
            <button className="rounded-md p-2 text-[var(--text-muted)] hover:bg-[var(--bg-fill-secondary)] md:hidden" aria-label={t('shell.openNavigation')} onClick={() => setSidebarOpen(true)}>
              <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 12h16M4 18h16"/></svg>
            </button>
            <h2 className="text-[13px] font-semibold text-[var(--text-strong)]">{viewTitle[view]}</h2>
          </div>
          <div className="flex items-center gap-2">
            <div className="relative group">
              <button
                type="button"
                className={`flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                  source !== 'all' || project !== 'all' || tag !== 'all'
                    ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--text-strong)]'
                    : 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-muted)] hover:text-[var(--text-strong)]'
                }`}
                onClick={(e) => {
                  const el = e.currentTarget.nextElementSibling as HTMLElement | null
                  if (el) el.classList.toggle('hidden')
                }}
              >
                <svg className="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 3H2l8 9.46V19l4 2v-8.54L22 3z"/></svg>
                {t('filter.title')}
                {(source !== 'all' || project !== 'all' || tag !== 'all') && (
                  <span className="ms-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--accent)] px-1 text-[9px] font-bold text-[var(--accent-contrast)]">
                    {[source !== 'all', project !== 'all', tag !== 'all'].filter(Boolean).length}
                  </span>
                )}
              </button>
              <div className="absolute end-0 top-full mt-1.5 hidden z-40 w-[280px] max-w-[280px] rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-3 shadow-[var(--shadow-pop)]">
                <div className="space-y-2">
                  <div>
                    <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">{t('filter.source')}</label>
 <Select value={source} onChange={(v) => { setSource(v); if (v === 'all') document.querySelector('.group > div')?.classList.add('hidden') }} dropdownClassName="select-dropdown-constrained" options={[{ value: 'all', label: t('filter.allSources') }, { value: 'proxy', label: t('filter.proxy') }, { value: 'import', label: t('filter.imported') }, ...(filterOptions?.by_source ?? []).map((s) => ({ value: s.source ?? '', label: `${s.source} (${s.requests})` }))]} />
                  </div>
                  <div>
                    <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">{t('filter.project')}</label>
 <Select value={project} onChange={(v) => { setProject(v); if (v === 'all') document.querySelector('.group > div')?.classList.add('hidden') }} dropdownClassName="select-dropdown-constrained" options={[{ value: 'all', label: t('filter.allProjects') }, ...(filterOptions?.by_project ?? []).map((p) => ({ value: p.project_id ?? '', label: `${p.project_id} (${p.requests})` }))]} />
                  </div>
                  <div>
                    <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">{t('filter.tag')}</label>
 <Select value={tag} onChange={(v) => { setTag(v); if (v === 'all') document.querySelector('.group > div')?.classList.add('hidden') }} dropdownClassName="select-dropdown-constrained" options={[{ value: 'all', label: t('filter.allTags') }, ...(filterOptions?.by_tag ?? []).map((t) => ({ value: t.tag ?? '', label: `${t.tag} (${t.requests})` }))]} />
                  </div>
                </div>
              </div>
            </div>
            <div className="flex items-center gap-0.5 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-0.5">
              {(['today', '7d', '30d', '90d', '180d', '365d', 'all'] as const).map((f) => (
                <button
                  key={f}
                  className={`rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                    timeFilter === f
                      ? 'bg-[var(--accent)] text-[var(--accent-contrast)]'
                      : 'text-[var(--text-muted)] hover:bg-[var(--bg-fill-secondary)] hover:text-[var(--text-strong)]'
                  }`}
                  aria-pressed={timeFilter === f}
                  onClick={() => setTimeFilter(f)}
                >
                  {f === 'today' ? t('time.today') : f === '7d' ? t('time.7d') : f === '30d' ? t('time.30d') : f === '90d' ? t('time.90d') : f === '180d' ? t('time.180d') : f === '365d' ? t('time.365d') : t('time.all')}
                </button>
              ))}
            </div>
            <PaletteSelector palette={theme.palette} setPalette={theme.setPalette} />
            <button
              className="rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-2 text-[var(--text-muted)] hover:text-[var(--text-strong)]"
              title={t('shell.theme', { mode: theme.mode })}
              aria-label={t('shell.theme', { mode: theme.mode })}
              onClick={() => theme.setMode(theme.mode === 'dark' ? 'light' : theme.mode === 'light' ? 'system' : 'dark')}
            >
              <ThemeGlyph mode={theme.mode} />
            </button>
          </div>
        </header>

        <AnimatePresence mode="wait">
          <motion.main
            key={view}
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.15 }}
          >
            <View id={view} timeFilter={timeFilter} project={project} source={source} tag={tag} />
          </motion.main>
        </AnimatePresence>
      </div>
      <ToastHost />
    </div>
    </ErrorBoundary>
  )
}

function ThemeGlyph({ mode }: { mode: 'light' | 'dark' | 'system' }) {
  const stroke = { fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round', strokeLinejoin: 'round' } as const
  if (mode === 'dark') return <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" {...stroke}><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
  if (mode === 'light') return <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" {...stroke}><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/></svg>
  return <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" {...stroke}><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>
}

function PaletteSelector({ palette, setPalette }: { palette: PaletteId; setPalette: (id: PaletteId) => void }) {
  const { t } = useLocale()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const current = PALETTES.find(p => p.id === palette) ?? PALETTES[0]

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen(v => !v)}
        className="flex items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-2 py-2 text-[var(--text-muted)] hover:text-[var(--text-strong)]"
        title={t('shell.palette', { name: current.name })}
        aria-label={t('shell.palette', { name: current.name })}
        aria-expanded={open}
      >
        <span className="inline-block h-3.5 w-3.5 rounded-full border border-[var(--border)]" style={{ backgroundColor: current.color }} />
      </button>
      {open && (
        <div className="absolute end-0 top-full z-40 mt-1.5 w-[220px] max-h-[420px] overflow-auto rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-2 shadow-[var(--shadow-pop)]">
          <div className="mb-1.5 px-2 text-[10px] font-semibold uppercase tracking-wider text-[var(--text-faint)]">{t('shell.colorPalette')}</div>
          <div className="space-y-0.5">
            {PALETTES.map((p) => (
              <button
                key={p.id}
                onClick={() => { setPalette(p.id); setOpen(false) }}
                className={`flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-start text-[11px] transition-colors ${
                  palette === p.id
                    ? 'bg-[var(--accent)] text-[var(--accent-contrast)]'
                    : 'text-[var(--text-muted)] hover:bg-[var(--bg-fill-secondary)] hover:text-[var(--text-strong)]'
                }`}
              >
                <span className="inline-block h-3.5 w-3.5 shrink-0 rounded-full border border-[var(--border)]" style={{ backgroundColor: p.color }} />
                <span className="font-medium">{p.name}</span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}