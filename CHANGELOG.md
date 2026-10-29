# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.7.0] - 2026-10-28
### Added
- Dashboard localization in 20 languages, including right-to-left layout for Arabic and Persian
- Local session import shared by the dashboard Import view, `odotoken import`, and `/api/import/*`
- Import history with per-CLI inserted/duplicate counts, tokens, cost, and terminal status
- `--lang` on `odotoken import`, falling back to `LC_ALL`, `LC_MESSAGES`, then `LANG`

### Changed
- CLI table columns pad by display width, so CJK, Devanagari, and Arabic labels no longer misalign
- `--json` output is never translated

### Fixed
- `zh_TW` no longer collapses to `zh-CN`, and Traditional Chinese keeps its script
- Currency totals in import reports keep their full precision instead of compacting to `$1k`
## [0.6.0] - 2026-10-28
### Added
- 21 color palettes inspired by native fruits from Acre, Brazil — each palette extracts real tones from skin, pulp and leaves of Amazonian fruits (Açaí, Cupuaçu, Pupunha, Tucumã, Bacuri, Biribá, Buriti, Bacaba, Patauá, Ingá, Taperebá, Abiu, Uxi, Araçá-boi, Banana, Abacaxi, Graviola, Maracujá, Manga, Mamão, Melancia)
- Click-controlled palette selector in the dashboard header with colored circle icons replacing emojis
- Beautiful trilingual READMEs (PT-BR, EN, ES) with full documentation of the fruit-inspired color system

### Changed
- Filter labels updated from months to days ("90 days" / "180 days" instead of "3 months" / "6 months") for consistency
- Palette dropdown switched from hover-based (group-hover) to click-controlled with mousedown-outside listener for stability

### Fixed
- Calendar heatmap now recalculates the 35-day window on every render using a stable midnight-based date string, ensuring today is always included and days 22-27 of October display correctly
- Removed frozen useMemo dependency that caused the calendar query key to change every millisecond, breaking React Query cache stability

## [0.5.0] - 2026-10-25
### Added
- Real in-flight run tracking through `active_runs`, `GET /api/sessions/live`, and an independently polled Live Runs panel
- `X-Odo-Session` request header for grouping concurrent model calls by conversation while retaining a unique `run_id`
- Lightweight `/api/guard/status` and `/api/filters` endpoints; CSV and SVG snapshot exports
- Budget transition history in the dashboard, guard timing based on an explicit IANA timezone, and `alert`/`block` breach behavior
- Pricing coverage warnings for models missing from the local price catalog
- Frontend Vitest coverage, actionable empty/error states, keyboard-sortable tables, and reduced-motion support

### Changed
- Replaced the legacy Python service with the complete Go runtime and rebuilt the shipped dashboard
- Hardened network defaults, API authentication, CORS, request-body limits, and outbound alert timeouts
- Made dashboard filters stable across 7-day/30-day ranges and split expensive aggregate loading from shell status/filter requests
- Reduced polling pressure for historical windows and added composite SQLite indexes

### Fixed
- Correctly account for exact provider usage on OpenAI-compatible custom providers
- Compute local-day windows in the configured timezone instead of mixing local labels with UTC boundaries
- Surface storage/query partition failures instead of rendering misleading zero or partial data
- Remove the unused token counter helper that blocked the CI lint job

## [0.4.0] - 2026-10-23
### Changed
- Added the clean filter bar, dark mode contrast fixes, and Go 1.26 CI toolchain fixes published in the prior GitHub release

## [0.3.0] - 2026-10-22
### Added
- TPS metrics: `get_tps_series`/`get_tps_summary`/`get_tps_by_model`/`get_tps_by_project` in `db.py`, `GET /api/tps`, TPS KPIs + ThroughputView, TTFT/streaming (`ttfb_ms`/`streaming_ms` columns, TTFT column in EventsView + ticker), `odotoken stats --tps`
- Guard per-project budgets (`limits.projects` with per-project daily/monthly cost/token caps, `PATCH /api/guard/limits` merge + per-project alerts)
- Guard alerts to Slack/Discord/Telegram/SMTP (`dispatch_alerts`) + `GET /api/export?format=csv` + `token bucket rate limit por project_id`
- `odotoken replay` (re-price historical events with current prices, `--apply` to write, `shadow_cost` sim)
- Dashboard auth (Bearer token / Basic, `config.auth`, `_check_auth` middleware)
- `X-Odo-Tags`/`by_tag` filter (tags column + index, `get_summary_by_tag`, dashboard tag filter, tag-filtered CSV)

### Changed
- Replay/`shadow_cost` diff surfaces in CLI + dashboard; `PATCH /api/guard/limits` now merges fields instead of replacing
- `optimize` surfaces per-model ratio/cache-read notes; fallback triggers on non-auth 4xx and 5xx

### Fixed
- Restored `semantic_cache_get`/`semantic_cache_set` import/lint integrity in `db.py` and `PATCH /api/guard/limits` merge structure

## [0.2.0] - 2026-10-21
### Added
- Local CLI session import (`odotoken sessions scan|import|stats`) — reads `~/.omp/stats.db`, `~/.claude/projects/**/*.jsonl`, `~/.codex/sessions/**`, `~/.local/share/crush/**`; re-priced via `prices.json`, deduped by `UNIQUE(source, session_file, entry_id)`, incremental `--since` + `--cli` filters
- Dashboard Source filter (All / Proxy / Imported / import:omp …) + `GET /api/stats?source=` + `POST /api/sessions/import` + Overview "By Source" card
- `sessions` section in `config.json` and `src/odotoken/infra/config.py` (`enabled`, `paths` per CLI)
- Project tags (`X-Odo-Project` / `X-Project` header) — `project_id` column, `get_summary_by_project`, dashboard project filter, `odotoken stats --by-project`
- Shadow pricing (`shadow_cost` column, `PricingCalculator.calculate_shadow_cost`, `cloud_equivalent` in catalog, "Saved vs Cloud" KPI)
- Usage-only import and proxy metadata storage; prompt and response content are not parsed. Earlier releases advertised a Python PII redactor that is not present in the Go implementation.
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