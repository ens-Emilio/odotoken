import { useState, useRef, useEffect } from 'react'
import { Card, CardHeader } from './ui/Card'
import { Button } from './ui/Button'
import { useLocale } from './i18n/useLocale'

interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
}

interface PlaygroundResponse {
  choices?: Array<{ message?: { content?: string } }>
  error?: { message?: string; type?: string }
  model?: string
  usage?: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number }
}

/**
 * Feature 18 — Interactive Playground.
 * Sends real requests through the OdoToken proxy (/v1/chat/completions),
 * exercising the same guard, rate-limit and attribution middleware as
 * production traffic. Streaming is intentionally not used here so the
 * operator can inspect latency, token counts and errors inline.
 */
export default function PlaygroundView() {
  const { t } = useLocale()
  const [model, setModel] = useState('gpt-4o-mini')
  const [systemPrompt, setSystemPrompt] = useState('You are a helpful assistant.')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [meta, setMeta] = useState<{ model?: string; tokens?: number; latencyMs?: number } | null>(null)
  const [apiKey, setApiKey] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, loading])

  const send = async () => {
    const text = input.trim()
    if (!text || loading) return

    const userMsg: Message = { role: 'user', content: text }
    const nextMessages: Message[] = [...messages, userMsg]
    setMessages(nextMessages)
    setInput('')
    setLoading(true)
    setError(null)
    setMeta(null)

    const body = {
      model,
      messages: [
        ...(systemPrompt ? [{ role: 'system', content: systemPrompt }] : []),
        ...nextMessages,
      ],
    }

    const start = performance.now()
    try {
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}),
        },
        body: JSON.stringify(body),
      })

      const data = (await res.json()) as PlaygroundResponse
      const latency = Math.round(performance.now() - start)

      if (!res.ok || data.error) {
        setError(data.error?.message ?? `HTTP ${res.status}`)
        return
      }

      const reply = data.choices?.[0]?.message?.content ?? ''
      setMessages([...nextMessages, { role: 'assistant', content: reply }])
      setMeta({
        model: data.model,
        tokens: data.usage?.total_tokens,
        latencyMs: latency,
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  const clear = () => {
    setMessages([])
    setError(null)
    setMeta(null)
  }

  return (
    <div className="flex h-[calc(100vh-64px)] flex-col gap-4 p-5">
      {/* Config panel */}
      <Card>
        <CardHeader title={t('nav.playground') ?? 'Playground'} subtitle={t('playground.subtitle') ?? 'Test prompts through the live proxy with full guard & attribution'} />
        <div className="grid gap-3 px-5 pb-4 sm:grid-cols-[1fr_1fr_2fr_auto]">
          <label className="flex flex-col gap-1">
            <span className="text-[11px] text-[var(--text-soft)]">{t('common.model') ?? 'Model'}</span>
            <input
              value={model}
              onChange={(e) => setModel(e.target.value)}
              className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[11px] text-[var(--text-soft)]">API Key</span>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="sk-..."
              className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[11px] text-[var(--text-soft)]">{t('playground.systemPrompt') ?? 'System prompt'}</span>
            <input
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
            />
          </label>
          <div className="flex items-end">
            <Button variant="secondary" onClick={clear}>{t('common.clear') ?? 'Clear'}</Button>
          </div>
        </div>
      </Card>

      {/* Chat area */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--bg-panel)]">
        <div className="flex-1 space-y-3 overflow-y-auto p-4">
          {messages.length === 0 && !loading && (
            <div className="flex h-full items-center justify-center text-[12px] text-[var(--text-muted)]">
              {t('playground.empty') ?? 'Send a message to start testing through the proxy.'}
            </div>
          )}
          {messages.map((m, i) => (
            <div key={i} className={`flex ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              <div className={`max-w-[80%] rounded-lg px-3 py-2 text-[12px] leading-relaxed ${
                m.role === 'user'
                  ? 'bg-[var(--brand)] text-[var(--brand-contrast)]'
                  : 'bg-[var(--bg-fill-secondary)] text-[var(--text-strong)]'
              }`}>
                <pre className="whitespace-pre-wrap font-sans">{m.content}</pre>
              </div>
            </div>
          ))}
          {loading && (
            <div className="flex justify-start">
              <div className="rounded-lg bg-[var(--bg-fill-secondary)] px-3 py-2 text-[12px] text-[var(--text-muted)]">
                {t('common.loading') ?? 'Thinking…'}
              </div>
            </div>
          )}
          <div ref={bottomRef} />
        </div>

        {/* Meta bar */}
        {(meta || error) && (
          <div className="border-t border-[var(--border)] px-4 py-2 text-[10px] text-[var(--text-muted)]">
            {error && <span className="text-[var(--danger-ink)]">Error: {error}</span>}
            {meta && (
              <span className="flex gap-4">
                <span>Model: <strong className="text-[var(--text-strong)]">{meta.model}</strong></span>
                <span>Tokens: <strong className="text-[var(--text-strong)]">{meta.tokens ?? '—'}</strong></span>
                <span>Latency: <strong className="text-[var(--text-strong)]">{meta.latencyMs}ms</strong></span>
              </span>
            )}
          </div>
        )}

        {/* Input */}
        <div className="border-t border-[var(--border)] p-3">
          <div className="flex gap-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  send()
                }
              }}
              placeholder={t('playground.placeholder') ?? 'Type a message… (Enter to send)'}
              rows={2}
              className="flex-1 resize-none rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
            />
            <Button variant="primary" onClick={send} disabled={loading || !input.trim()}>
              {loading ? (t('common.loading') ?? 'Sending…') : (t('playground.send') ?? 'Send')}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}