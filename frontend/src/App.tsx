import { useState, useEffect, useRef } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { Sidebar, useTheme } from './Shell'
import { ToastHost, toast } from './ui/Toast'
import { useStats } from './api'
import type { ViewId } from './views'
import { OverviewView, ModelsView, ClientsView, EventsView, GuardView, SettingsView } from './views'

function View({ id, timeFilter, project, source }: { id: ViewId; timeFilter: string; project: string; source: string }) {
  switch (id) {
    case 'overview': return <OverviewView timeFilter={timeFilter} project={project} source={source} />
    case 'models': return <ModelsView timeFilter={timeFilter} project={project} source={source} />
    case 'clients': return <ClientsView timeFilter={timeFilter} project={project} source={source} />
    case 'events': return <EventsView timeFilter={timeFilter} project={project} source={source} />
    case 'guard': return <GuardView />
    case 'settings': return <SettingsView />
  }
}

export default function App() {
  const [view, setView] = useState<ViewId>(() => {
    const p = new URLSearchParams(location.search)
    return (p.get('view') as ViewId) || (location.hash ? (location.hash.slice(1) as ViewId) : 'overview')
  })
  const initParams = new URLSearchParams(location.search)
  const [timeFilter, setTimeFilter] = useState(() => initParams.get('since') || 'today')
  const [project, setProject] = useState(() => initParams.get('project') || 'all')
  const [source, setSource] = useState(() => initParams.get('source') || 'all')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const theme = useTheme()


  const { data } = useStats(undefined, project, source)
  const guardStatus = data?.guard.status ?? 'ok'
  const prevStatus = useRef(guardStatus)

  // Toast when guard status transitions into warning/breached
  useEffect(() => {
    if (guardStatus !== prevStatus.current) {
      if (guardStatus === 'warning') toast('warn', 'Budget warning: ' + (data?.guard.reason || 'threshold reached'))
      if (guardStatus === 'breached') toast('error', 'Budget limit breached: ' + (data?.guard.reason || ''))
      prevStatus.current = guardStatus
    }
  }, [guardStatus, data?.guard.reason])

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
    if (source !== 'all') p.set('source', source)
    else p.delete('source')
    const qs = p.toString()
    history.replaceState(null, '', `${location.pathname}${qs ? '?' + qs : ''}${location.hash}`)
  }, [view, timeFilter, project, source])

  const viewTitle: Record<ViewId, string> = {
    overview: 'Overview',
    models: 'Models',
    clients: 'Client Apps',
    events: 'Events',
    guard: 'Guard',
    settings: 'Settings',
  }

  return (
    <div className="flex min-h-screen">
      <Sidebar view={view} onView={changeView} guardBreached={guardStatus === 'breached' || guardStatus === 'warning'} open={sidebarOpen} onToggle={() => setSidebarOpen((o) => !o)} />
      <div className="min-w-0 flex-1">
        <header className="sticky top-0 z-30 flex items-center justify-between gap-3 border-b border-[var(--border)] bg-[var(--glass-regular)] px-3 py-3 backdrop-blur-xl sm:px-5">
          <div className="flex items-center gap-2">
            <button className="rounded-md p-2 text-[var(--text-muted)] hover:bg-[var(--bg-fill-secondary)] md:hidden" aria-label="Open navigation" onClick={() => setSidebarOpen(true)}>
              <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 12h16M4 18h16"/></svg>
            </button>
            <h2 className="text-[13px] font-semibold text-[var(--text-strong)]">{viewTitle[view]}</h2>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2 sm:gap-3">
            <span className="hidden text-[11px] font-medium text-[var(--text-soft)] md:inline">Time</span>
            <div className="flex items-center gap-0.5 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-0.5">
              {(['today', '7d', '30d', 'all'] as const).map((f) => (
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
                  {f === 'today' ? 'Today' : f === '7d' ? '7 days' : f === '30d' ? '30 days' : 'All'}
                </button>
              ))}
            </div>
            <span className="hidden text-[11px] font-medium text-[var(--text-soft)] md:inline">Source</span>
            <label className="sr-only" htmlFor="source-filter">Source filter</label>
            <select id="source-filter" value={source} onChange={(e) => setSource(e.target.value)} className="max-w-[150px] rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1.5 text-[11px] font-medium text-[var(--text-muted)]">
              <option value="all">All sources</option>
              <option value="proxy">Proxy</option>
              <option value="import">Imported</option>
              {(data?.by_source ?? []).map((s: any) => (
                <option key={s.source} value={s.source}>{s.source} ({s.requests})</option>
              ))}
            </select>
            <span className="hidden text-[11px] font-medium text-[var(--text-soft)] md:inline">Project</span>
            <label className="sr-only" htmlFor="project-filter">Project filter</label>
            <select
              id="project-filter"
              value={project}
              onChange={(e) => setProject(e.target.value)}
              className="max-w-[140px] rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1.5 text-[11px] font-medium text-[var(--text-muted)]"
            >
              <option value="all">All projects</option>
              {(data?.by_project ?? []).map((p) => (
                <option key={p.project_id} value={p.project_id}>
                  {p.project_id} ({p.requests})
                </option>
              ))}
            </select>
            <button
              className="rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-2 text-[var(--text-muted)] hover:text-[var(--text-strong)]"
              title={`Theme: ${theme.mode}`}
              aria-label={`Theme: ${theme.mode}`}
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
            <View id={view} timeFilter={timeFilter} project={project} source={source} />
          </motion.main>
        </AnimatePresence>
      </div>
      <ToastHost />
    </div>
  )
}

function ThemeGlyph({ mode }: { mode: 'light' | 'dark' | 'system' }) {
  const stroke = { fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round', strokeLinejoin: 'round' } as const
  if (mode === 'dark') return <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" {...stroke}><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
  if (mode === 'light') return <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" {...stroke}><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/></svg>
  return <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" {...stroke}><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>
}