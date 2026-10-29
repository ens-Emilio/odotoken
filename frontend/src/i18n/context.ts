import { createContext } from 'react'
import type { LocaleCode, LocaleMessages, MessageParams } from './types'

export interface LocaleContextValue {
  locale: LocaleCode
  direction: 'ltr' | 'rtl'
  messages: LocaleMessages
  setLocale: (locale: LocaleCode) => void
  t: (key: string, params?: MessageParams) => string
}

export const LocaleContext = createContext<LocaleContextValue | null>(null)