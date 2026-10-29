import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import { LocaleProvider } from './i18n/LocaleProvider'
import { TimezoneProvider } from './context/TimezoneContext'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchInterval: 10_000,
      staleTime: 5_000,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <LocaleProvider>
      <TimezoneProvider>
        <QueryClientProvider client={queryClient}>
          <App />
        </QueryClientProvider>
      </TimezoneProvider>
    </LocaleProvider>
  </StrictMode>
)