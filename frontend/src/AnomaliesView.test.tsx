import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AnomaliesView from './AnomaliesView'
import { mockFetch, renderView } from './test/renderView'

const ANOMALY = {
  day: '2026-10-27',
  cost_usd: 42.5,
  threshold: 10,
  z_score: 3.4,
  by_model: [
    { key: 'gpt-a', cost_usd: 30, baseline_usd: 5, delta_usd: 25, share_pct: 76.9 },
    { key: 'gpt-b', cost_usd: 12.5, baseline_usd: 5, delta_usd: 7.5, share_pct: 23.1 },
  ],
  by_project: [
    { key: 'acme', cost_usd: 42.5, baseline_usd: 10, delta_usd: 32.5, share_pct: 100 },
  ],
}

function render(project = 'all') {
  return renderView(<AnomaliesView project={project} source="all" tag="all" />)
}

describe('AnomaliesView', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows the empty state when no day crossed the threshold', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/anomalies': { body: { anomalies: [], generated_at: '2026-10-29T00:00:00Z' } } }))
    render()
    expect(await screen.findByText('No anomalous day in the last 45 days.')).toBeInTheDocument()
  })

  it('shows the error state and retries the listing', async () => {
    const fetchMock = mockFetch({ 'GET /api/anomalies': { status: 500, body: { code: 'error.stats.unavailable' } } })
    vi.stubGlobal('fetch', fetchMock)
    render()

    expect(await screen.findByText('Anomalies are unavailable.')).toBeInTheDocument()

    fetchMock.mockImplementation(async () => ({ ok: true, status: 200, json: async () => ({ anomalies: [ANOMALY], generated_at: '2026-10-29T00:00:00Z' }) }) as Response)
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('2026-10-27')).toBeInTheDocument()
  })

  it('attributes the day to the models and projects that moved', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/anomalies': { body: { anomalies: [ANOMALY], generated_at: '2026-10-29T00:00:00Z' } } }))
    render()

    expect(await screen.findByText('2026-10-27')).toBeInTheDocument()
    expect(screen.getByText('3.40σ')).toBeInTheDocument()
    expect(screen.getByText('By model')).toBeInTheDocument()
    expect(screen.getByText('By project')).toBeInTheDocument()
    expect(screen.getByText('76.9%')).toBeInTheDocument()
    expect(screen.getByText('+$25.00')).toBeInTheDocument()
    expect(screen.getByText('acme')).toBeInTheDocument()
  })

  it('passes the active project filter to the API', async () => {
    const fetchMock = mockFetch({ 'GET /api/anomalies': { body: { anomalies: [], generated_at: '2026-10-29T00:00:00Z' } } })
    vi.stubGlobal('fetch', fetchMock)
    render('acme')

    await screen.findByText('No anomalous day in the last 45 days.')
    const url = String(fetchMock.mock.calls[0]?.[0])
    expect(url).toContain('project=acme')
  })
})
