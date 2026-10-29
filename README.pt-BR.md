# odotoken

Proxy transparente de metering de tokens de IA entre seus agentes CLI e os provedores LLM. Mede uso, calcula custo real e impõe limites de orçamento.

**v0.7.0** · dashboard em 20 idiomas · importação de sessões locais · [English](README.md) · [Español](README.es.md) · [todos os quick starts](docs/i18n/README.md)

## Como funciona

```
agentes CLI → odotoken :8080 → provedores configurados
                  │
                  ├─ metering (JSON + SSE)
                  ├─ custo a partir de prices.json
                  ├─ guarda de orçamento (alerta / bloqueio 429)
                  └─ histórico SQLite → dashboard (/app/)
```

Cada chave em `providers` vira uma rota. Uma requisição para `/openai/chat/completions` vai para o `upstream_url` daquele provedor com o prefixo removido; sem prefixo correspondente, o proxy detecta o provedor pelo corpo. Tokens são lidos de JSON e SSE, e cada requisição é gravada com tokens de prompt, completion e cache, custo, latência e status.

## Instalação

Binário pronto em [Releases](https://github.com/ens-Emilio/odotoken/releases/latest). Do fonte:

```bash
git clone https://github.com/ens-Emilio/odotoken.git && cd odotoken
go build -o bin/odotoken ./cmd/odotoken
cd frontend && pnpm install && pnpm build
```

O dashboard é lido de `internal/server/static` relativo ao diretório de trabalho, então inicie o binário da raiz do projeto ou de um diretório de instalação que contenha essa pasta.

## Configuração

```bash
cp config.json.example config.json
```

| Chave | Significado | Padrão |
|-------|-------------|--------|
| `server.bind_host` | endereço de bind | `127.0.0.1` |
| `server.api_key` | exige chave em `/api` e `/v1` | vazio (aberto) |
| `server.cors_allowlist` | origens de navegador permitidas | vazio |
| `providers.<nome>.upstream_url` | destino da rota por prefixo | — |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | limites de custo, `0` desliga | `0` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | limites de tokens, `0` desliga | `0` |
| `guard.warning_threshold_pct` | limiar de aviso | `80` |
| `guard.action_on_breach` | `alert` ou `block` (429) | `block` |
| `guard.timezone` | fuso IANA das janelas diária/mensal | `UTC` |
| `limits.rate_limit` | `enabled`, `rps`, `burst_size` | desligado |
| `alerts.slack_url` / `discord_url` / `telegram` / `email` | avisos de breach | vazio |
| `sessions.enabled` / `sessions.paths` | fontes de importação local | `true` / vazio |

A porta é uma flag, não uma chave de config: `--port` tem padrão `8080` e `--host` sobrepõe `server.bind_host`. O bloco legado `limits` de orçamento ainda é lido e dobrado em `guard`, então um config antigo mantém seus limites.

Chaves de API nunca ficam nesse arquivo. Seu CLI envia `Authorization` ou `x-api-key` e o proxy repassa o header intacto.

```bash
odotoken start --config config.json --db odotoken.db
```

Dashboard: `http://127.0.0.1:8080/app/` — o uso aparece em até 10s. Ele roda em 20 idiomas via **Ajustes → Idioma** ou `?lang=pt-BR` num link compartilhado; árabe e persa invertem o layout para RTL.

## Funcionalidades

- Metering transparente — zero mudança no fluxo dos CLIs
- Preços reais — 2.700+ modelos em `prices.json`, atualizados por `node scripts/sync-prices.mjs`
- Guarda de orçamento — limites diários e mensais de custo e tokens, alerta ou bloqueio com 429
- Dashboard — KPIs, gráficos, tabelas ordenáveis, stream de eventos, temas claro/escuro/sistema, 21 paletas de cores
- Throughput — TPS e TTFT por modelo (`GET /api/tps`)
- Forecast e anomalias — projeção linear mais detecção de anomalias
- Alertas — Slack, Discord, Telegram, SMTP em breach; export CSV e SVG
- Runs live — requisições em voo agrupadas por conversa com `X-Odo-Session`
- Import de sessões locais — a tela Importar e `odotoken import` usam um único mecanismo e leem sessões já em disco dos CLIs listados em `sessions.paths`
- Histórico de importação — contagens por CLI, inseridos e duplicados, tokens, custo e status terminal

## CLI

```bash
odotoken start [--host] [--port] [--config] [--db]
odotoken stop
odotoken status
odotoken stats
odotoken import [cli...] [--since RFC3339] [--cli NOME] [--dry-run] [--discover] [--json] [--lang LOCALE]
```

`--lang` escolhe o idioma da saída humana. Sem ele, a CLI lê `LC_ALL`, depois `LC_MESSAGES`, depois `LANG`, e cai no inglês. A saída `--json` nunca é traduzida.

`import` sem argumentos lê todo CLI detectado. `--discover` lista o que existe na máquina sem gravar nada. Os custos são recalculados a partir de `prices.json` em vez de confiados ao arquivo de origem.

## API HTTP

Proxy (`:8080`):

```
GET   /health
GET   /api/stats?since=ISO&project=&source=&tag=&tz=
GET   /api/stats.csv
GET   /api/stats.svg
GET   /api/stats/daily-detail
GET   /api/guard/status
GET   /api/guard/limits
PATCH /api/guard/limits
GET   /api/filters
GET   /api/tps
GET   /api/forecast
GET   /api/sessions/live
GET   /api/events
GET   /api/import/sources
GET   /api/import/status
GET   /api/import/history
POST  /api/import/run
POST  /api/import/cancel
POST  /api/import/path
```

Contagem standalone (`/v1`):

```
POST /v1/count        {"model","text"}
POST /v1/count/batch  {"items":[...]}
GET  /v1/models
GET  /v1/health
```

## Troubleshooting

Um CLI falhando com `ECONNREFUSED 127.0.0.1:8080` significa que o proxy caiu, não o provedor:

```bash
odotoken status
curl http://127.0.0.1:8080/health
```

Uma resposta com status HTTP (503 e afins) é problema do upstream. Erro de conexão sem status é o proxy local.

## Segurança

- Processo único — SQLite e o cache in-memory assumem um worker
- Bind em `127.0.0.1` por padrão
- Chaves passam nos headers; só tokens, modelo, client, custo e latência são gravados
- Coloque atrás de autenticação e reverse proxy antes de expor em rede

## Desenvolvimento

```bash
make build
make test
cd frontend && pnpm install && pnpm test && pnpm build
```

## Licença

GPL-3.0-or-later. Veja [LICENSE](LICENSE).
