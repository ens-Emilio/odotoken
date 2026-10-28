# odotoken
Transparent AI token metering proxy between your CLI agents and LLM providers. Tracks usage, calculates real cost, enforces budget limits.
**v0.6.0** · 21 color palettes inspired by Acre fruits · [Português](README.md) · [Español](README.es.md)
## How it works
```
CLI agents → odotoken :8090 → your proxies/routers → LLM providers
│
├─ metering (JSON + SSE)
├─ real cost (2,700+ models)
├─ budget guard (alert / block 429)
└─ SQLite history → React dashboard (/app/)
```
Routes by path prefix: each key in `providers` becomes a route (`/anthropic/...`, `/openai/...`, or any custom). Tokens extracted from JSON and SSE. Every request recorded with prompt/completion/cache tokens, cost, latency and status.
## Installation
Pre-built binary at [Releases](https://github.com/ens-Emilio/odotoken/releases/latest), or from source:
```bash
git clone https://github.com/ens-Emilio/odotoken.git && cd odotoken
go build -o bin/odotoken ./cmd/odotoken # Go 1.26+
cd frontend && pnpm install && pnpm build # optional; releases ship with dashboard
```
## Minimal config
```bash
cp config.json.example config.json
```
```jsonc
{
"proxy": { "host": "127.0.0.1", "port": 8090 },
"providers": {
"anthropic": { "upstream_url": "https://api.anthropic.com" },
"openai": { "upstream_url": "https://api.openai.com" }
},
"guard": {
"daily_cost_limit_usd": 10,
"monthly_cost_limit_usd": 100,
"action_on_breach": "block"
},
"database": { "path": "odotoken.db" }
}
```
API keys never stored in config — your CLIs send `Authorization`/`x-api-key` and the proxy forwards them untouched.
Point your CLIs at `http://127.0.0.1:8090`:
| CLI | How |
|-----|-----|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | `baseURL: "http://127.0.0.1:8090/<key>"` |
```bash
odotoken start # proxy + dashboard
# or as a service:
odotoken install-service && systemctl --user enable --now odotoken
```
Dashboard at http://127.0.0.1:8090/app/ — usage appears within 10s.
## Color palettes
21 palettes extracted from real skin, pulp and leaf tones of native Acre fruits (Açaí, Cupuaçu, Pupunha, Tucumã, Bacuri, Biribá, Buriti, Bacaba, Patauá, Ingá, Taperebá, Abiu, Uxi, Araçá-boi, Banana, Abacaxi, Graviola, Maracujá, Manga, Mamão, Melancia). Selector in the dashboard header (colored circle next to theme toggle), persisted in `localStorage`, AA contrast in light and dark modes.
## Features
- Transparent metering — zero changes to CLI workflow
- Real pricing — 2,700+ models, sync via `node scripts/sync-prices.mjs`
- Budget guard — daily/monthly cost and token limits, alert or block (429)
- React dashboard — KPIs, charts, sortable tables, event stream, light/dark/system themes
- CLI — `odotoken stats --by-model`, `log`, `count`, `models`, `sessions`
- Standalone counting API — `odotoken serve` on port 8081
- Throughput — TPS/TTFT per model (`GET /api/tps`)
- Replay & optimize — re-price history, suggest cheaper models
- Tags & forecast — `X-Odo-Tags`, linear forecast + anomaly detection
- Alerts — Slack/Discord/Telegram/SMTP on breach; CSV/SVG export
- Live runs — `GET /api/sessions/live`; `X-Odo-Session` groups calls from one conversation
- Local session import — `odotoken sessions import` reads OMP/Claude/Codex/Crush already saved on disk
## CLI
```bash
odotoken start [--host] [--port] [--reload]
odotoken serve [--port 8081]
odotoken status
odotoken stats [--by-model] [--by-client] [--since ISO] [--days N] [--tps]
odotoken log [--limit 20]
odotoken count text "..." [--model X] [--output json]
odotoken count chat --file msgs.json
odotoken count batch requests.jsonl
odotoken models list | info <id>
odotoken sessions scan | import [--dry-run] [--cli omp|claude|codex|crush] [--since ISO] | stats
odotoken replay
odotoken optimize
```
## HTTP API
Proxy (port 8090):
```
GET /health
GET /api/stats?since=ISO&project=&source=&tag=&tz=
GET /api/guard/status
GET /api/filters
GET /api/tps
GET /api/sessions/live
GET /api/export?format=csv
GET /api/stats.svg
GET /metrics
```
Standalone counting (port 8081):
```
POST /v1/count {"model","text"}
POST /v1/count/batch {"items":[...]}
GET /v1/models
GET /health | /ready | /metrics
```
## Configuration
| Key | Description | Default |
|-----|-------------|---------|
| `proxy.host` / `proxy.port` | bind | `127.0.0.1:8090` |
| `providers.<k>.upstream_url` | path-prefix route upstream | — |
| `proxy.upstream_openai` / `upstream_anthropic` | fallback for auto-detected requests | official APIs |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | cost limits (0 = off) | `null` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | token limits (0 = off) | `null` |
| `guard.warning_threshold_pct` | % for warning | `80` |
| `guard.action_on_breach` | `alert` or `block` (429) | `block` |
| `guard.timezone` | IANA zone for limits | UTC |
| `limits.webhook_url` | webhook on warning/breach | `null` |
| `database.path` | SQLite WAL | `odotoken.db` |
| `token_counter.enabled` | pre-flight middleware | `true` |
| `sessions.enabled` / `sessions.paths` | local import | `true` |
Price overrides: `"override": true` on a `prices.json` entry prevents sync from overwriting it.
## Troubleshooting
All CLIs fail with `ECONNREFUSED 127.0.0.1:8090` → proxy is down, not the providers.
```bash
odotoken status
curl http://127.0.0.1:8090/health
journalctl --user -u odotoken -n 50 --no-pager
```
- `status=203/EXEC` → `ExecStart` points to nonexistent path; run `odotoken install-service` from project root
- `activating (auto-restart)` loop → read journal, fix cause, `systemctl --user restart odotoken`
- `/health` OK but CLIs fail → CLI configs stale; restart the CLI
Rule: error with HTTP status (503 etc.) → upstream problem. Connection error with no status → proxy is down.
## Security
- Single process — SQLite + in-memory cache assume 1 worker
- Binds to `127.0.0.1` by default
- Keys pass in headers; nothing logged beyond tokens, model, client, cost and latency
- Put behind auth/reverse-proxy before exposing on a network
## Development
```bash
make build
make test
cd frontend && pnpm install && pnpm build
```
## License
GPL-3.0-or-later. See [LICENSE](LICENSE).