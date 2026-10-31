import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ClientKeysView from './ClientKeysView'
import { mockFetch, renderView } from './test/renderView'

const KEY_ROW = {
  key: 'odt_…a1b2',
  client: 'claude',
  project: 'acme',
  daily_cost_limit_usd: 2.5,
  monthly_cost_limit_usd: 0,
  daily_tokens_limit: 0,
  monthly_tokens_limit: 100000,
}

describe('ClientKeysView', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows the empty state when no key exists', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/client-keys': { body: { keys: [] } } }))
    renderView(<ClientKeysView />)
    expect(await screen.findByText('No client key yet.')).toBeInTheDocument()
  })

  it('shows the error state and retries the listing', async () => {
    const fetchMock = mockFetch({ 'GET /api/client-keys': { status: 500, body: { code: 'error.clientKeys.unavailable' } } })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<ClientKeysView />)
    expect(await screen.findByText('Could not load the client keys.')).toBeInTheDocument()

    fetchMock.mockImplementation(async () => ({ ok: true, status: 200, json: async () => ({ keys: [KEY_ROW] }) }) as Response)
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('odt_…a1b2')).toBeInTheDocument()
  })

  it('lists a masked key with its identity and quota', async () => {
    vi.stubGlobal('fetch', mockFetch({ 'GET /api/client-keys': { body: { keys: [KEY_ROW] } } }))
    renderView(<ClientKeysView />)

    expect(await screen.findByText('odt_…a1b2')).toBeInTheDocument()
    expect(screen.getByText('claude')).toBeInTheDocument()
    expect(screen.getByText('acme')).toBeInTheDocument()
    // A zero quota is inherited, not a limit of zero.
    expect(screen.getAllByText('no limit').length).toBe(2)
  })

  it('creates a key and reveals the secret exactly once', async () => {
    const fetchMock = mockFetch({
      'GET /api/client-keys': { body: { keys: [] } },
      'PUT /api/client-keys': { body: { ...KEY_ROW, key: 'odt_…a1b2', plaintext_key: 'odt_live_secret_value' } },
    })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<ClientKeysView />)
    await screen.findByText('No client key yet.')

    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    const inputs = screen.getAllByRole('textbox')
    await userEvent.type(inputs[0], 'claude')
    await userEvent.type(inputs[1], 'acme')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('odt_live_secret_value')).toBeInTheDocument()
    expect(screen.getByText('Copy the key now')).toBeInTheDocument()
    const put = fetchMock.mock.calls.find(([url, init]) => String(url).endsWith('/api/client-keys') && (init as RequestInit)?.method === 'PUT')
    expect(JSON.parse(String((put?.[1] as RequestInit).body))).toMatchObject({ client: 'claude', project: 'acme' })
  })

  it('revokes a key through the DELETE path', async () => {
    const fetchMock = mockFetch({
      'GET /api/client-keys': { body: { keys: [KEY_ROW] } },
      'DELETE /api/client-keys/odt_…a1b2': { status: 204, body: null },
    })
    vi.stubGlobal('fetch', fetchMock)
    renderView(<ClientKeysView />)
    await screen.findByText('odt_…a1b2')

    await userEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) => String(url).includes('/api/client-keys/odt_') && (init as RequestInit)?.method === 'DELETE')).toBe(true)
    })
  })
})
