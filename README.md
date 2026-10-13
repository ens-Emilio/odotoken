# odotoken

Transparent AI token metering proxy, usage tracker and budget guard.

**[Português (BR)](README.ptbr.md) · [English](README.md) · [Español](README.es.md)**

## Overview

odotoken is a lightweight reverse proxy that sits between your AI CLI agents (Claude Code, crush, opencode, omp…) and their LLM providers, transparently metering token usage and estimating costs — with budget enforcement, a React dashboard, and a standalone token counting API.

## How it works

```
your CLI agents ──→ odotoken :8090 ──→ your existing proxies/routers ──→ LLM providers
                        │
                        ├── token metering (JSON + SSE streaming)
                        ├── real cost calculation (2,700+ model prices)
                        ├── budget guard (alert / block)
                        └── SQLite usage history
                                │
                     React dashboard (/dashboard)
```

The proxy routes by **path prefix** — each provider key in `config.json` becomes a route:

| Request path | Upstream |
|--------------|----------|
| `/anthropic/v1/messages` | your Anthropic-compatible endpoint |
| `/openai/v1/chat/completions` | your OpenAI-compatible endpoint |
| `/verboo/...`, `/kodeo/...` | any custom provider you configure |
| `/v1/messages`, `/v1/chat/completions` | auto-detected by request shape |

Token usage is extracted from both regular JSON responses and SSE streams (including custom OpenAI-compatible providers). Every request is recorded in SQLite with prompt/completion/cache tokens, calculated cost, latency and status code.

## Features

- **Transparent metering** — no changes to how your CLIs work, just point them at the proxy
- **Real pricing** — 2,700+ models with per-1M-token rates from a public pricing API and local router configs
- **Budget guard** — daily/monthly cost & token limits with alert or hard block (HTTP 429)
- **React dashboard** — live KPIs, charts, sortable tables, event stream with per-request detail dialog, light/dark/system themes
- **CLI** — terminal stats (`odotoken stats --by-model`), recent log, service management
- **Token counting API** — optional standalone service for pre-flight counts (heuristic + remote provider APIs)
- **Model catalog** — aliases, context windows, per-model counting strategies
- **Cache with TTL** — versioned, invalidated on catalog/tokenizer changes

## Installation

```bash
git clone https://github.com/ens-Emilio/odotoken.git
cd odotoken

# backend
python -m venv .venv
.venv/bin/pip install -e .

# frontend (optional, dashboard ships pre-built in releases)
cd frontend && pnpm install && pnpm build && cd ..
```

## Quick start

### 1. Configuration

Copy and edit `config.json`:

```jsonc
{
  "proxy": {
    "host": "127.0.0.1",
    "port": 8090,
    "upstream_openai": "https://api.openai.com",
    "upstream_anthropic": "https://api.anthropic.com"
  },
  "providers": {
    // path-prefix routes: /<key>/... → upstream_url
    "anthropic": { "upstream_url": "https://api.anthropic.com" },
    "openai": { "upstream_url": "https://api.openai.com" },
    "myrouter": { "upstream_url": "https://my-router.example/v1" }
  },
  "limits": {
    "monthly_budget_usd": 50.0,
    "block_on_budget_exceeded": false
  },
  "database": { "path": "odotoken.db" }
}
```

API keys are **never stored** in the config — your CLIs send their own `Authorization`/`x-api-key` headers and the proxy forwards them untouched.

### 2. Point your CLI agents at the proxy

| CLI | How |
|-----|-----|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | provider `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | provider `baseURL: "http://127.0.0.1:8090/<provider-key>"` |
| Anything else | set its base URL to the proxy; use a `/provider-key/` prefix for multi-provider setups |

Restart the CLI after changing its config (configs load at startup).

### 3. Run it

```bash
odotoken start              # proxy + dashboard
# or as a systemd user service (generates the unit with correct absolute paths):
odotoken install-service
systemctl --user daemon-reload
systemctl --user enable --now odotoken
```

Open **http://127.0.0.1:8090/dashboard** — usage appears within 10 seconds.

## CLI reference

```bash
odotoken start [--host] [--port] [--reload]   # start proxy + dashboard
odotoken serve [--port 8081]                  # standalone token counter API
odotoken status                               # check service/port
odotoken stats [--by-model] [--by-client] [--since ISO-DATE] [--days N]
odotoken log [--limit 20]                     # recent request log
odotoken count text "hello" [--model X] [--output json]
odotoken count chat --file messages.json      # JSON array of messages
odotoken count batch requests.jsonl
odotoken models list [--provider X]
odotoken models info <model-id>
```

## HTTP API

Proxy endpoints (same port as the dashboard):

```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/api/stats?since=2026-10-01T00:00:00
```

Token counter API (`odotoken serve`, port 8081):

```bash
curl -X POST http://127.0.0.1:8081/v1/count \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Hello world"}'

curl -X POST http://127.0.0.1:8081/v1/count/batch -d '{"items": [...]}'
curl http://127.0.0.1:8081/v1/models
curl http://127.0.0.1:8081/health      # + /ready, /metrics
```

## Model pricing

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

## Configuration reference

| Setting | Description | Default |
|---------|-------------|---------|
| `proxy.host` / `proxy.port` | bind address | `127.0.0.1:8080` |
| `providers.<key>.upstream_url` | path-prefix route upstream | — |
| `proxy.upstream_openai` / `upstream_anthropic` | fallback upstreams for auto-detected requests | official APIs |
| `limits.monthly_budget_usd` | monthly cost limit (0 = disabled) | `0.0` |
| `limits.daily_budget_usd`, `daily/monthly_tokens_limit` | more limits | `0` (disabled) |
| `limits.action_on_breach` | `alert` or `block` (429) | `alert` |
| `database.path` | SQLite file (WAL mode) | `odotoken.db` |
| `token_counter.enabled` | pre-flight counting middleware on the proxy | `false` |

## Troubleshooting

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

## Security notes

- **Single process only** — SQLite + the in-memory cache assume one worker. Run `odotoken start` without `--workers` (default 1); multi-worker would split metering across processes with no shared state.

- The proxy binds to `127.0.0.1` by default — local only
- API keys pass through in headers; **nothing is logged or stored** — only token counts, model names, client app name (from User-Agent), cost and latency
- Put it behind auth/reverse-proxy before exposing on a network

## Development

```bash
.venv/bin/pip install -e ".[dev]" 2>/dev/null || .venv/bin/pip install -e . pytest pytest-asyncio
.venv/bin/pytest                          # 75 tests (unit, integration, CLI, golden)
cd frontend && pnpm install && pnpm build  # build dashboard → src/odotoken/static/
```

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).

## Contributing

PRs welcome — run the test suite and keep `prices.json` regenerated via the sync script rather than hand-edited (except `"override": true` entries).