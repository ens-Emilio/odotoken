# odotoken

Transparent AI token metering proxy between your CLI agents and LLM providers. Measures usage, calculates real cost, enforces budget limits.

**v0.7.0** · Dashboard in 20 languages · local session import · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [all quick starts](docs/i18n/README.md)

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
| `server.cors_allowlist` | allowed browser origins | empty |
| `providers.<name>.upstream_url` | path-prefix route target | — |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | cost limits, `0` disables | `0` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | token limits, `0` disables | `0` |
| `guard.warning_threshold_pct` | warning threshold | `80` |
| `guard.action_on_breach` | `alert` or `block` (429) | `block` |
| `guard.timezone` | IANA zone used for daily/monthly windows | `UTC` |
| `limits.rate_limit` | `enabled`, `rps`, `burst_size` | disabled |
| `alerts.slack_url` / `discord_url` / `telegram` / `email` | breach notifications | empty |
| `sessions.enabled` / `sessions.paths` | local import sources | `true` / empty |
| `--db` flag | SQLite file (WAL) | `odotoken.db` |

The port is a flag, not a config key: `--port` defaults to `8080`, and `--host` overrides `server.bind_host`. The legacy top-level `limits` budget block is still read and folded into `guard`, so an older config keeps its limits.

API keys never live in this file. Your CLI sends `Authorization` or `x-api-key`, and the proxy forwards the header untouched.

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

## CLI

```bash
odotoken start [--host] [--port] [--config] [--db]
odotoken stop
odotoken status
odotoken stats
odotoken import [cli...] [--since RFC3339] [--cli NAME] [--dry-run] [--discover] [--json] [--lang LOCALE]
```

`--lang` picks the language of human-readable output. Without it the CLI reads `LC_ALL`, then `LC_MESSAGES`, then `LANG`, and falls back to English. `--json` output is never translated.

`import` with no arguments reads every detected CLI. `--discover` lists what exists on this machine without writing anything. Costs are recomputed from `prices.json` rather than trusted from the source file.

## HTTP API

Proxy (`:8080`):

```
GET  /health
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
GET  /v1/health
```

## Troubleshooting

A CLI failing with `ECONNREFUSED 127.0.0.1:8080` means the proxy is down, not the provider:

```bash
odotoken status
curl http://127.0.0.1:8080/health
```

A response carrying an HTTP status (503 and so on) is an upstream problem. A connection error with no status is the local proxy.

## Security

- Single process — SQLite and the in-memory cache assume one worker
- Binds to `127.0.0.1` by default
- Keys pass through headers; only tokens, model, client, cost and latency are stored
- Put it behind authentication and a reverse proxy before exposing it on a network

## Development

```bash
make build
make test
cd frontend && pnpm install && pnpm test && pnpm build
```

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).
