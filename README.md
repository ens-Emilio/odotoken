# odotoken

Transparent AI token metering proxy between your CLI agents and LLM providers. Measures usage, calculates real cost, enforces budget limits.

**v0.8.0** · Dashboard in 20 languages · local session import · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [all quick starts](docs/i18n/README.md)

## How it works

```
CLI agents → odotoken :8080 → configured providers
                  │
                  ├─ metering (JSON + SSE)
                  ├─ cost from prices.json
                  ├─ budget guard (alert / block 429)
                  └─ SQLite history → dashboard (/app/)
```

Every key under `providers` becomes a route. A request to `/openai/chat/completions` goes to that provider's `upstream_url` with the prefix stripped; a request with no matching prefix falls back to provider detection. Tokens are read from JSON and SSE bodies, and each request is stored with prompt, completion and cache tokens, cost, latency and status.

## Installation

Pre-built binaries are at [Releases](https://github.com/ens-Emilio/odotoken/releases/latest). From source:

```bash
git clone https://github.com/ens-Emilio/odotoken.git && cd odotoken
go build -o bin/odotoken ./cmd/odotoken
cd frontend && pnpm install && pnpm build
```

The dashboard is read from `internal/server/static` relative to the working directory, so start the binary from the project root or from an install directory that contains it.

## Configuration

```bash
cp config.json.example config.json
```

| Key | Meaning | Default |
|-----|---------|---------|
| `server.bind_host` | bind address | `127.0.0.1` |
| `server.api_key` | require a key on `/api` and `/v1` | empty (open) |
| `server.client_keys` | client key (the `odt_…` secret itself) → `client`, `project`, optional quotas; written by `PUT /api/client-keys` and the dashboard, always shown masked | empty |
| `server.cors_allowlist` | allowed browser origins | empty |
| `providers.<name>.fallbacks` / `fallback_on_status` | retry another provider on transient statuses (default 429/500/502/503/504) | none |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | cost limits, `0` disables | `0` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | token limits, `0` disables | `0` |
| `guard.warning_threshold_pct` | warning threshold | `80` |
| `guard.action_on_breach` | `alert` or `block` (429) | `block` |
| `guard.timezone` | IANA zone used for daily/monthly windows | `UTC` |
| `guard.projects.<id>` | per-project overrides of any guard field | inherit global |
| `limits.rate_limit` | `enabled`, `rps`, `burst_size`, plus `projects`/`clients` overrides keyed by `X-Odo-Project` / `X-Odo-Client` | disabled |
| `alerts.webhook_url` / `alerts.slack_url` / `alerts.discord_url` / `alerts.telegram` / `alerts.email` | breach notifications | empty |
| `sessions.enabled` / `sessions.paths` | local import sources | `true` / empty |
| `alerts.retry` | durable outbox backoff: `max_attempts`, `base_delay_seconds`, `max_delay_seconds` | 5 / 30 / 3600 |
| `storage.token_events_retention_days` / `vacuum_after_retention` | delete old events, optionally VACUUM | `0` (off) / `false` |
| `scheduled_import.enabled` / `interval` / `clis` | import detected CLIs while the daemon runs | `false` / `24h` |
| `otel.enabled` / `endpoint` / `interval_seconds` | push OTLP/HTTP JSON metrics to `endpoint/v1/metrics` while the daemon runs | `false` / empty / `60` |
| `--db` flag | SQLite file (WAL) | `odotoken.db` |

The port is a flag, not a config key: `--port` defaults to `8080`, and `--host` overrides `server.bind_host`. The legacy top-level `limits` budget block is still read and folded into `guard`, so an older config keeps its limits.

Provider keys never live in this file: your CLI sends `Authorization` or `x-api-key` and the proxy forwards the header untouched. A key listed under `server.client_keys` is resolved locally first — it sets the project and client, and may carry its own quotas. Those client keys are working secrets in `config.json` (written `0600`); keep the file private and revoke a leaked key with `DELETE /api/client-keys/{masked-key}`. The API only ever shows them masked.

Both limits are always checked, and each is measured where it applies: the
project limits against everything the project spent, a key's quota against that
key's own spend inside the project. A key inside its quota is still blocked when
the project as a whole is over budget, and a key without a quota cannot spend
past the project limits.

```bash
odotoken start --config config.json --db odotoken.db
```

Dashboard: `http://127.0.0.1:8080/app/` — usage appears within 10s. It runs in 20 languages via Settings → Language or `?lang=pt-BR` on a shared link; Arabic and Persian switch the layout to right-to-left.

## Features

- Transparent metering — no change to the CLI workflow
- Real pricing — 2,700+ models in `prices.json`, refreshed by `node scripts/sync-prices.mjs`
- Budget guard — daily and monthly cost/token limits, alert or block with 429
- Dashboard — KPIs, charts, sortable tables, event stream, light/dark/system themes, 21 color palettes
- Throughput — TPS and TTFT per model (`GET /api/tps`)
- Forecast and anomalies — linear projection plus anomaly detection
- Alerts — Slack, Discord, Telegram, SMTP on breach; CSV and SVG export
- Live runs — in-flight requests grouped by conversation with `X-Odo-Session`
- Local session import — the Import view and `odotoken import` share one engine and read sessions already on disk from the CLIs listed under `sessions.paths`
- Import history — per-CLI inserted and duplicate counts, tokens, cost and terminal status

- Prometheus — `GET /metrics` exposes spend, tokens and request counters for scraping; OTLP/HTTP push is optional
- Resilience — provider fallback on configured 429/5xx, per-project/client rate limits, durable alert retry, scheduled imports
- Operations — `odotoken doctor` checks config, database, alerts and schedules; `odotoken export` and `odotoken prices` work without the daemon

## CLI

```bash
odotoken start [--host] [--port] [--config] [--db]
odotoken stop
odotoken status
odotoken stats
odotoken export [--format csv|json] [--since RFC3339] [--until RFC3339] [--project ID] [--source NAME] [--tag TAG] [--limit N] [--out PATH|-] [--json]
odotoken prices list [--query TEXT] [--limit N] [--json]
odotoken prices set --model NAME --input N --output N [--cache-read N] [--cache-write N]
odotoken prices delete --model NAME
odotoken import [cli...] [--since RFC3339] [--cli NAME] [--dry-run] [--discover] [--json] [--lang LOCALE]
odotoken doctor [--config] [--db] [--json] [--flush-alerts] [--vacuum]
odotoken mcp [--config] [--db]
```

`--lang` picks the language of human-readable output. Without it the CLI reads `LC_ALL`, then `LC_MESSAGES`, then `LANG`, and falls back to English. `--json` output is never translated.

`import` with no arguments reads every detected CLI. `--discover` lists what exists on this machine without writing anything. Costs are recomputed from `prices.json` rather than trusted from the source file.

`export` reads the database directly, so it works with the daemon stopped. It uses the same query and CSV contract as `GET /api/events.csv`. `--out -` writes to stdout; with `--out FILE` the `--json` flag prints a row-count summary instead of the rows.

`prices` edits `prices.json` (`--prices` to point elsewhere). `list` marks each row `override` or `builtin`. `delete` only removes overrides — a built-in rate is refused, because the model would fall back to $0 and stop being metered.

## HTTP API

Proxy (`:8080`):

```
GET  /health             # liveness of odotoken itself: no key, never proxied
GET  /metrics            # Prometheus text exposition
GET  /api/events.csv
GET  /api/sessions
GET  /api/sessions/{id}
GET  /api/cache-savings
POST /api/simulate
GET|PUT|DELETE /api/prices
GET|PUT /api/client-keys
DELETE /api/client-keys/{masked-key}
GET  /api/anomalies
GET  /api/stats?since=ISO&project=&source=&tag=&tz=
GET  /api/stats.csv
GET  /api/stats.svg
GET  /api/stats/daily-detail
GET  /api/guard/status
GET  /api/guard/limits
PATCH /api/guard/limits
GET  /api/filters
GET  /api/tps
GET  /api/forecast
GET  /api/sessions/live
GET  /api/events
GET  /api/import/sources
GET  /api/import/status
GET  /api/import/history
POST /api/import/run
POST /api/import/cancel
POST /api/import/path
```

Standalone counting (`/v1`):

```
POST /v1/count        {"model","text"}
POST /v1/count/batch  {"items":[...]}
GET  /v1/models
GET  /v1/health       # API health; requires the key when server.api_key is set
```

## Troubleshooting

A CLI failing with `ECONNREFUSED 127.0.0.1:8080` means the proxy is down, not the provider:

```bash
odotoken status
curl http://127.0.0.1:8080/health
```

A response carrying an HTTP status (503 and so on) is an upstream problem. A connection error with no status is the local proxy.

`/health` is the one to probe from a monitor or a `systemd` check: it needs no
key and never reaches the proxy, so it still answers while a budget is breached
or the upstream is down. `/v1/health` is the authenticated API surface and is
only there to prove the key works.

## Security

- Single process — SQLite and the in-memory cache assume one worker
- Binds to `127.0.0.1` by default
- Keys pass through headers; only tokens, model, client, cost and latency are stored
- `server.client_keys` holds working secrets in `config.json` (written `0600`); keep the file private and rotate a key through `DELETE /api/client-keys/{masked-key}` when it leaks
- Put it behind authentication and a reverse proxy before exposing it on a network

## Development

```bash
make build
make test
cd frontend && pnpm install && pnpm test && pnpm build
```

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).
