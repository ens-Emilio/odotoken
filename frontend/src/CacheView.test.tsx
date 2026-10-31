import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import CacheView from './CacheView'
import { mockFetch, renderView } from './test/renderView'

const SAVINGS = {
  requests: 12,
  cache_read_tokens: 1_000_000,
  cache_write_tokens: 0,
  actual_cost_usd: 0.1,
  read_saved_usd: 0.9,
  write_premium_usd: 0,
  net_saved_usd: 0.8,
  uncached_cost_usd: 1.0,
  discount_pct: 80,
  unpriced_models: 0,
  by_model: [
    { model: 'gpt-test', provider: 'openai', requests: 12, cache_read_tokens: 1_000_000, cache_write_tokens: 0, net_saved_usd: 0.8, discount_pct: 80, priced: true },
  ],
}

const view = <CacheView timeFilter="7d" project="all" source="all" tag="all" />

describe('CacheView', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows the empty state when nothing was cached', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/cache-savings': { body: { ...SAVINGS, by_model: [] } } }))
    renderView(view)
    expect(await screen.findByText('No cached traffic in this window.')).toBeInTheDocument()
  })

  it('shows the error state when savings cannot be computed', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/cache-savings': { status: 500, body: {} } }))
    renderView(view)
    expect((await screen.findAllByText('Cache savings are unavailable.')).length).toBeGreaterThan(0)
  })

  it('renders the net saving KPI and the per-model row', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/cache-savings': { body: SAVINGS } }))
    renderView(view)

    expect((await screen.findAllByText('Net saved')).length).toBeGreaterThan(1)
    expect(screen.getAllByText('80%').length).toBeGreaterThan(1)
    expect(screen.getByText('gpt-test')).toBeInTheDocument()
    expect(screen.getByText('openai')).toBeInTheDocument()
  })
})
