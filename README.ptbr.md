# odotoken

Proxy transparente de metering de tokens de IA, rastreador de uso e guarda de orçamento.

**[Português (BR)](README.ptbr.md) · [English](README.md) · [Español](README.es.md)**

## Visão geral

odotoken é um reverse proxy leve que fica entre seus agentes CLI de IA (Claude Code, crush, opencode, omp…) e os provedores de LLM, medindo de forma transparente o uso de tokens e estimando custos — com controle de orçamento, dashboard React e uma API standalone de contagem de tokens.

## Como funciona

```
seus agentes CLI ──→ odotoken :8090 ──→ seus proxies/routers existentes ──→ provedores LLM
                        │
                        ├── metering de tokens (JSON + streaming SSE)
                        ├── cálculo de custo real (2.700+ preços de modelos)
                        ├── guarda de orçamento (alerta / bloqueio)
                        └── histórico de uso em SQLite
                                │
                     Dashboard React (/dashboard)
```

O proxy roteia por **prefixo de path** — cada chave de provider no `config.json` vira uma rota:

| Path da requisição | Upstream |
|--------------------|----------|
| `/anthropic/v1/messages` | seu endpoint compatível com Anthropic |
| `/openai/v1/chat/completions` | seu endpoint compatível com OpenAI |
| `/verboo/...`, `/kodeo/...` | qualquer provider customizado que você configurar |
| `/v1/messages`, `/v1/chat/completions` | detecção automática pelo formato da requisição |

O uso de tokens é extraído tanto de respostas JSON quanto de streams SSE (incluindo providers OpenAI-compatíveis customizados). Cada requisição é gravada no SQLite com tokens de prompt/completion/cache, custo calculado, latência e status code.

## Funcionalidades

- **Metering transparente** — sem mudanças no fluxo dos seus CLIs, só aponte para o proxy
- **Preços reais** — 2.700+ modelos com taxas por 1M de tokens de uma API pública de preços e configs de routers locais
- **Guarda de orçamento** — limites diários/mensais de custo e tokens com alerta ou bloqueio (HTTP 429)
- **Dashboard React** — KPIs ao vivo, gráficos, tabelas ordenáveis, stream de eventos com dialog de detalhes por requisição, temas claro/escuro/sistema
- **CLI** — stats no terminal (`odotoken stats --by-model`), log recente, gerenciamento do serviço
- **API de contagem de tokens** — serviço standalone opcional para contagem pré-flight (heurística + APIs remotas dos provedores)
- **Catálogo de modelos** — aliases, janelas de contexto, estratégias de contagem por modelo
- **Cache com TTL** — versionado, invalidado quando catálogo/tokenizer mudam

## Instalação

```bash
git clone https://github.com/ens-Emilio/odotoken.git
cd odotoken

# backend
python -m venv .venv
.venv/bin/pip install -e .

# frontend (opcional, o dashboard vem pré-compilado nos releases)
cd frontend && pnpm install && pnpm build && cd ..
```

## Início rápido

### 1. Configuração

Copie e edite o `config.json`:

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
  "limits": {
    "monthly_budget_usd": 50.0,
    "block_on_budget_exceeded": false
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

Abra **http://127.0.0.1:8090/dashboard** — o uso aparece em até 10 segundos.

## Referência da CLI

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
```

## API HTTP

Endpoints do proxy (mesma porta do dashboard):

```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/api/stats?since=2026-10-01T00:00:00
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

## Preços de modelos

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

## Referência de configuração

| Setting | Descrição | Padrão |
|---------|-----------|--------|
| `proxy.host` / `proxy.port` | endereço de bind | `127.0.0.1:8080` |
| `providers.<chave>.upstream_url` | upstream da rota por prefixo | — |
| `proxy.upstream_openai` / `upstream_anthropic` | upstreams de fallback para requisições detectadas automaticamente | APIs oficiais |
| `limits.monthly_budget_usd` | limite mensal de custo (0 = desativado) | `0.0` |
| `limits.daily_budget_usd`, `daily/monthly_tokens_limit` | mais limites | `0` (desativado) |
| `limits.action_on_breach` | `alert` ou `block` (429) | `alert` |
| `database.path` | arquivo SQLite (modo WAL) | `odotoken.db` |
| `token_counter.enabled` | middleware de contagem pré-flight no proxy | `false` |

## Solução de problemas

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

## Notas de segurança

- O proxy faz bind em `127.0.0.1` por padrão — só local
- Chaves de API passam nos headers; **nada é logado ou armazenado** — apenas contagens de tokens, nomes de modelos, nome do app cliente (do User-Agent), custo e latência
- Coloque atrás de autenticação/reverse-proxy antes de expor em rede

## Desenvolvimento

```bash
.venv/bin/pip install -e . pytest pytest-asyncio
.venv/bin/pytest                          # 75 testes (unit, integração, CLI, golden)
cd frontend && pnpm install && pnpm build  # compila o dashboard → src/odotoken/static/
```

## Licença

GPL-3.0-or-later. Veja [LICENSE](LICENSE).

## Contribuindo

PRs são bem-vindos — rode a suíte de testes e regenere o `prices.json` via script de sync em vez de editar à mão (exceto entradas com `"override": true`).