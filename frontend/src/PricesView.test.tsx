import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import PricesView from './PricesView'
import { mockFetch, renderView } from './test/renderView'

const OVERRIDE = { model: 'gpt-a', input: 1, output: 2, cache_read: 0.1, cache_write: 1.25, explicit: true }
const BUILTIN = { model: 'gpt-b', input: 5, output: 10, cache_read: 0.5, cache_write: 6.25, explicit: false }

describe('PricesView', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows the empty state when the catalog matches nothing', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/prices': { body: { path: 'prices.json', prices: [] } } }))
    renderView(<PricesView />)
    expect(await screen.findByText('No model matches.')).toBeInTheDocument()
  })

  it('shows the error state and retries the catalog', async () => {
    const fetchMock = mockFetch({ 'GET /api/prices': { status: 500, body: { code: 'error.prices.unreadable' } } })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<PricesView />)

    expect(await screen.findByText('The price catalog is unavailable.')).toBeInTheDocument()

    fetchMock.mockImplementation(async () => ({ ok: true, status: 200, json: async () => ({ path: 'prices.json', prices: [OVERRIDE] }) }) as Response)
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('gpt-a')).toBeInTheDocument()
  })

  it('separates stored overrides from built-in fallbacks', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/prices': { body: { path: 'prices.json', prices: [OVERRIDE, BUILTIN] } } }))
    renderView(<PricesView />)

    expect(await screen.findByText('gpt-a')).toBeInTheDocument()
    expect(screen.getAllByText('prices.json').length).toBeGreaterThan(0)
    expect(screen.getByText('Built-in')).toBeInTheDocument()
    // Only a stored override can be removed; a built-in rate has no Delete.
    expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(1)
  })

  it('saves edited rates through the PUT endpoint', async () => {
    const fetchMock = mockFetch({
      'GET /api/prices': { body: { path: 'prices.json', prices: [OVERRIDE] } },
      'PUT /api/prices/gpt-a': { body: { ...OVERRIDE, input: 3 } },
    })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<PricesView />)
    await screen.findByText('gpt-a')

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    const dialog = await screen.findByRole('dialog')
    const input = within(dialog).getAllByRole('spinbutton')[0]
    await userEvent.clear(input)
    await userEvent.type(input, '3')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      const put = fetchMock.mock.calls.find(([url, init]) => String(url).endsWith('/api/prices/gpt-a') && (init as RequestInit)?.method === 'PUT')
      expect(put).toBeTruthy()
      expect(JSON.parse(String((put?.[1] as RequestInit).body))).toMatchObject({ input: 3, output: 2, cache_read: 0.1, cache_write: 1.25 })
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('reports a rejected price without closing the editor', async () => {
    vi.stubGlobal('fetch', mockFetch({
      'GET /api/prices': { body: { path: 'prices.json', prices: [OVERRIDE] } },
      'PUT /api/prices/gpt-a': { status: 400, body: { code: 'error.prices.invalid' } },
    }))
    renderView(<PricesView />)
    await screen.findByText('gpt-a')

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  })

  it('removes an override through the DELETE endpoint', async () => {
    const fetchMock = mockFetch({
      'GET /api/prices': { body: { path: 'prices.json', prices: [OVERRIDE] } },
      'DELETE /api/prices/gpt-a': { status: 204, body: null },
    })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<PricesView />)
    await screen.findByText('gpt-a')

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) => String(url).endsWith('/api/prices/gpt-a') && (init as RequestInit)?.method === 'DELETE')).toBe(true)
    })
  })
})
