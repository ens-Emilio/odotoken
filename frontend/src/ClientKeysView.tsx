import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useClientKeys, saveClientKey, deleteClientKey, type ClientKeyInput, type ClientKeyRecord } from './api'
import { formatCurrency, formatNumber } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { Button } from './ui/Button'
import { Dialog } from './ui/Dialog'
import { SortableTable, type Column } from './ui/SortableTable'
import { toast } from './ui/toast-store'
import { useLocale } from './i18n/useLocale'

const QUOTA_FIELDS = [
  ['daily_cost_limit_usd', 'clientKeys.dailyCost'],
  ['monthly_cost_limit_usd', 'clientKeys.monthlyCost'],
  ['daily_tokens_limit', 'clientKeys.dailyTokens'],
  ['monthly_tokens_limit', 'clientKeys.monthlyTokens'],
] as const

const EMPTY_FORM: ClientKeyInput = {
  client: '',
  project: '',
  daily_cost_limit_usd: 0,
  monthly_cost_limit_usd: 0,
  daily_tokens_limit: 0,
  monthly_tokens_limit: 0,
}

/**
 * One row is one key: who it identifies and what it may spend. The listing only
 * ever carries a masked value — the full secret is shown once, when it is
 * created, because a stored secret that can be re-read is a secret in the log.
 */
export default function ClientKeysView() {
  const { t, locale } = useLocale()
  const queryClient = useQueryClient()
  const { data, isLoading, isError, refetch } = useClientKeys()
  const [editing, setEditing] = useState<ClientKeyInput | null>(null)
  const [form, setForm] = useState<ClientKeyInput>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [secret, setSecret] = useState<string | null>(null)

  const openCreate = () => {
    setForm(EMPTY_FORM)
    setEditing(EMPTY_FORM)
  }

  const openEditor = (row: ClientKeyRecord) => {
    const { plaintext_key: _secret, ...rest } = row
    setForm(rest)
    setEditing(rest)
  }

  const closeEditor = () => {
    setEditing(null)
  }

  const save = async () => {
    setSaving(true)
    try {
      const saved = await saveClientKey(form)
      await queryClient.invalidateQueries({ queryKey: ['client-keys'] })
      toast('ok', t('clientKeys.saved'))
      closeEditor()
      if (saved.plaintext_key) setSecret(saved.plaintext_key)
    } catch (err) {
      const code = err instanceof Error ? err.message : ''
      toast('error', code.startsWith('error.') ? t(code) : t('clientKeys.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  const revoke = async (row: ClientKeyRecord) => {
    try {
      await deleteClientKey(row.key)
      await queryClient.invalidateQueries({ queryKey: ['client-keys'] })
      toast('ok', t('clientKeys.revoked'))
    } catch (err) {
      const code = err instanceof Error ? err.message : ''
      toast('error', code.startsWith('error.') ? t(code) : t('clientKeys.saveFailed'))
    }
  }

  const quota = (value: number | undefined, kind: 'money' | 'count') => {
    if (!value) return <span className="text-[var(--text-faint)]">{t('clientKeys.noLimit')}</span>
    return <span className="num tabular-nums">{kind === 'money' ? formatCurrency(value, locale) : formatNumber(value, locale)}</span>
  }

  const columns: Column<ClientKeyRecord>[] = [
    { key: 'key', header: t('clientKeys.columnKey'), render: (r) => <span className="num font-medium text-[var(--text-strong)]">{r.key}</span>, sortValue: (r) => r.key },
    { key: 'client', header: t('clientKeys.client'), render: (r) => <Badge>{r.client}</Badge>, sortValue: (r) => r.client },
    { key: 'project', header: t('clientKeys.project'), render: (r) => r.project || '—', sortValue: (r) => r.project },
    { key: 'dailyCost', header: t('clientKeys.dailyCost'), align: 'right', render: (r) => quota(r.daily_cost_limit_usd, 'money'), sortValue: (r) => r.daily_cost_limit_usd },
    { key: 'monthlyCost', header: t('clientKeys.monthlyCost'), align: 'right', render: (r) => quota(r.monthly_cost_limit_usd, 'money'), sortValue: (r) => r.monthly_cost_limit_usd },
    { key: 'dailyTokens', header: t('clientKeys.dailyTokens'), align: 'right', render: (r) => quota(r.daily_tokens_limit, 'count'), sortValue: (r) => r.daily_tokens_limit },
    { key: 'monthlyTokens', header: t('clientKeys.monthlyTokens'), align: 'right', render: (r) => quota(r.monthly_tokens_limit, 'count'), sortValue: (r) => r.monthly_tokens_limit },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: (r) => (
        <span className="flex items-center justify-end gap-2">
          <button type="button" onClick={() => openEditor(r)} className="rounded border border-[var(--border)] px-2 py-0.5 text-[11px] text-[var(--text-strong)] hover:bg-[var(--bg-fill-secondary)]">{t('clientKeys.edit')}</button>
          <button type="button" onClick={() => { void revoke(r) }} title={t('clientKeys.revokeHint')} className="rounded border border-[var(--border)] px-2 py-0.5 text-[11px] text-[var(--danger-ink)] hover:bg-[var(--danger-soft)]">{t('clientKeys.revoke')}</button>
        </span>
      ),
    },
  ]

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader
          title={t('nav.clientKeys')}
          subtitle={t('clientKeys.subtitle')}
          right={<Button variant="primary" onClick={openCreate}>{t('clientKeys.new')}</Button>}
        />
        {isLoading && <div className="mx-5 mb-5 h-40 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" />}
        {isError && <Empty label={t('clientKeys.loadFailed')} hint={t('error.clientKeys.unavailable')} onRetry={() => { void refetch() }} />}
        {data && data.length === 0 && <Empty label={t('clientKeys.empty')} hint={t('clientKeys.subtitle')} />}
        {data && data.length > 0 && (
          <SortableTable rows={data} rowKey={(r) => r.key} columns={columns} pageSize={25} searchable searchKeys={['key', 'client', 'project']} />
        )}
      </Card>

      <Dialog open={editing !== null} onClose={closeEditor} title={form.key ? t('clientKeys.editTitle', { key: form.key }) : t('clientKeys.createTitle')}>
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-[11px] text-[var(--text-soft)]">{t('clientKeys.client')}</span>
              <input
                value={form.client}
                onChange={(e) => setForm((current) => ({ ...current, client: e.target.value }))}
                className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-[11px] text-[var(--text-soft)]">{t('clientKeys.project')}</span>
              <input
                value={form.project}
                onChange={(e) => setForm((current) => ({ ...current, project: e.target.value }))}
                className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
              />
            </label>
          </div>
          <p className="text-[11px] text-[var(--text-soft)]">{t('clientKeys.quotaHint')}</p>
          <div className="grid grid-cols-2 gap-3">
            {QUOTA_FIELDS.map(([field, label]) => (
              <label key={field} className="flex flex-col gap-1">
                <span className="text-[11px] text-[var(--text-soft)]">{t(label)}</span>
                <input
                  type="number"
                  min="0"
                  step="any"
                  onChange={(e) => setForm((current) => ({ ...current, [field]: Number(e.target.value) || 0 }))}
                  className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
                />
              </label>
            ))}
          </div>
          <div className="flex justify-end gap-2">
            <Button onClick={closeEditor}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={save} disabled={saving}>{saving ? t('common.saving') : t('clientKeys.save')}</Button>
          </div>
        </div>
      </Dialog>

      <Dialog open={secret !== null} onClose={() => setSecret(null)} title={t('clientKeys.secretTitle')}>
        <div className="space-y-3">
          <code className="block break-all rounded-md border border-[var(--border)] bg-[var(--bg-fill-tertiary)] px-3 py-2 text-[12px] text-[var(--text-strong)]">{secret}</code>
          <div className="flex justify-end">
            <Button onClick={() => setSecret(null)}>{t('common.close')}</Button>
          </div>
        </div>
      </Dialog>
    </div>
  )
}
