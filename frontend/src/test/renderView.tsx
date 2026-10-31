import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { vi } from 'vitest'
import { LocaleProvider } from '../i18n/LocaleProvider'
import { TimezoneProvider } from '../context/TimezoneContext'

export function renderView(ui: ReactNode) {
  localStorage.setItem('odt-lang', 'en')
  localStorage.setItem('odotoken:timezone', 'UTC')
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
  return render(
    <LocaleProvider>
      <TimezoneProvider>
        <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
      </TimezoneProvider>
    </LocaleProvider>,
  )
}

type FetchBody = unknown

export function mockFetch(routes: Record<string, { status?: number; body: FetchBody }>) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url
    const url = new URL(raw, 'http://localhost')
    const method = init?.method ?? (input instanceof Request ? input.method : 'GET')
    const key = `${method} ${url.pathname}`
    const route = routes[key]
    if (!route) throw new Error(`No fetch mock for ${key}`)
    return {
      ok: (route.status ?? 200) >= 200 && (route.status ?? 200) < 300,
      status: route.status ?? 200,
      json: async () => route.body,
    } as Response
  })
}