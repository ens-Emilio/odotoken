import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SimulateView from './SimulateView'
import { mockFetch, renderView } from './test/renderView'

const ROWS = [
  {
    model: 'gpt-a',
    input_rate_per_1m: 1,
    output_rate_per_1m: 2,
    cache_read_rate_per_1m: 0.1,
    cache_write_rate_per_1m: 1.25,
    per_request_usd: 0.002,
    total_usd: 2,
    delta_vs_baseline_usd: 0,
    priced: true,
  },
  {
    model: 'gpt-b',
    input_rate_per_1m: 5,
    output_rate_per_1m: 10,
    cache_read_rate_per_1m: 0.5,
    cache_write_rate_per_1m: 6.25,
    per_request_usd: 0.01,
    total_usd: 10,
    delta_vs_baseline_usd: 8,
    priced: false,
  },
]

describe('SimulateView', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('asks for a workload before calling the API', async () => {
    const fetchMock = mockFetch({})
    vi.stubGlobal('fetch', fetchMock)
    renderView(<SimulateView />)

    expect(screen.getByText('Enter a workload and simulate.')).toBeInTheDocument()
    // The form starts at one request; zeroing it must not issue a query.
    const requests = screen.getByLabelText('Requests') as HTMLInputElement
    await userEvent.clear(requests)
    await userEvent.type(requests, '0')
    await userEvent.click(screen.getByRole('button', { name: 'Simulate' }))
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('prices the workload across the catalog and marks baseline and unpriced rows', async () => {
    const fetchMock = mockFetch({
      'POST /api/simulate': { body: { baseline: 'gpt-a', rows: ROWS } },
    })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<SimulateView />)

    await userEvent.click(screen.getByRole('button', { name: 'Simulate' }))

    expect(await screen.findByText('gpt-a')).toBeInTheDocument()
    expect(screen.getByText('gpt-b')).toBeInTheDocument()
    expect(screen.getByText('Baseline')).toBeInTheDocument()
    expect(screen.getByText('No price')).toBeInTheDocument()

    const post = fetchMock.mock.calls.find(([url, init]) => String(url).endsWith('/api/simulate') && (init as RequestInit)?.method === 'POST')
    expect(JSON.parse(String((post?.[1] as RequestInit).body))).toMatchObject({
      requests: 1,
      prompt_tokens: 1000,
      completion_tokens: 500,
      cache_read_tokens: 0,
      cache_write_tokens: 0,
    })

    const rowA = screen.getAllByText('gpt-a').map((node) => node.closest('tr')).find(Boolean) as HTMLElement
    const rowB = screen.getAllByText('gpt-b').map((node) => node.closest('tr')).find(Boolean) as HTMLElement
    expect(within(rowA).getAllByText('$2.00').length).toBeGreaterThan(0)
    expect(within(rowB).getByText('+$8.00')).toBeInTheDocument()
  })

  it('shows the failure state when the simulation call fails', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'POST /api/simulate': { status: 400, body: { code: 'error.simulate.negative' } } }))
    renderView(<SimulateView />)

    await userEvent.click(screen.getByRole('button', { name: 'Simulate' }))
    expect(await screen.findByText('The simulation failed.')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })
})
