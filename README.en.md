# 🍇 odotoken
**Transparent AI token metering proxy, usage tracker and budget guard.**
> **v0.6.0** — Color palettes inspired by native fruits from **Acre, Brazil** 🌿
**[Português (BR)](README.md) · [English](README.en.md) · [Español](README.es.md)**
---
## 🎨 Color Palettes: Fruits of Acre
The odotoken dashboard now offers **21 color palettes** extracted directly from the real tones of the skin, pulp and leaves of native and typical fruits from the Amazonian state of Acre, Brazil:
| Fruit | Dominant Color | Inspiration |
|-------|---------------|-------------|
| **Açaí** | `#7A3E7D` | Deep purple of pure pulp |
| 🟤 **Cupuaçu** | `#6B4728` | Earth brown of velvety skin |
| 🟠 **Pupunha** | `#E65100` | Fire orange of ripe skin |
| 🟡 **Tucumã** | `#F57C00` | Saffron orange of oily pulp |
| 🟫 **Bacuri** | `#C68B23` | Golden yellow of fibrous skin |
| 🟢 **Biribá** | `#7CB342` | Lime green of young skin |
| 🔴 **Buriti** | `#D84315` | Dark terracotta of scales |
| 🌸 **Bacaba** | `#9C5B6F` | Dark wine of ripe skin |
| 🫒 **Patauá** | `#8B9574` | Grey-green of oily pulp |
| 🌿 **Ingá** | `#475B28` | Dark moss green of grooves |
| 💛 **Taperebá** | `#FBC02D` | Intense solar yellow of pulp |
| 🟨 **Abiu** | `#FFD600` | Bright yellow of smooth skin |
| 🟤 **Uxi** | `#795548` | Chestnut brown of dry skin |
| 🍋 **Araçá-boi** | `#C0CA33` | Yellow-green of velvety skin |
| 🍌 **Banana** | `#FFC107` | Deep yolk yellow of pulp |
| 🍍 **Abacaxi** | `#1B4D3E` | Rough forest green of crown |
| 🥝 **Graviola** | `#2E4A28` | Dark leaf green with thorns |
| **Maracujá** | `#EA580C` | Burnt orange of skin |
| 🥭 **Manga** | `#DC2626` | Vivid carmine red of exposed cheek |
| 🍑 **Mamão** | `#FB923C` | Rich papaya orange of sweet pulp |
| 🍉 **Melancia** | `#E11D48` | Vivid ruby red of pulp heart |
Each palette applies colors extracted directly from the real tones of each fruit's skin, pulp and leaves, maintaining AA contrast and readability in both light and dark modes.
---
## 📋 Overview
odotoken is a lightweight reverse proxy that sits between your AI CLI agents (Claude Code, crush, opencode, omp…) and their LLM providers, transparently metering token usage and estimating costs — with budget enforcement, a React dashboard, and a standalone token counting API.
## ⚙️ How It Works
```
your CLI agents ──→ odotoken :8090 ──→ your existing proxies/routers ──→ LLM providers
                        │
                        ├── token metering (JSON + SSE streaming)
                        ├── real cost calculation (2,700+ model prices)
                        ├── budget guard (alert / block)
                        └── SQLite usage history
                                │
                     React dashboard (/app/)
```
The proxy routes by **path prefix** — each provider key in `config.json` becomes a route:
| Request path | Upstream |
|--------------|----------|
| `/anthropic/v1/messages` | your Anthropic-compatible endpoint |
| `/openai/v1/chat/completions` | your OpenAI-compatible endpoint |
| `/verboo/...`, `/kodeo/...` | any custom provider you configure |
| `/v1/messages`, `/v1/chat/completions` | auto-detected by request shape |
Token usage is extracted from both regular JSON responses and SSE streams (including custom OpenAI-compatible providers). Every request is recorded in SQLite with prompt/completion/cache tokens, calculated cost, latency and status code.
## ✨ Features
- **Transparent metering** — point CLIs at the local proxy; requests and response streams pass through unchanged
- **Real pricing** — 2,700+ models with per-1M-token USD rates from a public pricing API and local router configs
- **Budget guard** — daily/monthly cost and token limits with alerts or blocking (HTTP 429)
- **React dashboard** — live KPIs, charts, sortable tables, event stream with per-request detail dialog, light/dark/system themes, **21 color palettes inspired by Acre fruits**
- **CLI** — terminal stats (`odotoken stats --by-model`), recent log, service management
- **Token counting API** — optional standalone service for pre-flight counting (heuristic + remote provider APIs)
- **Model catalog** — aliases, context windows, per-model counting strategies
- **TTL cache** — versioned, invalidated when catalog/tokenizer changes
- **Throughput metrics** — TPS/TTFT per model, project and interval (`GET /api/tps`, `odotoken stats --tps`, Throughput view)
- **Replay & optimize** — re-price historical events with current prices (`odotoken replay`) and cheaper model recommendations (`odotoken optimize`)
- **Tags & forecast** — segmentation via `X-Odo-Tags` and linear cost forecast with anomaly detection on Overview
- **Alerts & export** — Slack/Discord/Telegram/SMTP alerts on budget breach, per-project limits, CSV export (`GET /api/export?format=csv`)
- **Live runs** — `GET /api/sessions/live` lists in-flight model requests; `X-Odo-Session` groups concurrent calls from one conversation
## 🚀 Installation
Download the binary for your platform from the latest [GitHub release](https://github.com/ens-Emilio/odotoken/releases/latest), or build from source:
```bash
git clone https://github.com/ens-Emilio/odotoken.git
cd odotoken
# backend (Go 1.26+)
go build -o bin/odotoken ./cmd/odotoken
# frontend (optional, dashboard ships pre-built in releases)
cd frontend && pnpm install && pnpm build && cd ..
```
## ⚡ Quick Start
### 1. Configuration
```bash
cp config.json.example config.json  # edit upstreams / limits / sessions
```
Minimal `config.json`:
```jsonc
{
  "proxy": {
    "host": "127.0.0.1",
    "port": 8090,
    "upstream_openai": "https://api.openai.com",
    "upstream_anthropic": "https://api.anthropic.com"
  },
  "providers": {
    // routes by prefix: /<key>/... → upstream_url
    "anthropic": { "upstream_url": "https://api.anthropic.com" },
    "openai": { "upstream_url": "https://api.openai.com" },
    "myrouter": { "upstream_url": "https://my-router.example/v1" }
  },
  "guard": {
    "daily_cost_limit_usd": 10,
    "monthly_cost_limit_usd": 100,
    "daily_tokens_limit": 50000,
    "monthly_tokens_limit": 500000,
    "warning_threshold_pct": 80,
    "action_on_breach": "block"
  },
  "database": { "path": "odotoken.db" }
}
```
API keys are **never** stored in the config — your CLIs send their own `Authorization`/`x-api-key` headers and the proxy forwards them untouched.
### 2. Point your CLI agents at the proxy
| CLI | How |
|-----|-----|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | provider `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | provider `baseURL: "http://127.0.0.1:8090/<provider-key>"` |
| Others | set base URL to the proxy; use `/<provider-key>/` prefix for multi-provider setups |
Restart the CLI after changing its config (configs load at startup).
### 3. Running
```bash
odotoken start              # proxy + dashboard
# or as a user systemd service (generates unit with correct absolute paths):
odotoken install-service
systemctl --user daemon-reload
systemctl --user enable --now odotoken
```
Open **http://127.0.0.1:8090/app/** — usage appears within 10 seconds.
## 🎨 Selecting a Color Palette
In the top-right corner of the dashboard, next to the theme button (sun/moon), you'll see a **colored circle** representing the active palette. Click to open the menu with the 21 options inspired by Acre fruits:
1. **Original** — default neutral palette (brand green)
2. **Açaí** 🟣 — deep purple
3. **Cupuaçu** 🟤 — earth brown
4. **Pupunha** 🟠 — fire orange
5. **Tucumã** 🟡 — golden amber
6. **Bacuri** 🟫 — golden brown
7. **Biribá** 🟢 — lime green
8. **Buriti** 🔴 — dark terracotta
9. **Bacaba** 🌸 — dark wine
10. **Patauá** 🫒 — grey-green
11. **Ingá** 🌿 — moss green
12. **Taperebá** 💛 — solar yellow
13. **Abiu** 🟨 — bright yellow
14. **Uxi** 🟤 — chestnut brown
15. **Araçá-boi** 🍋 — yellow-green
16. **Banana** 🍌 — yolk yellow
17. **Abacaxi** 🍍 — forest green
18. **Graviola** 🥝 — leaf green
19. **Maracujá** 🧡 — burnt orange
20. **Manga** 🥭 — carmine red
21. **Mamão** 🍑 — papaya orange
22. **Melancia** 🍉 — ruby red
The choice is persisted in `localStorage` and applied instantly via dynamic CSS variables.
## 📖 CLI Reference
```bash
odotoken start [--host] [--port] [--reload]   # starts proxy + dashboard
odotoken serve [--port 8081]                  # standalone counting API
odotoken status                               # checks service/port
odotoken stats [--by-model] [--by-client] [--since ISO-DATE] [--days N]
odotoken log [--limit 20]                     # recent request log
odotoken count text "hello" [--model X] [--output json]
odotoken count chat --file messages.json      # JSON array of messages
odotoken count batch requests.jsonl
odotoken models list [--provider X]
odotoken models info <model-id>
odotoken sessions scan [--since ISO-DATE]     # read-only: which CLIs exist here
odotoken sessions import [--dry-run] [--cli omp|claude|codex|crush] [--since ISO-DATE]
odotoken sessions stats [--since ISO-DATE] [--by-cli] [--by-model] [--by-project]
```
## 🌐 HTTP API
Proxy endpoints (same port as dashboard):
```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/api/stats?since=2026-10-01T00:00:00
curl http://127.0.0.1:8090/api/guard/status
curl http://127.0.0.1:8090/api/filters
curl http://127.0.0.1:8090/api/stats.csv
curl http://127.0.0.1:8090/api/stats.svg
```
Token counting API (`odotoken serve`, port 8081):
```bash
curl -X POST http://127.0.0.1:8081/v1/count \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Hello world"}'
curl -X POST http://127.0.0.1:8081/v1/count/batch -d '{"items": [...]}'
curl http://127.0.0.1:8081/v1/models
curl http://127.0.0.1:8081/health      # + /ready, /metrics
```
## 💰 Model Pricing
Cost tracking uses `prices.json` — **2,700+ models** with real per-1M-token USD rates, synced from two sources (later wins):
| Source | Coverage | Priority |
|--------|----------|----------|
| Pricing API (default `http://154.29.76.173`, override with `PEGASUS_PRICING_API`) | 3,500+ models, public REST | 1 |
| Router configs (`~/.local/share/crush/providers.json`, if present) | exact prices your local routers charge | 2 (wins) |
Refresh prices at any time:
```bash
node scripts/sync-prices.mjs
systemctl --user restart odotoken
```
Manual overrides: add `"override": true` to any entry and the sync script will never touch it:
```json
"my-custom-model": { "input": 1.5, "output": 6.0, "cache_read": 0.2, "cache_write": 0, "override": true }
```
The public pricing API needs no auth key for reads (rate limited: 10 req/s per IP at nginx, 60 req/min at the app). Deploy your own instance and point `PEGASUS_PRICING_API` at it if you prefer.
## 📥 Importing Local Sessions
Use odotoken to read sessions already saved on this machine (no proxy needed) and price them via `prices.json`:
```bash
odotoken sessions scan                        # which CLIs/files were found (read-only)
odotoken sessions import --dry-run            # counts tokens + cost without writing
odotoken sessions import                      # imports into SQLite (idempotent, with dedup)
odotoken sessions import --since 2026-10-01   # incremental
odotoken sessions stats --by-cli --by-model   # breakdown by origin
```
| CLI | Source | What is read |
|-----|--------|--------------|
| OMP | `~/.omp/stats.db` | SQLite `messages` |
| Claude Code | `~/.claude/projects/**/*.jsonl` | JSONL `type=assistant` → `message.usage` |
| Codex | `~/.codex/sessions/**` | `token_usage_record` → `payload.usage` |
| Crush | `~/.local/share/crush/projects.json` → `data_dir/**/*.jsonl` | JSONL `usage` (best-effort) |
- **Single pricing** — every imported row is re-priced by `PricingCalculator` (`prices.json`).
- **Dedup** — `UNIQUE(source, session_file, entry_id)`; re-running `import` inserts `0`.
- **Dashboard** — **Source** filter in header, `GET /api/stats?source=...`, `POST /api/sessions/import` and **By Source** card.
## ⚙️ Configuration Reference
| Setting | Description | Default |
|---------|-------------|---------|
| `proxy.host` / `proxy.port` | bind address | `127.0.0.1:8090` |
| `providers.<key>.upstream_url` | path-prefix route upstream | — |
| `proxy.upstream_openai` / `upstream_anthropic` | fallback upstreams for auto-detected requests | official APIs |
| `guard.daily_cost_limit_usd` / `guard.monthly_cost_limit_usd` | daily/monthly cost limits (0 = disabled) | `null` |
| `guard.daily_tokens_limit` / `guard.monthly_tokens_limit` | daily/monthly token limits (0 = disabled) | `null` |
| `guard.warning_threshold_pct` | percentage at which the guard moves from ok to warning | `80` |
| `guard.action_on_breach` | `alert` keeps serving requests while emitting alerts; `block` rejects breached requests with HTTP 429 | `block` |
| `guard.timezone` | IANA zone used for daily and monthly budget boundaries | UTC |
| `limits.webhook_url` | webhook notified on `warning`/`breached` | `null` |
| `database.path` | SQLite file (WAL mode) | `odotoken.db` |
| `token_counter.enabled` | pre-flight counting middleware on proxy | `true` |
| `sessions.enabled` / `sessions.paths` | local session import (claude/codex/omp/crush) | `true` / `~/.claude/...` |
## 🏷️ Project Tags, Shadow Pricing & Privacy
- **Project tags** — send `X-Odo-Project: <name>` (or `X-Project`) on any request; costs segment per project in the dashboard, `/api/stats?project=<id>` and `odotoken stats --by-project`.
- **Live runs** — `GET /api/sessions/live` lists in-flight model requests. Send `X-Odo-Session: <conversation-id>` to group concurrent calls from one conversation; each request still receives its own `run_id`.
- **Shadow pricing** — local models (zero price) get a `shadow_cost` calculated from the cloud equivalent chosen by `getCloudEquivalent` (`internal/pricing/calculator.go`); the dashboard shows the "Saved vs Cloud" KPI when there's savings.
- **Privacy** — odotoken stores usage metadata, not prompt or response content. Imported messages are never parsed: the proxy records tokens, times, model, project and tags. No PII redactor is currently applied; do not treat the database as anonymized.
- **Prometheus** — `GET /metrics` on the proxy port serves metrics in text format; import `examples/grafana-dashboard.json` or run `docker compose -f examples/docker-compose.observability.yml up -d`.
- **Mock & chaos** — set a provider's upstream to `mock://local` for synthetic responses (no network, with metering). Enable `chaos: {enabled: true, rate: 0.05, errors: [429,500], latency_ms: 3000}` to test client resilience; `X-Odo-No-Chaos: true` bypasses it.
## 🔧 Troubleshooting
**All CLIs fail at once with `ECONNREFUSED 127.0.0.1:8090`** — the proxy is down, not your providers. Quick triage:
```bash
odotoken status          # checks the systemd unit AND the port
curl http://127.0.0.1:8090/health
journalctl --user -u odotoken -n 50 --no-pager
```
| Symptom | Meaning | Fix |
|---------|---------|-----|
| `status=203/EXEC` in journal | unit `ExecStart` path doesn't exist (project moved/cloned elsewhere) | run `odotoken install-service` from the project root |
| `activating (auto-restart)` forever | service in a crash loop — check journal for the reason | fix the cause, then `systemctl --user restart odotoken` |
| nothing on :8090, service inactive | never started | `systemctl --user enable --now odotoken` |
| `/health` OK but CLIs fail | proxy is fine — the CLI configs are stale | restart the CLI (configs load at startup) |
**Triage rule**: if the error has an HTTP status (e.g. 503 from the provider), the proxy delivered the request and the problem is upstream. If it's a connection error with no status at all (`ECONNREFUSED`, "Unable to connect"), the proxy itself is down.
## 🔒 Security Notes
- **Single process only** — SQLite + the in-memory cache assume one worker. Run `odotoken start` without `--workers` (default 1); multi-worker would split metering across processes with no shared state.
- The proxy binds to `127.0.0.1` by default — local only
- API keys pass in headers; **nothing is logged or stored** — only token counts, model names, client app name (from User-Agent), cost and latency
- Put behind authentication/reverse-proxy before exposing on a network
## 🛠️ Development
```bash
make build                                 # compiles ./cmd/odotoken → bin/odotoken
make test                                  # go test -race ./...
cd frontend && pnpm install && pnpm build   # builds dashboard → internal/server/static/
```
## 📄 License
GPL-3.0-or-later. See [LICENSE](LICENSE).
## 🤝 Contributing
PRs welcome — run the test suite and keep `prices.json` regenerated via the sync script rather than hand-edited (except `"override": true` entries).
---
**Made with 🌿 in Acre, Brazil** — where the forest inspires even the colors of code.