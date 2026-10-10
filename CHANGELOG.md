# Changelog

All notable changes to this project will be documented in this file.

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