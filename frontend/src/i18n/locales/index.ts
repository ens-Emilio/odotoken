import type { LocaleCode, LocaleMessages } from '../types'
import { ar } from './ar'
import { de } from './de'
import { en } from '../en'
import { es } from './es'
import { fa } from './fa'
import { fr } from './fr'
import { hi } from './hi'
import { id } from './id'
import { it } from './it'
import { ja } from './ja'
import { ko } from './ko'
import { pl } from './pl'
import { ptBR } from './pt-BR'
import { ru } from './ru'
import { th } from './th'
import { tr } from './tr'
import { uk } from './uk'
import { vi } from './vi'
import { zhCN } from './zh-CN'
import { zhTW } from './zh-TW'

/**
 * English is the fallback for every other locale, so it is complete by
 * construction. The rest are `Partial<Dict>` on purpose: a locale that lags
 * behind a new key degrades one string at a time instead of failing to build.
 */
export const LOCALE_MESSAGES: Readonly<Record<LocaleCode, LocaleMessages>> = {
  en,
  'pt-BR': ptBR,
  es,
  'zh-CN': zhCN,
  'zh-TW': zhTW,
  ja,
  ko,
  de,
  fr,
  it,
  ru,
  uk,
  pl,
  tr,
  hi,
  id,
  vi,
  th,
  ar,
  fa,
}