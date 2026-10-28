# odotoken

Proxy transparente de metering de tokens de IA entre seus agentes CLI e os provedores LLM. Mede uso, calcula custo real e impõe limites de orçamento.

**v0.6.0** · 21 paletas de cores inspiradas em frutas do Acre · [English](README.en.md) · [Español](README.es.md)

## Como funciona

```
agentes CLI → odotoken :8090 → seus proxies/routers → provedores LLM
                  │
                  ├─ metering (JSON + SSE)
                  ├─ custo real (2.700+ modelos)
                  ├─ guarda de orçamento (alerta / bloqueio 429)
                  └─ histórico SQLite → dashboard React (/app/)
```

Roteamento por prefixo de path: cada chave em `providers` vira uma rota (`/anthropic/...`, `/openai/...`, ou qualquer custom). Tokens extraídos de JSON e SSE. Cada requisição gravada com prompt/completion/cache tokens, custo, latência e status.

## Instalação

Binário pronto em [Releases](https://github.com/ens-Emilio/odotoken/releases/latest), ou do fonte:

```bash
git clone https://github.com/ens-Emilio/odotoken.git && cd odotoken
go build -o bin/odotoken ./cmd/odotoken        # Go 1.26+
cd frontend && pnpm install && pnpm build      # opcional; releases já vêm com dashboard
```

## Configuração mínima

```bash
cp config.json.example config.json
```

```jsonc
{
  "proxy": { "host": "127.0.0.1", "port": 8090 },
  "providers": {
    "anthropic": { "upstream_url": "https://api.anthropic.com" },
    "openai":    { "upstream_url": "https://api.openai.com" }
  },
  "guard": {
    "daily_cost_limit_usd": 10,
    "monthly_cost_limit_usd": 100,
    "action_on_breach": "block"
  },
  "database": { "path": "odotoken.db" }
}
```

Chaves de API nunca ficam no config — seus CLIs enviam `Authorization`/`x-api-key` e o proxy repassa intacto.

Aponte seus CLIs para `http://127.0.0.1:8090`:

| CLI | Como |
|-----|------|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | `baseURL: "http://127.0.0.1:8090/<chave>"` |

```bash
odotoken start                # proxy + dashboard
# ou como serviço:
odotoken install-service && systemctl --user enable --now odotoken
```

Dashboard em http://127.0.0.1:8090/app/ — uso aparece em até 10s.

## Paletas de cores

21 paletas extraídas dos tons reais de casca, polpa e folhas de frutas nativas do Acre (Açaí, Cupuaçu, Pupunha, Tucumã, Bacuri, Biribá, Buriti, Bacaba, Patauá, Ingá, Taperebá, Abiu, Uxi, Araçá-boi, Banana, Abacaxi, Graviola, Maracujá, Manga, Mamão, Melancia). Seletor no header do dashboard (círculo colorido ao lado do tema), persistido em `localStorage`, contraste AA nos modos claro e escuro.

## Funcionalidades

- Metering transparente — zero mudança no fluxo dos CLIs
- Preços reais — 2.700+ modelos, sync via `node scripts/sync-prices.mjs`
- Guarda de orçamento — limites diários/mensais de custo e tokens, alerta ou bloqueio (429)
- Dashboard React — KPIs, gráficos, tabelas ordenáveis, stream de eventos, temas claro/escuro/sistema
- CLI — `odotoken stats --by-model`, `log`, `count`, `models`, `sessions`
- API de contagem standalone — `odotoken serve` na porta 8081
- Throughput — TPS/TTFT por modelo (`GET /api/tps`)
- Replay & optimize — reprecifica histórico, sugere modelos mais baratos
- Tags & forecast — `X-Odo-Tags`, forecast linear + detecção de anomalias
- Alertas — Slack/Discord/Telegram/SMTP em breach; export CSV/SVG
- Runs live — `GET /api/sessions/live`; `X-Odo-Session` agrupa chamadas de uma conversa
- Import de sessões locais — `odotoken sessions import` lê OMP/Claude/Codex/Crush já salvos na máquina

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

Proxy (porta 8090):

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

Contagem standalone (porta 8081):

```
POST /v1/count         {"model","text"}
POST /v1/count/batch   {"items":[...]}
GET  /v1/models
GET  /health | /ready | /metrics
```

## Configuração

| Chave | Descrição | Padrão |
|-------|-----------|--------|
| `proxy.host` / `proxy.port` | bind | `127.0.0.1:8090` |
| `providers.<k>.upstream_url` | upstream da rota por prefixo | — |
| `proxy.upstream_openai` / `upstream_anthropic` | fallback para requisições auto-detectadas | APIs oficiais |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | limites de custo (0 = off) | `null` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | limites de tokens (0 = off) | `null` |
| `guard.warning_threshold_pct` | % para warning | `80` |
| `guard.action_on_breach` | `alert` ou `block` (429) | `block` |
| `guard.timezone` | fuso IANA dos limites | UTC |
| `limits.webhook_url` | webhook em warning/breach | `null` |
| `database.path` | SQLite WAL | `odotoken.db` |
| `token_counter.enabled` | middleware pré-flight | `true` |
| `sessions.enabled` / `sessions.paths` | import local | `true` |

Overrides de preço: `"override": true` numa entrada de `prices.json` impede o sync de sobrescrever.

## Troubleshooting

Todos os CLIs falham com `ECONNREFUSED 127.0.0.1:8090` → proxy caiu, não os providers.

```bash
odotoken status
curl http://127.0.0.1:8090/health
journalctl --user -u odotoken -n 50 --no-pager
```

- `status=203/EXEC` → `ExecStart` aponta para path inexistente; rode `odotoken install-service` da raiz do projeto
- `activating (auto-restart)` loop → leia o journal, corrija a causa, `systemctl --user restart odotoken`
- `/health` OK mas CLIs falham → configs dos CLIs stale; reinicie o CLI

Regra: erro com status HTTP (503 etc.) → problema upstream. Erro de conexão sem status → proxy caiu.

## Segurança

- Processo único — SQLite + cache in-memory assumem 1 worker
- Bind em `127.0.0.1` por padrão
- Chaves passam nos headers; nada é logado além de tokens, modelo, client, custo e latência
- Coloque atrás de auth/reverse-proxy antes de expor em rede

## Desenvolvimento

```bash
make build
make test
cd frontend && pnpm install && pnpm build
```

## Licença

GPL-3.0-or-later. Ver [LICENSE](LICENSE).