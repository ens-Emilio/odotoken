# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0] - 2026-10-21
### Added
- Local CLI session import (`odotoken sessions scan|import|stats`) — reads `~/.omp/stats.db`, `~/.claude/projects/**/*.jsonl`, `~/.codex/sessions/**`, `~/.local/share/crush/**`; re-priced via `prices.json`, deduped by `UNIQUE(source, session_file, entry_id)`, incremental `--since` + `--cli` filters (49k+ rows verified)
- Dashboard Source filter (All / Proxy / Imported / import:omp …) + `GET /api/stats?source=` + `POST /api/sessions/import` + Overview "By Source" card
- `sessions` section in `config.json` and `src/odotoken/infra/config.py` (`enabled`, `paths` per CLI)
- Project tags (`X-Odo-Project` / `X-Project` header) — `project_id` column, `get_summary_by_project`, dashboard project filter, `odotoken stats --by-project`
- Shadow pricing (`shadow_cost` column, `PricingCalculator.calculate_shadow_cost`, `cloud_equivalent` in catalog, "Saved vs Cloud" KPI)
- PII redaction for logs (`src/odotoken/infra/redact.py`, `privacy.redact_logs` + `privacy.custom_patterns`) and opt-in prompt preview (`privacy.store_prompts` → SHA-256 hash + 200-char redacted preview)
- Prometheus text-format metrics at `GET /metrics` on the proxy port, Grafana dashboard + `docker-compose.observability.yml` + `prometheus.yml`
- Mock provider (`mock://local` upstream, synthetic metered responses) and chaos middleware (`chaos.enabled`, `rate`, `errors`, `latency_ms`; bypass via `X-Odo-No-Chaos`)
- Budget guard `monthly_tokens_limit` + `webhook_url` (notified on `warning`/`breached` via `PATCH /api/guard/limits`, also persisted to `config.json`)
- UI overhaul — AA contrast tokens, chart polish, sortable tables, sidebar/mobile polish

### Changed
- Config `limits` now uses `daily_cost_limit_usd` / `monthly_cost_limit_usd` / `daily_tokens_limit` / `monthly_tokens_limit` / `webhook_url` (legacy `monthly_budget_usd` etc. still read for back-compat)
- Proxy now extracts `X-Odo-Project` per request and passes `source` (proxy vs import:*) through to `db.get_summary` / `get_recent_events`
- Test suite expanded to 125 tests (privacy, project/shadow, prometheus/chaos, token counter, CLI)

### Fixed
- Eliminated redundant `GET /api/guard/limits` handler (duplicate `return`) and restored correct `GuardLimits` init
- Verified no secrets leak: no API keys/tokens in committed code, `.env` never tracked, `odotoken.db` / WAL / `.venv` / `PLAN*.md` properly ignored

## [0.1.0] - 2026-10-10
### Added
- Token counter service with multiple strategies (heuristic, remote provider APIs)
- HTTP API endpoints for token counting (/v1/count, /v1/count/batch, /v1/models)
- Health and metrics endpoints (/health, /ready, /metrics)
- CLI commands for token counting (odotoken count text/chat/batch, odotoken models)
- Pre-flight token counting middleware for proxy integration
- Pre-flight and post-flight token headers on proxy responses
- Model catalog with aliases and counting method configuration
- LRU cache with TTL for token counting results
- Comprehensive test suite (67 tests: unit, integration, CLI, golden)
- Documentation (README) and usage examples
- Optional dependencies for local tokenizers, metrics, OpenTelemetry
- GPL-3.0-or-later license

### Changed
- Updated proxy to include optional pre-flight token counting middleware
- Enhanced CLI with serve command for token counter API
- Improved error handling and validation

## [0.0.1] - 2026-10-09
### Added
- Initial project structure with proxy, parser, pricing, guard, db, dashboard, CLI
- Basic token metering via response parsing (JSON + SSE)
- Budget guard with alert/block functionality
- Pricing calculator for multiple providers
- Web dashboard for usage statistics
- CLI for proxy management (start/stop/status/stats/log)