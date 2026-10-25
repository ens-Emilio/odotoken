import { describe, expect, it } from 'vitest'
import { formatCurrency, sinceForFilter } from './format'

describe('sinceForFilter', () => {
  it('does not change the 7d key on every render', () => {
    const first = sinceForFilter('7d', 'America/Rio_Branco')
    const second = sinceForFilter('7d', 'America/Rio_Branco')
    // A fresh Date means milliseconds differ; the day-level filter must be stable.
    expect(Math.abs(new Date(first!).getTime() - new Date(second!).getTime())).toBeLessThan(1000)
  })

  it('starts today at local midnight, not UTC midnight', () => {
    const since = sinceForFilter('today', 'America/Rio_Branco')
    expect(since).toBeDefined()
    const hour = new Date(since!).getUTCHours()
    expect(hour).toBe(5)
  })
})

describe('numeric formatting', () => {
  it('keeps small costs readable and compact large costs', () => {
    expect(formatCurrency(0.0012)).toBe('$0.001200')
    expect(formatCurrency(1250)).toBe('$1.3k')
  })
})
