import { useEffect, useState, type ReactNode } from 'react'
import { TimezoneContext } from './timezone-context'

const STORAGE_KEY = 'odotoken:timezone'
const DEFAULT_TZ = 'America/Rio_Branco'

// ponytail: computed once at module load; Intl.supportedValuesOf is stable per runtime
const TIMEZONES = Intl.supportedValuesOf('timeZone')

export function TimezoneProvider({ children }: { children: ReactNode }) {
  const [timezone, setTimezoneState] = useState(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored && TIMEZONES.includes(stored) ? stored : DEFAULT_TZ
  })

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
