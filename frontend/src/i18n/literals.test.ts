import { describe, expect, it } from 'vitest'

/**
 * The plan's acceptance rule: no visible prose left as a JSX literal.
 *
 * This is a regression guard, not a parser. It reads every UI source through
 * Vite's raw glob (so it needs no filesystem access) and flags text nodes and
 * user-facing attributes whose value is a bare English sentence — the shape
 * extraction leaves behind. Identifiers, symbols, numbers and class names are
 * data, not prose.
 */

// Two or more words starting with a capital, so `‹`, `—`, `1m`, `∞`, `UTC-5`
// and `ms` stay out. JSX children in braces never match because of the braces.
const PROSE = /^[A-Z][A-Za-z'’-]*(?: [A-Za-z'’(),.:/+-]+)+$/

const ATTRIBUTES = ['placeholder', 'title', 'aria-label', 'alt', 'download', 'hint']

function visibleLiterals(source: string): string[] {
  const found: string[] = []
  for (const match of source.matchAll(/>\s*([A-Za-z][^<>{}]*?)\s*</g)) {
    const text = match[1].trim()
    if (PROSE.test(text)) found.push(`text: ${text}`)
  }
  for (const name of ATTRIBUTES) {
    for (const match of source.matchAll(new RegExp(`${name}="([^"]+)"`, 'g'))) {
      if (PROSE.test(match[1])) found.push(`${name}: ${match[1]}`)
    }
  }
  return found
}

const SOURCES = import.meta.glob(['../**/*.tsx', '!../i18n/**'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

describe('no visible literal escapes the catalog', () => {
  it('finds the UI sources it is meant to scan', () => {
    expect(Object.keys(SOURCES).length).toBeGreaterThan(10)
  })

  it('keeps JSX text and user-facing attributes behind t()', () => {
    const offenders = Object.entries(SOURCES).flatMap(([file, source]) =>
      visibleLiterals(source).map((literal) => `${file}: ${literal}`),
    )
    expect(offenders).toEqual([])
  })
})
