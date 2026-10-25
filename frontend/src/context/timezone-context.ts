import { createContext } from 'react'

export interface TimezoneContextValue {
  timezone: string
  setTimezone: (tz: string) => void
  timezones: readonly string[]
}

export const TimezoneContext = createContext<TimezoneContextValue | null>(null)
