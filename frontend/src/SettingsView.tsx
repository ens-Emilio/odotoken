import { Card, CardHeader, Badge } from './ui/Card'

export default function SettingsView() {
  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader title="Service" subtitle="odotoken proxy status" />
        <div className="space-y-2 px-5 pb-5 text-[12px]">
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">Endpoint</span>
            <span className="num text-[var(--text-strong)]">127.0.0.1:8090</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">Version</span>
            <Badge>0.1.0</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">Dashboard refresh</span>
            <Badge tone="brand">every 10s</Badge>
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title="Configuration" subtitle="edit config.json and restart the service" />
        <div className="space-y-2 px-5 pb-5 text-[12px]">
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">Config file</span>
            <span className="num text-[var(--text-strong)]">config.json</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">Database</span>
            <span className="num text-[var(--text-strong)]">odotoken.db (SQLite WAL)</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[var(--text-muted)]">Prices</span>
            <span className="num text-[var(--text-strong)]">prices.json</span>
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title="Manage" subtitle="systemd user service commands" />
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
        <CardHeader title="Routed Providers" subtitle="upstreams behind this proxy" />
        <div className="flex flex-wrap gap-2 px-5 pb-5">
          {['anthropic → crustoxy:8082', 'openai → useoneai:8791', 'kodeo', 'kodeo-cc', 'verboo', 'cheapestinference', 'useoneai', 'pegasus', 'gemini', 'ollama'].map((p) => (
            <Badge key={p}>{p}</Badge>
          ))}
        </div>
      </Card>
    </div>
  )
}