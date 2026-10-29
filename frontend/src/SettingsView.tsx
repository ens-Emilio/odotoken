import { useMemo } from 'react'
import { Select, type SelectGroup } from './ui/Select'
import { Card, CardHeader, Badge } from './ui/Card'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'
import { LOCALES, LOCALE_NAMES } from './i18n/core'
import { APP_VERSION } from './version'

function getOffsetString(tz: string): string {
  try {
    const formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: tz,
      timeZoneName: 'shortOffset',
    })
    const parts = formatter.formatToParts(new Date())
    const tzPart = parts.find(p => p.type === 'timeZoneName')
    if (tzPart) {
      return tzPart.value.replace('GMT', 'UTC')
    }
  } catch {}
  return ''
}

export default function SettingsView() {
  const { timezone, setTimezone } = useTimezone()
  const { locale, setLocale, t } = useLocale()

  const groupedOptions: SelectGroup[] = useMemo(() => {
    const zones = Intl.supportedValuesOf ? Intl.supportedValuesOf('timeZone') : []
    const groups: Record<string, string[]> = {}
    for (const z of zones) {
      const region = z.split('/')[0] || 'Other'
      if (!groups[region]) groups[region] = []
      groups[region].push(z)
    }
    return Object.entries(groups).map(([label, opts]) => ({
      label,
      options: opts.map(value => ({ value, label: value })),
    }))
  }, [])

  // Native names, never the name translated into the current language: a
  // reader scanning for their own language recognises `Português`, not
  // "Portuguese".
  const languageOptions = useMemo(
    () => LOCALES.map((code) => ({ value: code, label: LOCALE_NAMES[code] })),
    [],
  )

  const currentOffset = getOffsetString(timezone)
  const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader title={t('settings.timezone')} subtitle={t('settings.timezoneSub')} />
        <div className="space-y-3 px-5 pb-5 text-[12px]">
          <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
            <label htmlFor="tz-select" className="w-24 shrink-0 text-[var(--text-muted)]">{t('settings.timezoneLabel')}</label>
            <Select
              id="tz-select"
              value={timezone}
              onChange={setTimezone}
              options={groupedOptions}
            />
            <Badge>{currentOffset}</Badge>
          </div>
          <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
            <span className="w-24 shrink-0" />
            <button
              onClick={() => setTimezone(browserTz)}
              className="rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-[11px] font-medium text-[var(--text-muted)] hover:text-[var(--text-strong)]"
            >
              {t('settings.detectFromBrowser', { tz: browserTz })}
            </button>
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title={t('settings.language')} subtitle={t('settings.languageSub')} />
        <div className="space-y-3 px-5 pb-5 text-[12px]">
          <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
            <label htmlFor="lang-select" className="w-24 shrink-0 text-[var(--text-muted)]">{t('settings.language')}</label>
            <Select
              id="lang-select"
              value={locale}
              onChange={(value) => setLocale(value as typeof locale)}
              options={languageOptions}
            />
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title={t('settings.service')} subtitle={t('settings.serviceSub')} />
        <div className="space-y-2 px-5 pb-5 text-[12px]">
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">{t('settings.endpoint')}</span>
            <span className="num text-[var(--text-strong)]">{window.location.host}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">{t('settings.version')}</span>
            <Badge>{APP_VERSION}</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">{t('settings.refresh')}</span>
            <Badge tone="brand">{t('settings.every10s')}</Badge>
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title={t('settings.configuration')} subtitle={t('settings.configurationSub')} />
        <div className="space-y-2 px-5 pb-5 text-[12px]">
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">{t('settings.configFile')}</span>
            <span className="num text-[var(--text-strong)]">config.json</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">{t('settings.database')}</span>
            <span className="num text-[var(--text-strong)]">odotoken.db (SQLite WAL)</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">{t('settings.prices')}</span>
            <span className="num text-[var(--text-strong)]">prices.json</span>
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title={t('settings.manage')} subtitle={t('settings.manageSub')} />
        <pre className="mx-5 mb-5 overflow-auto rounded-md border border-[var(--border-soft)] bg-[var(--bg)] p-3 font-mono text-[11px] leading-relaxed text-[var(--text-muted)]">
{`systemctl --user status odotoken
systemctl --user restart odotoken
systemctl --user stop odotoken

# terminal stats
odotoken stats --by-model
odotoken stats --by-client
odotoken log --limit 20`}
        </pre>
      </Card>

      <Card>
        <CardHeader title={t('settings.routedProviders')} subtitle={t('settings.routedProvidersSub')} />
        <div className="flex flex-wrap gap-2 px-5 pb-5">
          {['openai', 'anthropic', 'gemini', 'ollama', 'custom'].map((p) => (
            <Badge key={p}>{p}</Badge>
          ))}
        </div>
      </Card>
    </div>
  )
}