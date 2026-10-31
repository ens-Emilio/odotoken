import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SessionsView from './SessionsView'
import { mockFetch, renderView } from './test/renderView'

const SESSION = {
  session_id: 'conv-1',
  session_file: '',
  client_app: 'claude',
  project_id: 'acme',
  models: 'gpt-test',
  requests: 2,
  prompt_tokens: 1000,
  completion_tokens: 500,
  total_tokens: 1500,
  cache_read_tokens: 400,
  cache_write_tokens: 0,
  cost_usd: 0.2,
  started_at: '2026-10-28T10:00:00Z',
  ended_at: '2026-10-28T10:05:00Z',
}

const EVENT = {
  id: 1,
  timestamp: '2026-10-28T10:00:00Z',
  provider: 'openai',
  model: 'gpt-test',
  client_app: 'claude',
  project_id: 'acme',
  session_id: 'conv-1',
  source: 'proxy',
  status_code: 200,
  prompt_tokens: 500,
  completion_tokens: 250,
  total_tokens: 750,
  cache_read_tokens: 200,
  cache_write_tokens: 0,
  cost_usd: 0.1,
  latency_ms: 120,
  ttfb_ms: 40,
}

describe('SessionsView', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows the empty state for a window without conversations', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/sessions': { body: [] } }))
    renderView(<SessionsView timeFilter="7d" project="all" source="all" tag="all" />)
    expect(await screen.findByText('No conversations in this window.')).toBeInTheDocument()
  })

  it('shows the error state when the listing fails', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/sessions': { status: 500, body: {} } }))
    renderView(<SessionsView timeFilter="7d" project="all" source="all" tag="all" />)
    expect((await screen.findAllByText('Conversations are unavailable.')).length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })

  it('rolls a conversation up and opens its events', async () => {
    vi.stubGlobal('fetch', mockFetch({
      'GET /api/sessions': { body: [SESSION] },
      'GET /api/sessions/conv-1': { body: { session_id: 'conv-1', events: [EVENT] } },
    }))
    renderView(<SessionsView timeFilter="7d" project="all" source="all" tag="all" />)

    expect(await screen.findByText('conv-1')).toBeInTheDocument()
    expect(screen.getByText('claude')).toBeInTheDocument()
    await userEvent.click(screen.getByText('conv-1'))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Session conv-1')).toBeInTheDocument()
    expect(await within(dialog).findByText('gpt-test')).toBeInTheDocument()
  })
})
