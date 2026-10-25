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

- **Transparent metering** — point CLIs at the local proxy; requests and response streams pass through unchanged
- **Pricing** — model prices from `prices.json`, plus shadow pricing for local models
- **Budget guard** — daily/monthly cost and token limits with alerts
- **Dashboard** — KPIs, charts, sortable tables, event stream, timezone selection, light/dark themes
- **CLI** — service management plus terminal stats and session import
- **Token counting API** — `/v1/count`, `/v1/count/batch`, and `/v1/models`
- **Throughput metrics** — measured output tokens per generation second, with a 10-second live window
- **Tags and forecast** — `X-Odo-Tags` segmentation and a linear cost forecast with anomaly detection
- **Alerts** — Slack, Discord, Telegram, and SMTP on warning/breach transitions

## Installation

Download the binary for your platform from the latest [GitHub release](https://github.com/ens-Emilio/odotoken/releases/latest), or build from source:

```bash
git clone https://github.com/ens-Emilio/odotoken.git
cd odotoken

# backend (Go 1.26+)
go build -o bin/odotoken ./cmd/odotoken

# frontend (optional, dashboard ships pre-built in releases)
cd frontend && pnpm install && pnpm build && cd ..
```

## Quick start

```bash
cp config.json.example config.json
bin/odotoken start --host 127.0.0.1 --port 8090
```

Open **http://127.0.0.1:8090/app/**. Point each CLI's provider base URL at the
proxy and keep its API key in the client. A provider-prefixed route such as
`/<provider>/...` selects `providers.<provider>.upstream_url`.

Typical `config.json`:

```json
{
  "server": {
    "bind_host": "127.0.0.1",
    "api_key": "change-me",
    "cors_allowlist": ["http://localhost:5173"]
  },
  "providers": {
    "openai": { "upstream_url": "https://api.openai.com" },
    "anthropic": { "upstream_url": "https://api.anthropic.com" }
  },
  "guard": {
    "daily_cost_limit_usd": 10,
    "monthly_cost_limit_usd": 100,
    "daily_tokens_limit": 50000,
    "monthly_tokens_limit": 500000,
    "warning_threshold_pct": 80,
    "action_on_breach": "block"
  }
}
```

`server.api_key` also resolves from `ODO_API_KEY`. In that mode, send
`Authorization: Bearer <key>` or `X-Api-Key: <key>` to `/api/*` and `/v1/*`.
The proxy continues forwarding each client's own provider credentials.

## CLI reference

```bash
odotoken start [--host HOST] [--port PORT] [--config FILE] [--db FILE]
odotoken stop
odotoken status
odotoken stats [--by-model] [--by-client] [--since ISO-DATE] [--days N]
odotoken import [omp|claude|codex|crush|opencode|pi] [--since ISO-DATE]
```

## HTTP API

```bash
curl http://127.0.0.1:8090/v1/health
curl http://127.0.0.1:8090/api/stats
curl "http://127.0.0.1:8090/api/stats?tag=prod&project=web"
curl http://127.0.0.1:8090/api/tps?interval=1h
curl http://127.0.0.1:8090/api/forecast
curl http://127.0.0.1:8090/api/events?limit=50
curl http://127.0.0.1:8090/api/guard/status
curl http://127.0.0.1:8090/api/filters
curl http://127.0.0.1:8090/api/stats.csv
curl http://127.0.0.1:8090/api/stats.svg
```

When `server.api_key` or `ODO_API_KEY` is set, `/api/*` and `/v1/*` require
`Authorization: Bearer <key>` or `X-Api-Key: <key>`.

## Token counting API

The `odotoken` daemon exposes token counting on the same port as the proxy and
dashboard:

```bash
curl -X POST http://127.0.0.1:8090/v1/count \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Hello world"}'

curl http://127.0.0.1:8090/v1/models
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

## Importing local CLI sessions

Import session history already on this machine (no proxy needed):

```bash
odotoken import                         # import every detected source
odotoken import omp --since 2026-10-01  # one source, incrementally
odotoken stats --by-model               # inspect imported usage
```

Supported CLIs (auto-detected, missing ones are silently skipped):

| CLI | Source | What is read |
|-----|--------|--------------|
| OMP | `~/.omp/stats.db` | SQLite `messages` (tokens + `cost_total` cross-checked via `prices.json`) |
| Claude Code | `~/.claude/projects/**/*.jsonl` | JSONL `type=assistant` → `message.usage` (streaming, one line at a time) |
| Codex | `~/.codex/sessions/**` + `~/.codex/history.jsonl` + `session_index.json` | `token_usage_record` → `payload.usage` (one record = one turn) |
| Crush | `~/.local/share/crush/projects.json` → `data_dir/**/*.jsonl` | best-effort JSONL `usage` blocks |
| Gemini/Antigravity/Amp | `~/.gemini/`, `~/.config/antigravity/`, `~/.config/amp/` | planned via `BaseImporter` — enable in `sessions.enabled=false` until parsers land |

**CLI edge cases**

| Edge case | How odotoken behaves | Recommendation (until a formal contract exists) |
|-----------|----------------------|--------------------------------------------------|
| **Locked / busy DB** — `~/.omp/stats.db` exclusive-lock at import time | open with `timeout=5s` + best-effort `SELECT`; busy → retried once, then logged and skipped (no partial writes) | re-run `sessions import`; do not stop the OMP daemon |
| **Hash collision in Claude** — two different CWDs hash to same `~/.claude/projects/<hash>/` | dir is treated as one project; costs still correct, `project` label may be ambiguous | keep the raw hash in `metadata` and prefer `X-Odo-Project` on proxy traffic |
| **Missing `usage` blocks** — Claude/Codex line has no `payload.usage` (tool-only turn) | skipped (not a billable turn); not imported | expected — not counted toward cost |
| **Crush `projects.json` missing / malformed** | importer not detected (`available=false`); `sessions scan` shows placeholder | create the file or set `sessions.paths.crush` |
| **Codex `workdir` vs `cwd` drift** — history file stores `workdir` but session stores `cwd` | `project` prefers `cwd` → fallback `workdir` → `"default"`; never fails the turn | normalize project names server-side via `X-Odo-Project` where possible |
| **Stale `--since` (client clock skew / TOML date parsing)** | `since` is string-compared as ISO-8601 UTC; if unparsable, filter is ignored and all rows re-checked by `UNIQUE` dedup | pass explicit `--since 2026-10-01T00:00:00Z` |
| **Large sessions (50k+ lines) streamed on import** | read line-by-line with `JSONDecodeError` skip per line; corrupt lines logged and skipped | never load the whole file into memory; interrupt mid-import is safe (transaction per chunk) |
| **God model via env prefix** — same underlying model under `kube`/`pegasus`/`kodeo-*` keys | priced by resolved `model` id, not upstream URL prefix; `shadow_cost` follows `getCloudEquivalent` in `internal/pricing/calculator.go` | extend `getCloudEquivalent` when provisioning a new local model family |
| **Privacy + PII in imported messages** | importer only reads token counts and model identifiers; it never reads `content` or `tool_calls`, and no PII redactor runs over session messages | keep imported databases local; odotoken does not claim PII scrubbing |

Details:
- **One pricing source** — every imported row is re-priced with `PricingCalculator` (`prices.json`); original `cost_total` is kept in `metadata`.
- **Deduplication** — `UNIQUE(source, session_file, entry_id)` — rerunning `import` inserts `0` new rows.
- **Dashboard** — header **Source** filter (`All sources` / `Proxy` / `Imported` / `import:claude` …), `GET /api/stats?source=proxy|import|import:omp`, `POST /api/sessions/import`, and a `By Source` card on Overview.
- **Config** — `config.json` → `sessions: {enabled, paths: {claude, codex, omp, crush}}` (absolute or `~` paths). `since` is an ISO-8601 string; the dashboard also passes `since=` alongside `source=`.

## Configuration reference

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
| `limits.rate_limit.enabled` / `rps` / `burst_size` | global proxy token bucket (0 = disabled) | disabled |
| `alerts.slack_url` / `discord_url` / `telegram` / `email` | alert channels used on warning/breached transitions | disabled |
| `server.bind_host` | host the daemon binds to | `127.0.0.1` |
| `server.api_key` or `ODO_API_KEY` | Bearer / `X-Api-Key` required on `/api/*` and `/v1/*` | disabled |
| `server.cors_allowlist` | exact origins allowed to read the API cross-origin | empty |
- **Alerts** — on a warning or breach transition, odotoken posts to the configured Slack, Discord, Telegram, and SMTP channels. Repeated polls in the same state do not send another alert.
- **Rate limit** — `limits.rate_limit` applies a process-wide token bucket to proxy requests; excess requests get HTTP 429.

## Project tags, shadow pricing & privacy

- **Project tags** — send `X-Odo-Project: <name>` on any request; costs segment per project in the dashboard and `odotoken stats --by-project`.
- **Live runs** — `GET /api/sessions/live` lists real model requests currently in flight (the dashboard polls it every 2 seconds). Send `X-Odo-Session: <conversation-id>` to group concurrent calls from one conversation; each request still receives its own `run_id`.
- **Shadow pricing** — local models can carry a `shadow_cost` priced at a cloud equivalent; the dashboard shows "Saved vs Cloud" when the two differ.
- **Privacy** — odotoken stores usage metadata, not prompt or response content. Imported messages are never parsed. No PII redactor is currently applied, so do not treat the database as scrubbed.
- **Freeform tags** — send `X-Odo-Tags: prod,critical`; segment the dashboard with the **Tag** filter.
- **Throughput & forecast** — `GET /api/tps` powers the Throughput view; `GET /api/forecast` fits a linear trend to daily cost and flags anomalies on the Overview card.
- **Dashboard auth** — `server.api_key` or `ODO_API_KEY` gates `/api/*` and `/v1/*` with Bearer or `X-Api-Key`; an empty key leaves them open on the loopback-only default.

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

## Development

```bash
make build                                 # compiles ./cmd/odotoken → bin/odotoken
make test                                  # go test -race ./...
cd frontend && pnpm install && pnpm build   # build dashboard → internal/server/static/
```

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).

## Contributing

PRs welcome — run the test suite and keep `prices.json` regenerated via the sync script rather than hand-edited (except `"override": true` entries).