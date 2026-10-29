import type { LocaleCode, LocaleMessages, MessageParams } from './types'

export const LOCALES: readonly LocaleCode[] = [
  'en', 'pt-BR', 'es', 'zh-CN', 'zh-TW', 'ja', 'ko', 'de', 'fr', 'it',
  'ru', 'uk', 'pl', 'tr', 'hi', 'id', 'vi', 'th', 'ar', 'fa',
] as const

export const LOCALE_NAMES: Readonly<Record<LocaleCode, string>> = {
  en: 'English',
  'pt-BR': 'Português (Brasil)',
  es: 'Español',
  'zh-CN': '简体中文',
  'zh-TW': '繁體中文',
  ja: '日本語',
  ko: '한국어',
  de: 'Deutsch',
  fr: 'Français',
  it: 'Italiano',
  ru: 'Русский',
  uk: 'Українська',
  pl: 'Polski',
  tr: 'Türkçe',
  hi: 'हिन्दी',
  id: 'Bahasa Indonesia',
  vi: 'Tiếng Việt',
  th: 'ไทย',
  ar: 'العربية',
  fa: 'فارسی',
}

export const RTL_LOCALES: Readonly<Partial<Record<LocaleCode, true>>> = { ar: true, fa: true }

/** Match a browser tag (`pt`, `zh-Hant-TW`, `es-MX`) to the closest supported locale. */
export function matchLocale(raw: string | null | undefined): LocaleCode | null {
  if (!raw) return null
  const normalized = raw.trim().replace(/_/g, '-').toLowerCase()
  if (!normalized) return null
  const exact = LOCALES.find((locale) => locale.toLowerCase() === normalized)
  if (exact) return exact

  if (normalized.startsWith('pt')) return 'pt-BR'
  if (normalized.startsWith('zh')) {
    return /(?:hant|tw|hk|mo)/.test(normalized) ? 'zh-TW' : 'zh-CN'
  }
  const language = normalized.split('-')[0]
  return LOCALES.find((locale) => locale.toLowerCase().split('-')[0] === language) ?? null
}

export interface ResolveLocaleInput {
  query?: string
  stored?: string | null
  browser?: readonly string[]
}

/** Resolve UI language without touching browser globals, so it is testable. */
export function resolveLocale({ query = '', stored, browser = [] }: ResolveLocaleInput): LocaleCode {
  const fromQuery = matchLocale(new URLSearchParams(query).get('lang'))
  if (fromQuery) return fromQuery
  const fromStorage = matchLocale(stored)
  if (fromStorage) return fromStorage
  for (const tag of browser) {
    const match = matchLocale(tag)
    if (match) return match
  }
  return 'en'
}

export function directionFor(locale: LocaleCode): 'ltr' | 'rtl' {
  return RTL_LOCALES[locale] ? 'rtl' : 'ltr'
}

export function translate(
  messages: LocaleMessages,
  fallback: LocaleMessages,
  key: string,
  params?: MessageParams,
): string {
  // Both catalogs are `Partial<Dict>`, so a miss is a real possibility: fall
  // back key by key rather than failing the whole locale. An unknown server
  // code must not leak as `error.foo`, so a full miss renders empty and the
  // caller picks the translated generic message.
  const lookup = (catalog: LocaleMessages): string | undefined =>
    (catalog as Readonly<Record<string, string | undefined>>)[key]
  const template = lookup(messages) ?? lookup(fallback)
  if (!template) return ''
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (match: string, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match,
  )
}

/** Select a plural form with Intl rather than a language-specific `n === 1`. */
export function translatePlural(
  locale: LocaleCode,
  forms: Partial<Record<Intl.LDMLPluralRule, string>>,
  count: number,
  params?: MessageParams,
): string {
  const rule = new Intl.PluralRules(locale).select(count)
  const template = forms[rule] ?? forms.other ?? String(count)
  return template.replace(/\{(\w+)\}/g, (match, name: string) => {
    if (name === 'count') return String(count)
    return Object.prototype.hasOwnProperty.call(params ?? {}, name) ? String(params?.[name]) : match
  })
}