# 🍇 odotoken

**Proxy transparente de metering de tokens de IA, rastreador de uso e guarda de orçamento.**

> **v0.6.0** — Paletas de cores inspiradas nas frutas nativas do **Acre, Brasil** 🌿

**[Português (BR)](README.md) · [English](README.en.md) · [Español](README.es.md)**

---

## 🎨 Paletas de Cores: Frutas do Acre

O dashboard do odotoken agora oferece **21 paletas de cores** extraídas diretamente dos tons reais da casca, polpa e folhas das frutas nativas e típicas da Amazônia acreana:

| Fruta | Cor Dominante | Inspiração |
|-------|---------------|------------|
|  **Açaí** | `#7A3E7D` | Roxo profundo da polpa pura |
| 🟤 **Cupuaçu** | `#6B4728` | Marrom terra da casca aveludada |
| 🟠 **Pupunha** | `#E65100` | Laranja fogo da casca madura |
| 🟡 **Tucumã** | `#F57C00` | Laranja açafrão da polpa oleosa |
| 🟫 **Bacuri** | `#C68B23` | Amarelo dourado da casca fibrosa |
| 🟢 **Biribá** | `#7CB342` | Verde limão da casca jovem |
| 🔴 **Buriti** | `#D84315` | Terracota escuro das escamas |
| 🌸 **Bacaba** | `#9C5B6F` | Vinho escuro da casca madura |
| 🫒 **Patauá** | `#8B9574` | Verde acinzentado da polpa oleosa |
| 🌿 **Ingá** | `#475B28` | Verde musgo escuro das ranhuras |
| 💛 **Taperebá** | `#FBC02D` | Amarelo solar intenso da polpa |
| 🟨 **Abiu** | `#FFD600` | Amarelo brilhante da casca lisa |
| 🟤 **Uxi** | `#795548` | Marrom castanho da casca seca |
| 🍋 **Araçá-boi** | `#C0CA33` | Verde amarelado da casca aveludada |
| 🍌 **Banana** | `#FFC107` | Amarelo gema profundo da polpa |
| 🍍 **Abacaxi** | `#1B4D3E` | Verde floresta áspero da coroa |
| 🥝 **Graviola** | `#2E4A28` | Verde folha escuro com espinhos |
|  **Maracujá** | `#EA580C` | Laranja queimado da casca |
| 🥭 **Manga** | `#DC2626` | Vermelho carmim vivo da bochecha exposta |
| 🍑 **Mamão** | `#FB923C` | Laranja papaia rico da polpa doce |
| 🍉 **Melancia** | `#E11D48` | Vermelho rubi vivo do coração da polpa |

Cada paleta aplica cores extraídas diretamente dos tons reais da casca, polpa e folhas de cada fruta, mantendo contraste AA e legibilidade em modo claro e escuro.

---

## 📋 Visão Geral

odotoken é um reverse proxy leve que fica entre seus agentes CLI de IA (Claude Code, crush, opencode, omp…) e os provedores de LLM, medindo de forma transparente o uso de tokens e estimando custos — com controle de orçamento, dashboard React e uma API standalone de contagem de tokens.

## ⚙️ Como Funciona

```
seus agentes CLI ──→ odotoken :8090 ──→ seus proxies/routers existentes ──→ provedores LLM
                        │
                        ├── metering de tokens (JSON + streaming SSE)
                        ├── cálculo de custo real (2.700+ preços de modelos)
                        ├── guarda de orçamento (alerta / bloqueio)
                        └── histórico de uso em SQLite
                                │
                     Dashboard React (/app/)
```

O proxy roteia por **prefixo de path** — cada chave de provider no `config.json` vira uma rota:

| Path da requisição | Upstream |
|--------------------|----------|
| `/anthropic/v1/messages` | seu endpoint compatível com Anthropic |
| `/openai/v1/chat/completions` | seu endpoint compatível com OpenAI |
| `/verboo/...`, `/kodeo/...` | qualquer provider customizado que você configurar |
| `/v1/messages`, `/v1/chat/completions` | detecção automática pelo formato da requisição |

O uso de tokens é extraído tanto de respostas JSON quanto de streams SSE (incluindo providers OpenAI-compatíveis customizados). Cada requisição é gravada no SQLite com tokens de prompt/completion/cache, custo calculado, latência e status code.

## ✨ Funcionalidades

- **Metering transparente** — sem mudanças no fluxo dos seus CLIs, só aponte para o proxy
- **Preços reais** — 2.700+ modelos com taxas por 1M de tokens de uma API pública de preços e configs de routers locais
- **Guarda de orçamento** — limites diários/mensais de custo e tokens com alerta ou bloqueio (HTTP 429)
- **Dashboard React** — KPIs ao vivo, gráficos, tabelas ordenáveis, stream de eventos com dialog de detalhes por requisição, temas claro/escuro/sistema, **21 paletas de cores inspiradas nas frutas do Acre**
- **CLI** — stats no terminal (`odotoken stats --by-model`), log recente, gerenciamento do serviço
- **API de contagem de tokens** — serviço standalone opcional para contagem pré-flight (heurística + APIs remotas dos provedores)
- **Catálogo de modelos** — aliases, janelas de contexto, estratégias de contagem por modelo
- **Cache com TTL** — versionado, invalidado quando catálogo/tokenizer mudam
- **Métricas de throughput** — TPS/TTFT por modelo, projeto e intervalo (`GET /api/tps`, `odotoken stats --tps`, view Throughput)
- **Replay & optimize** — reprecificar eventos históricos com preços atuais (`odotoken replay`) e recomendações de modelos mais baratos (`odotoken optimize`)
- **Tags & forecast** — segmentação via `X-Odo-Tags` e forecast linear de custo + detecção de anomalias no Overview
- **Alertas & export** — alertas Slack/Discord/Telegram/SMTP em breach de orçamento, limites por projeto, export CSV (`GET /api/export?format=csv`)
- **Runs live** — `GET /api/sessions/live` lista requests de modelo em andamento; `X-Odo-Session` agrupa chamadas concorrentes da mesma conversa

## 🚀 Instalação

Baixe o binário da sua plataforma no [GitHub Releases](https://github.com/ens-Emilio/odotoken/releases/latest) mais recente, ou compile do código-fonte:

```bash
git clone https://github.com/ens-Emilio/odotoken.git
cd odotoken

# backend (Go 1.26+)
go build -o bin/odotoken ./cmd/odotoken

# frontend (opcional, o dashboard vem pré-compilado nos releases)
cd frontend && pnpm install && pnpm build && cd ..
```

## ⚡ Início Rápido

### 1. Configuração

```bash
cp config.json.example config.json  # edite upstreams / limits / sessions
```

`config.json` mínimo:

```jsonc
{
  "proxy": {
    "host": "127.0.0.1",
    "port": 8090,
    "upstream_openai": "https://api.openai.com",
    "upstream_anthropic": "https://api.anthropic.com"
  },
  "providers": {
    // rotas por prefixo: /<chave>/... → upstream_url
    "anthropic": { "upstream_url": "https://api.anthropic.com" },
    "openai": { "upstream_url": "https://api.openai.com" },
    "meurouter": { "upstream_url": "https://meu-router.exemplo/v1" }
  },
  "guard": {
    "daily_cost_limit_usd": 10,
    "monthly_cost_limit_usd": 100,
    "daily_tokens_limit": 50000,
    "monthly_tokens_limit": 500000,
    "warning_threshold_pct": 80,
    "action_on_breach": "block"
  },
  "database": { "path": "odotoken.db" }
}
```

Chaves de API **nunca** são armazenadas no config — seus CLIs enviam seus próprios headers `Authorization`/`x-api-key` e o proxy encaminha sem tocar.

### 2. Aponte seus agentes CLI para o proxy

| CLI | Como |
|-----|------|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | provider `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | provider `baseURL: "http://127.0.0.1:8090/<chave-do-provider>"` |
| Outros | defina a base URL para o proxy; use prefixo `/<chave-do-provider>/` para setups multi-provider |

Reinicie o CLI após mudar a config (configs carregam no boot).

### 3. Executando

```bash
odotoken start              # proxy + dashboard
# ou como serviço systemd de usuário (gera o unit com paths absolutos corretos):
odotoken install-service
systemctl --user daemon-reload
systemctl --user enable --now odotoken
```

Abra **http://127.0.0.1:8090/app/** — o uso aparece em até 10 segundos.

## 🎨 Selecionando uma Paleta de Cores

No canto superior direito do dashboard, ao lado do botão de tema (sol/lua), você verá um **círculo colorido** representando a paleta ativa. Clique para abrir o menu com as 21 opções inspiradas nas frutas do Acre:

1. **Original** — paleta neutra padrão (verde brand)
2. **Açaí** 🟣 — roxo profundo
3. **Cupuaçu** 🟤 — marrom terra
4. **Pupunha** 🟠 — laranja fogo
5. **Tucumã** 🟡 — âmbar dourado
6. **Bacuri** 🟫 — marrom dourado
7. **Biribá** 🟢 — verde limão
8. **Buriti** 🔴 — terracota escuro
9. **Bacaba** 🌸 — vinho escuro
10. **Patauá** 🫒 — verde acinzentado
11. **Ingá** 🌿 — verde musgo
12. **Taperebá** 💛 — amarelo solar
13. **Abiu** 🟨 — amarelo brilhante
14. **Uxi** 🟤 — marrom castanho
15. **Araçá-boi** 🍋 — verde amarelado
16. **Banana** 🍌 — amarelo gema
17. **Abacaxi** 🍍 — verde floresta
18. **Graviola** 🥝 — verde folha
19. **Maracujá** 🧡 — laranja queimado
20. **Manga** 🥭 — vermelho carmim
21. **Mamão** 🍑 — laranja papaia
22. **Melancia** 🍉 — vermelho rubi

A escolha é persistida no `localStorage` e aplicada instantaneamente via variáveis CSS dinâmicas.

## 📖 Referência da CLI

```bash
odotoken start [--host] [--port] [--reload]   # inicia proxy + dashboard
odotoken serve [--port 8081]                  # API standalone de contagem
odotoken status                               # verifica serviço/porta
odotoken stats [--by-model] [--by-client] [--since ISO-DATE] [--days N]
odotoken log [--limit 20]                     # log de requisições recentes
odotoken count text "olá" [--model X] [--output json]
odotoken count chat --file mensagens.json     # array JSON de mensagens
odotoken count batch requests.jsonl
odotoken models list [--provider X]
odotoken models info <id-do-modelo>
odotoken sessions scan [--since DATA-ISO]     # somente leitura: quais CLIs existem aqui
odotoken sessions import [--dry-run] [--cli omp|claude|codex|crush] [--since DATA-ISO]
odotoken sessions stats [--since DATA-ISO] [--by-cli] [--by-model] [--by-project]
```

## 🌐 API HTTP

Endpoints do proxy (mesma porta do dashboard):

```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/api/stats?since=2026-10-01T00:00:00
curl http://127.0.0.1:8090/api/guard/status
curl http://127.0.0.1:8090/api/filters
curl http://127.0.0.1:8090/api/stats.csv
curl http://127.0.0.1:8090/api/stats.svg
```

API de contagem de tokens (`odotoken serve`, porta 8081):

```bash
curl -X POST http://127.0.0.1:8081/v1/count \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Olá mundo"}'

curl -X POST http://127.0.0.1:8081/v1/count/batch -d '{"items": [...]}'
curl http://127.0.0.1:8081/v1/models
curl http://127.0.0.1:8081/health      # + /ready, /metrics
```

## 💰 Preços de Modelos

O cálculo de custos usa o `prices.json` — **2.700+ modelos** com taxas reais por 1M de tokens em USD, sincronizadas de duas fontes (a última vence):

| Fonte | Cobertura | Prioridade |
|-------|-----------|------------|
| API de preços (padrão `http://154.29.76.173`, sobrescreva com `PEGASUS_PRICING_API`) | 3.500+ modelos, REST pública | 1 |
| Configs de routers (`~/.local/share/crush/providers.json`, se existir) | preços exatos que seus routers locais cobram | 2 (vence) |

Atualize os preços quando quiser:

```bash
node scripts/sync-prices.mjs
systemctl --user restart odotoken
```

Overrides manuais: adicione `"override": true` em qualquer entrada e o script de sync nunca vai tocar nela:

```json
"meu-modelo-custom": { "input": 1.5, "output": 6.0, "cache_read": 0.2, "cache_write": 0, "override": true }
```

A API pública de preços não precisa de chave para leitura (rate limit: 10 req/s por IP no nginx, 60 req/min no app). Faça deploy da sua própria instância e aponte `PEGASUS_PRICING_API` para ela se preferir.

## 📥 Importando Sessões Locais

Use o odotoken para ler sessões já salvas na máquina (sem precisar do proxy) e precificar via `prices.json`:

```bash
odotoken sessions scan                        # quais CLIs/arquivos foram encontrados (somente leitura)
odotoken sessions import --dry-run            # conta tokens + custo sem gravar
odotoken sessions import                      # importa no SQLite (idempotente, com dedup)
odotoken sessions import --since 2026-10-01   # incremental
odotoken sessions stats --by-cli --by-model   # breakdown por origem
```

| CLI | Origem | O que é lido |
|-----|--------|--------------|
| OMP | `~/.omp/stats.db` | SQLite `messages` |
| Claude Code | `~/.claude/projects/**/*.jsonl` | JSONL `type=assistant` → `message.usage` |
| Codex | `~/.codex/sessions/**` | `token_usage_record` → `payload.usage` |
| Crush | `~/.local/share/crush/projects.json` → `data_dir/**/*.jsonl` | JSONL `usage` (best-effort) |

- **Preço único** — toda linha importada é recalculada pelo `PricingCalculator` (`prices.json`).
- **Dedup** — `UNIQUE(source, session_file, entry_id)`; reexecutar `import` insere `0`.
- **Dashboard** — filtro **Source** no header, `GET /api/stats?source=...`, `POST /api/sessions/import` e card **By Source**.

## ⚙️ Referência de Configuração

| Setting | Descrição | Padrão |
|---------|-----------|--------|
| `proxy.host` / `proxy.port` | endereço de bind | `127.0.0.1:8090` |
| `providers.<chave>.upstream_url` | upstream da rota por prefixo | — |
| `proxy.upstream_openai` / `upstream_anthropic` | upstreams de fallback para requisições detectadas automaticamente | APIs oficiais |
| `guard.daily_cost_limit_usd` / `guard.monthly_cost_limit_usd` | limites diário/mensal de custo (0 = desativado) | `null` |
| `guard.daily_tokens_limit` / `guard.monthly_tokens_limit` | limites diário/mensal de tokens (0 = desativado) | `null` |
| `guard.warning_threshold_pct` | percentual em que a guarda passa de ok para warning | `80` |
| `guard.action_on_breach` | `alert` continua atendendo e dispara alertas; `block` rejeita requisições estouradas com HTTP 429 | `block` |
| `guard.timezone` | fuso IANA usado nos limites diário e mensal | UTC |
| `limits.webhook_url` | webhook notificado em `warning`/`breached` | `null` |
| `database.path` | arquivo SQLite (modo WAL) | `odotoken.db` |
| `token_counter.enabled` | middleware de contagem pré-flight no proxy | `true` |
| `sessions.enabled` / `sessions.paths` | importação de sessões locais (claude/codex/omp/crush) | `true` / `~/.claude/...` |

## 🏷️ Tags de Projeto, Shadow Pricing e Privacidade

- **Tags de projeto** — envie `X-Odo-Project: <nome>` (ou `X-Project`) em qualquer requisição; custos segmentados por projeto no dashboard, `/api/stats?project=<id>` e `odotoken stats --by-project`.
- **Runs live** — `GET /api/sessions/live` lista requests de modelo em andamento. Envie `X-Odo-Session: <conversation-id>` para agrupar chamadas concorrentes da mesma conversa; cada request mantém seu próprio `run_id`.
- **Shadow pricing** — modelos locais (preço zero) ganham um `shadow_cost` calculado a partir do equivalente cloud escolhido por `getCloudEquivalent` (`internal/pricing/calculator.go`); o dashboard mostra o KPI "Saved vs Cloud" quando há economia.
- **Privacidade** — o odotoken armazena metadados de uso, não o conteúdo de prompts ou respostas. As mensagens importadas nunca são processadas: o proxy registra tokens, tempos, modelo, projeto e tags. Não há redator de PII no momento; não trate o banco como anonimizado.
- **Prometheus** — `GET /metrics` na porta do proxy serve métricas em text format; importe `examples/grafana-dashboard.json` ou rode `docker compose -f examples/docker-compose.observability.yml up -d`.
- **Mock & caos** — defina o upstream de um provider como `mock://local` para respostas sintéticas (sem rede, com metering). Ative `chaos: {enabled: true, rate: 0.05, errors: [429,500], latency_ms: 3000}` para testar resiliência do cliente; `X-Odo-No-Chaos: true` desvia.

## 🔧 Solução de Problemas

**Todos os CLIs falham ao mesmo tempo com `ECONNREFUSED 127.0.0.1:8090`** — o proxy caiu, não seus provedores. Triagem rápida:

```bash
odotoken status          # verifica o unit do systemd E a porta
curl http://127.0.0.1:8090/health
journalctl --user -u odotoken -n 50 --no-pager
```

| Sintoma | Significado | Correção |
|---------|-------------|----------|
| `status=203/EXEC` no journal | o path do `ExecStart` do unit não existe (projeto movido/clonado em outro lugar) | rode `odotoken install-service` na raiz do projeto |
| `activating (auto-restart)` para sempre | serviço em loop de crash — veja o motivo no journal | corrija a causa, depois `systemctl --user restart odotoken` |
| nada na :8090, serviço inactive | nunca foi iniciado | `systemctl --user enable --now odotoken` |
| `/health` responde mas CLIs falham | o proxy está bem — as configs dos CLIs estão desatualizadas | reinicie o CLI (configs carregam no boot) |

**Regra de triagem**: se o erro tem status HTTP (ex. 503 do provedor), o proxy entregou a requisição e o problema está acima. Se é erro de conexão sem status nenhum (`ECONNREFUSED`, "Unable to connect"), o problema é o próprio proxy.

## 🔒 Notas de Segurança

- **Um processo apenas** — o SQLite e o cache em memória assumem um worker. Rode `odotoken start` sem `--workers` (padrão 1); multi-worker dividiria o metering entre processos sem estado compartilhado.
- O proxy faz bind em `127.0.0.1` por padrão — só local
- Chaves de API passam nos headers; **nada é logado ou armazenado** — apenas contagens de tokens, nomes de modelos, nome do app cliente (do User-Agent), custo e latência
- Coloque atrás de autenticação/reverse-proxy antes de expor em rede

## 🛠️ Desenvolvimento

```bash
make build                                 # compila ./cmd/odotoken → bin/odotoken
make test                                  # go test -race ./...
cd frontend && pnpm install && pnpm build   # compila o dashboard → internal/server/static/
```

## 📄 Licença

GPL-3.0-or-later. Veja [LICENSE](LICENSE).

## 🤝 Contribuindo

PRs são bem-vindos — rode a suíte de testes e regenere o `prices.json` via script de sync em vez de editar à mão (exceto entradas com `"override": true`).

---

**Feito com 🌿 no Acre, Brasil** — onde a floresta inspira até as cores do código.