import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'

const STORAGE_KEY = 'odotoken:timezone'
const DEFAULT_TZ = 'America/Rio_Branco'

// ponytail: computed once at module load; Intl.supportedValuesOf is stable per runtime
const TIMEZONES = Intl.supportedValuesOf('timeZone')

interface TimezoneContextValue {
  timezone: string
  setTimezone: (tz: string) => void
  timezones: readonly string[]
}

const TimezoneContext = createContext<TimezoneContextValue | null>(null)

export function TimezoneProvider({ children }: { children: ReactNode }) {
  const [timezone, setTimezoneState] = useState(DEFAULT_TZ)

  // Load persisted timezone on mount
  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && TIMEZONES.includes(stored)) {
      setTimezoneState(stored)
    }
  }, [])

  // Persist on change
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, timezone)
  }, [timezone])

  const setTimezone = (tz: string) => {
    if (!TIMEZONES.includes(tz)) return
    setTimezoneState(tz)
  }

  return (
    <TimezoneContext.Provider value={{ timezone, setTimezone, timezones: TIMEZONES }}>
      {children}
    </TimezoneContext.Provider>
  )
}

export function useTimezone(): TimezoneContextValue {
  const ctx = useContext(TimezoneContext)
  if (!ctx) throw new Error('useTimezone must be used within TimezoneProvider')
  return ctx
}