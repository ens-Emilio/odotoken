import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { LocaleContext } from './context'
import { directionFor, resolveLocale, translate } from './core'
import { en } from './en'
import { LOCALE_MESSAGES } from './locales'
import type { LocaleCode, MessageParams } from './types'

const STORAGE_KEY = 'odt-lang'

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<LocaleCode>(() => {
    let stored: string | null = null
    try {
      stored = localStorage.getItem(STORAGE_KEY)
    } catch {
      // Private-mode storage denial must not break rendering.
    }
    return resolveLocale({
      query: location.search,
      stored,
      browser: navigator.languages?.length ? navigator.languages : [navigator.language],
    })
  })

  useEffect(() => {
    document.documentElement.lang = locale
    document.documentElement.dir = directionFor(locale)
    // The tab title is part of the product language, not chrome.
    document.title = translate(LOCALE_MESSAGES[locale] ?? {}, en, 'doc.title')
    try {
      localStorage.setItem(STORAGE_KEY, locale)
    } catch {
      // Same denial as above: persistence is best effort.
    }
  }, [locale])

  const setLocale = useCallback((next: LocaleCode) => {
    setLocaleState(next)
    // A `?lang=` link pins one language; choosing another in Settings has to
    // win, otherwise the selector looks broken on a shared deep link.
    const params = new URLSearchParams(location.search)
    if (params.has('lang')) {
      params.set('lang', next)
      history.replaceState(null, '', `${location.pathname}?${params.toString()}${location.hash}`)
    }
  }, [])

  const value = useMemo(() => {
    const messages = LOCALE_MESSAGES[locale] ?? {}
    return {
      locale,
      direction: directionFor(locale),
      messages,
      setLocale,
      t: (key: string, params?: MessageParams) => translate(messages, en, key, params),
    }
  }, [locale, setLocale])

  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
}