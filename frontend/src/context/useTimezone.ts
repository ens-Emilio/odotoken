import { useContext } from 'react'
import { TimezoneContext, type TimezoneContextValue } from './timezone-context'

export function useTimezone(): TimezoneContextValue {
  const ctx = useContext(TimezoneContext)
  if (!ctx) throw new Error('useTimezone must be used within TimezoneProvider')
  return ctx
}