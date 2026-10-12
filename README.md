# odotoken

Transparent AI token metering proxy, usage tracker and budget guard.

## Overview

odotoken is a lightweight, high-performance reverse proxy that provides transparent token metering for AI providers (OpenAI, Anthropic, Google Gemini, Ollama) with budget enforcement, usage analytics, and a standalone token counting API.

## Features

- **Transparent Proxy**: Intercepts LLM API calls and provides token usage analytics
- **Budget Controls**: Set daily/monthly cost/token limits with alert/block actions
- **Usage Analytics**: Real-time dashboard with historical usage by model/client
- **Token Counter API**: Standalone service for counting tokens before sending to LLMs
- **Multiple Strategies**: Remote (provider APIs), local (tokenizers), and heuristic counting
- **Extensible Design**: Pluggable adapter architecture for custom tokenizers
- **Modern Stack**: Python 3.13, FastAPI, Uvicorn, httpx, Typer, Rich, pytest, uv

## Architecture

```
┌─────────────────┐    ┌────────────────────┐    ┌──────────────────┐
│   HTTP Request  │───▶│  odotoken Proxy    │───▶│   LLM Provider   │
│  (OpenAI API)   │    │  • Token Metering  │    │  (API Response)  │
└─────────────────┘    │  • Budget Checks   │    └──────────────────┘
                       │  • Usage Tracking  │
                       └────────────────────┘
                               │
                               ▼
                     ┌──────────────────┐
                     │   Usage DB       │◀───────┐
                     │  (SQLite + WAL)  │        │
                     └──────────────────┘        │
                               ▲                │
                               │                │
                     ┌──────────────────┐        │
                     │   Token Counter  │◀───────┘
                     │  (Standalone API)│
                     │  • Pre-flight    │
                     │  • Counting      │
                     │  • Strategies    │
                     └──────────────────┘
```

## Installation

```bash
# Install with pip
pip install odotoken

# Install with optional dependencies
pip install odotoken[local-tokenizers]  # For tiktoken, sentencepiece
pip install odotoken[all]               # All optional features
```

## Quick Start

### 1. Configuration

Create `config.json`:

```json
{
  "proxy": {
    "host": "127.0.0.1",
    "port": 8080,
    "upstream_openai": "https://api.openai.com",
    "upstream_anthropic": "https://api.anthropic.com",
    "upstream_gemini": "https://generativelanguage.googleapis.com",
    "upstream_ollama": "http://127.0.0.1:11434"
  },
  "limits": {
    "monthly_budget_usd": 100.0,
    "monthly_token_budget": 1000000,
    "alert_percent": 80,
    "block_on_budget_exceeded": true
  },
  "database": {
    "path": "odotoken.db"
  },
  "token_counter": {
    "enabled": true,
    "default_model": "gpt-4o-mini",
    "cache_enabled": true,
    "providers": {
      "openai": {
        "enabled": true,
        "api_key_env": "OPENAI_API_KEY",
        "base_url": "https://api.openai.com",
        "count_endpoint": "/v1/tokens/count"
      },
      "anthropic": {
        "enabled": true,
        "api_key_env": "ANTHROPIC_API_KEY",
        "base_url": "https://api.anthropic.com",
        "count_endpoint": "/v1/messages/count_tokens"
      },
      "gemini": {
        "enabled": true,
        "api_key_env": "GEMINI_API_KEY",
        "base_url": "https://generativelanguage.googleapis.com",
        "count_endpoint": "/v1/models/{model}:countTokens"
      }
    }
  }
}
```

### 2. Set Environment Variables

```bash
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="AIza..."
```

### 3. Start Services

```bash
# Start proxy with dashboard
odotoken start

# Start standalone token counter API
odotoken serve --port 8081

# Or run both in development
odotoken start --reload
```

## Usage Examples

### Token Counter API

```bash
# Count text
odotoken count text "Hello world"

# Count chat messages
echo '[{"role": "user", "content": "Hello"}]' | odotoken count chat --model gpt-4o-mini --file -

# Count from file
odotoken count text --file prompt.txt --model claude-3-5-sonnet

# Batch processing
odotoken count batch requests.jsonl

# List models
odotoken models list
odotoken models info gpt-4o

# JSON output
odotoken count text "test" --output json
```

### Direct API Usage

```bash
curl -X POST http://127.0.0.1:8081/v1/count \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Hello world"}'

curl http://127.0.0.1:8081/v1/models
curl http://127.0.0.1:8081/health
```

## Model Pricing

Cost tracking uses `prices.json` — **2,700+ models** with real per-1M-token rates,
synced from three sources (later wins):

| Source | Coverage | Priority |
|---------|----------|----------|
| [pricetoken](https://www.npmjs.com/package/pricetoken) npm package | 46 official models (OpenAI, Anthropic, Google) | 1 |
| Pricing API (default `http://154.29.76.173`, override with `PEGASUS_PRICING_API`) | 3,500+ models via public REST API | 2 |
| Router configs (`~/.local/share/crush/providers.json` if present) | exact prices your local routers charge | 3 (wins) |

Refresh prices at any time:

```bash
npm install pricetoken
node scripts/sync-prices.mjs
systemctl --user restart odotoken   # or restart your odotoken process
```

Manual overrides: add `"override": true` to any entry in `prices.json` and the sync
script will never touch it:

```json
"my-custom-model": { "input": 1.5, "output": 6.0, "cache_read": 0.2, "cache_write": 0, "override": true }
```

The public pricing API needs no auth key for reads. If you deploy your own instance,
point `PEGASUS_PRICING_API` at it. Rate limits: nginx 10 req/s per IP, app 60 req/min.

## Proxy Headers

When token counting is enabled, the proxy adds:

- `X-Preflight-Tokens`: Estimated tokens before forwarding
- `X-Preflight-Model`: Resolved model ID
- `X-Preflight-Method`: Counting method used (remote/local/heuristic)
- `X-Postflight-Tokens`: Actual tokens from provider response
- `X-Postflight-Prompt-Tokens`: Input tokens
- `X-Postflight-Completion-Tokens`: Output tokens

## Configuration Reference

### Proxy Section

| Setting | Description | Default |
|---------|-------------|---------|
| `proxy.host` | Bind address | `127.0.0.1` |
| `proxy.port` | Bind port | `8080` |
| `proxy.upstream_*` | Provider base URLs | See config |
| `limits.monthly_budget_usd` | Monthly cost limit | `0.0` (disabled) |
| `limits.monthly_token_budget` | Monthly token limit | `0` (disabled) |
| `limits.alert_percent` | Alert threshold (%) | `80` |
| `limits.block_on_budget_exceeded` | Block instead of alert | `false` |
| `database.path` | SQLite database file | `odotoken.db` |

### Token Counter Section

| Setting | Description | Default |
|---------|-------------|---------|
| `token_counter.enabled` | Enable token counting features | `true` |
| `token_counter.default_model` | Default model for resolution | `gpt-4o-mini` |
| `token_counter.default_provider` | Default provider | `openai` |
| `token_counter.max_request_tokens` | Max tokens per request | `1,000,000` |
| `token_counter.max_batch_items` | Max items in batch | `100` |
| `token_counter.max_batch_tokens` | Max tokens in batch | `5,000,000` |
| `token_counter.cache_enabled` | Enable response caching | `true` |
| `token_counter.cache_ttl_seconds` | Cache TTL | `3600` |
| `token_counter.cache_max_size` | Max cache entries | `10,000` |

### Provider Configuration

Each provider in `token_counter.providers` supports:

| Setting | Description | Required |
|---------|-------------|----------|
| `enabled` | Enable provider | `false` |
| `api_key_env` | Environment variable for API key | Yes (for remote) |
| `base_url` | Provider API base URL | Yes |
| `count_endpoint` | Token counting endpoint | Yes (for remote) |
| `timeout_seconds` | Request timeout | `30` |
| `max_retries` | Retry attempts | `3` |

## Development

```bash
# Install in development mode
pip install -e .

# Run tests
pytest

# Run tests with coverage
pytest --cov=odotoken --cov-report=term-missing

# Build package
pip install build
python -m build

# Check for outdated dependencies
pip list --outdated
```

## License

GPL-3.0-or-later. See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.