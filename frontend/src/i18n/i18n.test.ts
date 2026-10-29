import { describe, expect, it } from 'vitest'
import { LOCALES, directionFor, matchLocale, resolveLocale, translate, translatePlural } from './core'
import { en } from './en'
import { LOCALE_MESSAGES } from './locales'

describe('locale resolution', () => {
  it('prefers ?lang= over storage and browser', () => {
    expect(resolveLocale({ query: '?lang=de', stored: 'ja', browser: ['ru'] })).toBe('de')
  })

  it('falls back to storage, then browser, then English', () => {
    expect(resolveLocale({ stored: 'ja', browser: ['ru'] })).toBe('ja')
    expect(resolveLocale({ browser: ['ru-RU'] })).toBe('ru')
    expect(resolveLocale({})).toBe('en')
  })

  it('ignores an unsupported ?lang= instead of showing a broken locale', () => {
    expect(resolveLocale({ query: '?lang=xx', stored: 'es' })).toBe('es')
  })

  it('matches language tags by prefix and script', () => {
    expect(matchLocale('pt')).toBe('pt-BR')
    expect(matchLocale('pt_PT')).toBe('pt-BR')
    expect(matchLocale('zh')).toBe('zh-CN')
    expect(matchLocale('zh-Hant-TW')).toBe('zh-TW')
    expect(matchLocale('zh-HK')).toBe('zh-TW')
    expect(matchLocale('es-MX')).toBe('es')
    expect(matchLocale(null)).toBeNull()
  })

  it('gives Arabic and Persian a right-to-left direction', () => {
    expect(directionFor('ar')).toBe('rtl')
    expect(directionFor('fa')).toBe('rtl')
    expect(directionFor('he' as never)).toBe('ltr')
    expect(directionFor('pt-BR')).toBe('ltr')
  })
})

describe('catalog', () => {
  it('ships exactly the twenty planned locales', () => {
    expect(LOCALES).toHaveLength(20)
    expect(new Set(LOCALES).size).toBe(20)
    for (const locale of LOCALES) expect(LOCALE_MESSAGES[locale]).toBeDefined()
  })

  it('translates a known key and falls back to English for a missing one', () => {
    const partial = { 'nav.overview': 'Visão geral' }
    expect(translate(partial, en, 'nav.overview')).toBe('Visão geral')
    expect(translate(partial, en, 'nav.models')).toBe(en['nav.models'])
  })

  it('never renders a raw key when the locale is incomplete', () => {
    for (const locale of LOCALES) {
      const rendered = translate(LOCALE_MESSAGES[locale] ?? {}, en, 'error.generic')
      expect(rendered).not.toBe('error.generic')
      expect(rendered.length).toBeGreaterThan(0)
    }
  })

  it('interpolates named parameters and leaves unknown ones untouched', () => {
    expect(translate(en, en, 'overview.peak', { value: '1.2k tok/s' })).toBe('peak 1.2k tok/s')
    expect(translate(en, en, 'overview.peak', {})).toBe('peak {value}')
  })

  it('selects plural forms with Intl instead of n === 1', () => {
    const rows = { one: '{count} row', other: '{count} rows' }
    expect(translatePlural('en', rows, 1)).toBe('1 row')
    expect(translatePlural('en', rows, 0)).toBe('0 rows')
    expect(translatePlural('en', rows, 21)).toBe('21 rows')

    const polish = { one: 'wiersz', few: 'wiersze', many: 'wierszy', other: 'wiersza' }
    expect(translatePlural('pl', polish, 1)).toBe('wiersz')
    expect(translatePlural('pl', polish, 3)).toBe('wiersze')
    expect(translatePlural('pl', polish, 5)).toBe('wierszy')

    const arabic = { zero: 'صفري', one: 'واحد', two: 'اثنان', few: 'قليل', many: 'كثير', other: 'آخر' }
    expect(translatePlural('ar', arabic, 0)).toBe('صفري')
    expect(translatePlural('ar', arabic, 2)).toBe('اثنان')
  })
})
describe('catalog completeness', () => {
  it('defines every English key in every locale', () => {
    const englishKeys = Object.keys(en)
    for (const locale of LOCALES) {
      const messages = LOCALE_MESSAGES[locale] ?? {}
      const missing = englishKeys.filter((key) => !(key in messages))
      expect({ locale, missing }).toEqual({ locale, missing: [] })
    }
  })

  it('keeps every placeholder used by English in each translation', () => {
    const placeholders = (value: string) => (value.match(/\{(\w+)\}/g) ?? []).sort().join(',')
    for (const locale of LOCALES) {
      const messages = LOCALE_MESSAGES[locale] ?? {}
      for (const [key, value] of Object.entries(en)) {
        const translated = messages[key as keyof typeof messages]
        if (translated === undefined) continue
        expect({ locale, key, placeholders: placeholders(translated) }).toEqual({
          locale,
          key,
          placeholders: placeholders(value),
        })
      }
    }
  })
})
