# odotoken

Proxy transparente de metering de tokens de IA entre seus agentes CLI e os provedores LLM. Mede uso, calcula custo real e impõe limites de orçamento.

**v0.8.0** · dashboard em 20 idiomas · importação de sessões locais · [English](README.md) · [Español](README.es.md) · [todos os quick starts](docs/i18n/README.md)

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
| `server.client_keys` | chaves de cliente com `client`, `project` e cotas opcionais; gerenciadas por `PUT /api/client-keys` e exibidas mascaradas | vazio |
| `server.cors_allowlist` | origens de navegador permitidas | vazio |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | limites de custo, `0` desliga | `0` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | limites de tokens, `0` desliga | `0` |
| `guard.warning_threshold_pct` | limiar de aviso | `80` |
| `guard.action_on_breach` | `alert` ou `block` (429) | `block` |
| `guard.timezone` | fuso IANA das janelas diária/mensal | `UTC` |
| `limits.rate_limit` | `enabled`, `rps`, `burst_size`, com `projects`/`clients` por `X-Odo-Project` / `X-Odo-Client` | desligado |
| `alerts.webhook_url` / `alerts.slack_url` / `discord_url` / `telegram` / `email` | avisos de breach | vazio |
| `sessions.enabled` / `sessions.paths` | fontes de importação local | `true` / vazio |
| `providers.<nome>.fallbacks` / `fallback_on_status` | repete em outro provider em 429/5xx (padrão 429/500/502/503/504) | nenhum |
| `guard.projects.<id>` | limites próprios por projeto | herda o global |
| `alerts.retry` | backoff da fila durável: `max_attempts`, `base_delay_seconds`, `max_delay_seconds` | 5 / 30 / 3600 |
| `storage.token_events_retention_days` / `vacuum_after_retention` | retenção de eventos e VACUUM opcional | `0` / `false` |
| `scheduled_import.enabled` / `interval` / `clis` | importação agendada com o daemon rodando | `false` / `24h` |
| `otel.enabled` / `endpoint` / `interval_seconds` | envia métricas OTLP/HTTP JSON para `endpoint/v1/metrics` com o daemon rodando | `false` / vazio / `60` |

A porta é uma flag, não uma chave de config: `--port` tem padrão `8080` e `--host` sobrepõe `server.bind_host`. O bloco legado `limits` de orçamento ainda é lido e dobrado em `guard`, então um config antigo mantém seus limites.

Chaves do provider nunca ficam nesse arquivo: seu CLI envia `Authorization` ou `x-api-key` e o proxy repassa o header intacto. As chaves de cliente em `server.client_keys` são segredos de trabalho no `config.json` (gravado como `0600`); mantenha o arquivo privado e revogue uma chave exposta com `DELETE /api/client-keys/{chave-mascarada}`.

Os dois limites são sempre verificados, e cada um é medido onde vale: os
limites do projeto contra tudo que o projeto gastou, a cota de uma chave contra
o gasto daquela chave dentro do projeto. Uma chave dentro da própria cota ainda
é bloqueada quando o projeto inteiro estourou, e uma chave sem cota não passa
dos limites do projeto.

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
- Prometheus — `GET /metrics` expõe contadores de custo, tokens e requisições; o envio OTLP/HTTP é opcional
- Resiliência — fallback de provider em 429/5xx configuráveis, rate limit por projeto/cliente, retry durável de alertas e importação agendada
- Operação — `odotoken doctor` verifica configuração, banco, alertas e agenda; `odotoken export` e `odotoken prices` funcionam sem o daemon

## CLI

```bash
odotoken start [--host] [--port] [--config] [--db]
odotoken stop
odotoken status
odotoken stats
odotoken export [--format csv|json] [--since RFC3339] [--until RFC3339] [--project ID] [--source NOME] [--tag TAG] [--limit N] [--out CAMINHO|-] [--json]
odotoken prices list [--query TEXTO] [--limit N] [--json]
odotoken prices set --model NOME --input N --output N [--cache-read N] [--cache-write N]
odotoken prices delete --model NOME
odotoken import [cli...] [--since RFC3339] [--cli NOME] [--dry-run] [--discover] [--json] [--lang LOCALE]
odotoken doctor [--config] [--db] [--json] [--flush-alerts] [--vacuum]
odotoken mcp [--config] [--db]
```

`--lang` escolhe o idioma da saída humana. Sem ele, a CLI lê `LC_ALL`, depois `LC_MESSAGES`, depois `LANG`, e cai no inglês. A saída `--json` nunca é traduzida.

`import` sem argumentos lê todo CLI detectado. `--discover` lista o que existe na máquina sem gravar nada. Os custos são recalculados a partir de `prices.json` em vez de confiados ao arquivo de origem.

`export` lê o banco direto, então funciona com o daemon parado. Usa a mesma consulta e o mesmo contrato CSV de `GET /api/events.csv`. `--out -` escreve na saída padrão; com `--out ARQUIVO`, a flag `--json` imprime um resumo com a contagem de linhas em vez das linhas.

`prices` edita o `prices.json` (`--prices` aponta para outro caminho). `list` marca cada linha como `override` ou `builtin`. `delete` remove apenas overrides — um preço builtin é recusado, porque o modelo cairia para $0 e deixaria de ser medido.

## API HTTP

Proxy (`:8080`):

```
GET   /health             # liveness do próprio odotoken: sem chave, nunca proxiado
GET   /metrics            # exposição Prometheus
GET   /api/events.csv
GET   /api/sessions
GET   /api/sessions/{id}
GET   /api/cache-savings
POST  /api/simulate
GET|PUT /api/client-keys
DELETE /api/client-keys/{chave-mascarada}
GET|PUT|DELETE /api/prices
GET   /api/anomalies
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
GET  /v1/health       # health da API; exige a chave quando server.api_key está setado
```

## Troubleshooting

Um CLI falhando com `ECONNREFUSED 127.0.0.1:8080` significa que o proxy caiu, não o provedor:

```bash
odotoken status
curl http://127.0.0.1:8080/health
```

Uma resposta com status HTTP (503 e afins) é problema do upstream. Erro de conexão sem status é o proxy local.

O `/health` é o que um monitor ou um check do `systemd` deve usar: não pede
chave e nunca chega ao proxy, então continua respondendo com budget estourado
ou upstream fora. `/v1/health` é a superfície autenticada da API e existe só
para provar que a chave funciona.

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
