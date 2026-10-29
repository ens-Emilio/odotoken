import type { GuardEvent, GuardStatus } from './api'

/**
 * Guard reasons arrive as a code plus numbers, never as a sentence: the server
 * meters money and has no idea which language the dashboard is in. Each code
 * names its own limit, so the label is resolved here and the numbers are
 * formatted by the caller.
 */
const LABEL_KEY: Record<string, string> = {
  'guard.dailyCostBreached': 'guard.dailyCost',
  'guard.monthlyCostBreached': 'guard.monthlyCostLimit',
  'guard.dailyCostWarning': 'guard.dailyCost',
  'guard.monthlyCostWarning': 'guard.monthlyCostLimit',
  'guard.dailyTokensBreached': 'guard.dailyTokensLimit',
  'guard.monthlyTokensBreached': 'guard.monthlyTokensLimit',
  'guard.dailyTokensWarning': 'guard.dailyTokensLimit',
  'guard.monthlyTokensWarning': 'guard.monthlyTokensLimit',
}

type Translate = (key: string, params?: Record<string, string>) => string

/** Render one reason code; `null` means the caller should fall back. */
export function guardReasonText(
  t: Translate,
  code: string | undefined,
  params: Record<string, string> | undefined,
): string | null {
  if (!code || !LABEL_KEY[code]) return null
  return t(code, { ...params, label: t(LABEL_KEY[code]) })
}

/** Reason for the current guard state, falling back to the server's prose. */
export function guardStatusReason(t: Translate, status: GuardStatus | undefined): string {
  return guardReasonText(t, status?.reason_code, status?.reason_params) ?? status?.reason ?? ''
}

/** Reason recorded on a past transition. Params are stored as JSON text. */
export function guardEventReason(t: Translate, event: GuardEvent): string {
  let params: Record<string, string> | undefined
  if (event.message_params) {
    try {
      params = JSON.parse(event.message_params) as Record<string, string>
    } catch {
      params = undefined
    }
  }
  return guardReasonText(t, event.message_code, params) ?? event.message
}
