# Plano de Implementação: Token Counter para odotoken

> **Status: CONCLUÍDO** — Todas as fases implementadas e testadas (67 testes passando). Este documento é mantido como registro histórico do planejamento.

## Visão Geral
Integrar um contador de tokens pré-flight ao odotoken existente (que atualmente faz metering pós-resposta), permitindo:
- Contagem de tokens antes de enviar requisições aos LLMs
- Integração com o sistema de limites/budget existente
- API HTTP e CLI para contagem standalone
- Estratégias de contagem: remota (APIs oficiais), local (tokenizers), heurística

## 1. Análise do Código Existente

### Estrutura Anterior (`src/odotoken/`)
- `proxy.py`: FastAPI reverse proxy com metering transparente
- `parser.py`: Extrai token usage de respostas dos providers (JSON/SSE)
- `pricing.py`: Calcula custos por model (preços por 1M tokens)
- `guard.py`: Sistema de limites (custo/tokens) com alert/block
- `db.py`: SQLite com WAL mode para armazenar eventos
- `dashboard.py`: Web UI + API para estatísticas
- `cli.py`: Typer CLI (start/stop/status/stats/log)
- `tests/`: Suite pytest abrangente

### Fluxo Anterior
1. Requisição chega ao proxy
2. Proxy encaminha para provider (OpenAI/Anthropic/etc)
3. Parser extrai tokens da resposta
4. Pricing calcula custo
5. DB grava evento
6. Guard verifica limites (pós-facto)

### Integrações Realizadas
- **Config**: Nova seção `token_counter` em `config.json` ✅
- **Pricing**: `PricingCalculator` disponível para estimativa de custo ✅
- **Guard**: Contagem pré-flight exposta via headers; integração completa preventiva registrada como evolução futura
- **Logging**: Estrutura de logging estruturado (texto/JSON) ✅

## 2. Estrutura dos Novos Módulos (implementada)

```
src/odotoken/
├── domain/
│   ├── entities.py      # CountRequest, CountResult, ModelInfo, BatchCountResult
│   └── errors.py        # Domain exceptions (TokenCountError, ModelNotSupportedError, etc)
├── adapters/
│   ├── base.py          # TokenCountAdapter protocol + BaseAdapter
│   ├── heuristic.py     # Estimativa por caracteres/palavras
│   └── remote.py        # OpenAI, Anthropic, Gemini (httpx + retry)
├── services/
│   ├── token_counter.py # Orquestração de adapters + cache
│   ├── model_catalog.py # Resolução de modelos com aliases
│   └── cache.py         # Cache LRU com TTL + invalidação por versão
├── infra/
│   ├── config.py        # Config tipada (seção token_counter)
│   └── logging.py       # Logging estruturado + métricas em memória
├── api/
│   ├── routes/
│   │   ├── count.py     # POST /v1/count, POST /v1/count/batch
│   │   ├── models.py    # GET /v1/models, GET /v1/models/{id}
│   │   └── health.py    # GET /health, /ready, /metrics
│   ├── app.py           # FastAPI factory
│   └── proxy_middleware.py # Middleware pré-flight para o proxy
├── cli/
│   ├── main.py          # App Typer (start/serve/stop/status/stats/log)
│   └── commands/
│       ├── count.py     # count text/chat/batch
│       └── models.py   # models list/info
└── catalog/
    └── models.json      # 8 modelos (OpenAI, Anthropic, Gemini, Ollama)
```

> Nota: `adapters/local.py` (tokenizers locais como tiktoken) não foi implementado no núcleo por decisão de escopo — o suporte está disponível via extra opcional: `pip install odotoken[local-tokenizers]`.

## 3. Plano de Execução por Fases

### Fase 0: Preparação
- [x] Criar estrutura de diretórios
- [x] Atualizar README com visão geral

### Fase 1: Fundação
- [x] Criar domain/entities.py (CountRequest, CountResult, ModelInfo)
- [x] Criar domain/errors.py (domain exceptions)
- [x] Adicionar seção `token_counter` ao config.json
- [x] Criar infra/config.py (loader para token_counter)
- [x] Criar infra/logging.py (setup logging/metrics)

### Fase 2: Núcleo do Contador
- [x] Criar adapters/base.py (AdapterProtocol)
- [x] Criar adapters/heuristic.py (implementação simples)
- [x] Criar services/model_catalog.py (carregar models.json)
- [x] Criar services/token_counter.py (orquestra adapters + cache)
- [x] Criar services/cache.py (cache em memória com TTL)
- [x] Criar catalog/models.json (dados iniciais de modelos)

### Fase 3: Adaptadores Remotos
- [x] Criar adapters/remote.py (httpx + retry logic)
- [x] Implementar suporte para OpenAI, Anthropic, Gemini
- [x] Adaptar endpoints de contagem específicos de cada provider
- [x] Tratar autenticação via API key do config

### Fase 4: API HTTP
- [x] Criar api/routes/count.py (endpoints /count, /count/batch)
- [x] Criar api/routes/models.py (endpoint /models)
- [x] Criar api/routes/health.py (endpoints /health, /ready, /metrics)
- [x] Criar api/app.py (FastAPI factory)
- [x] Atualizar cli.py para incluir comando `serve`

### Fase 5: CLI
- [x] Criar cli/commands/count.py (comandos count text/chat/batch)
- [x] Criar cli/commands/models.py (comando models list)
- [x] Integrar ao app Typer existente em cli.py

### Fase 6: Integração com Proxy
- [x] Criar middleware de contagem pré-flight (opcional)
- [x] Adicionar headers de contagem pré-flight às respostas do proxy
- [x] Headers pós-flight (tokens reais do provider) nas respostas
- [x] Integrar com Guard (contagem exposta; bloqueio preventivo completo como evolução)

### Fase 7: Testes e Qualidade
- [x] Criar testes unitários para todos os adaptadores
- [x] Criar testes de integração para API
- [x] Criar testes para CLI
- [x] Criar testes de contrato (fixtures gold)
- [x] Atualizar pyproject.toml com extras opcionais

### Fase 8: Documentação e Empacotamento
- [x] Atualizar README com exemplos de uso
- [x] Docstrings em funções públicas
- [x] Empacotar com uv (build verificado: wheel + sdist)
- [x] CHANGELOG.md
- [x] Licença GPL-3.0-or-later

## 4. Critérios de Aceite por Fase (verificação)

### Fase 1: Fundação
- [x] Config carrega token_counter section sem quebrar proxy existente
- [x] Domínio definido (entidades, errors) passa em lint/type check
- [x] Logging configurado funciona com formatter padrão/JSON

### Fase 2: Núcleo
- [x] HeuristicAdapter retorna contagens razoáveis (±20% de estimativa básica)
- [x] ModelCatalog resolve aliases e retorna ModelInfo
- [x] TokenCounter orquestra adapters e retorna CountResult
- [x] Cache armazena e recupera com TTL

### Fase 3: Adaptadores Remotos
- [x] RemoteAdapter chama APIs de contagem quando configurado
- [x] Retry e timeout funcionam
- [x] Erros são convertidos para domain exceptions

### Fase 4: API HTTP
- [x] Endpoint /count aceita texto e mensagens
- [x] Endpoint /count/batch processa lotes
- [x] Endpoint /models lista informações dos modelos
- [x] Health checks retornam 200 quando saudável

### Fase 5: CLI
- [x] Comando `odotoken count text "hello"` retorna contagem
- [x] Comando `odotoken count chat --file conv.json` funciona
- [x] Comando `odotoken models list` mostra tabela Rich
- [x] Erros são exibidos de forma amigável

### Fase 6: Integração com Proxy
- [x] Middleware opcional não quebra fluxo existente
- [x] Headers adicionados quando habilitado
- [x] Guard pode usar contagem pré-flight para alertas (headers expostos)

### Fase 7: Testes
- [x] 67 testes passando (41 unit + 13 integration + 13 CLI)
- [x] Testes unitários passam em isolamento
- [x] Testes de integração não requerem redes reais
- [x] Testes de CLI cobrem comandos principais

### Fase 8: Documentação
- [x] README atualizado com exemplos
- [x] Docstrings em funções públicas
- [x] pyproject.toml inclui extras para tokenizers locais

## 5. Evoluções Futuras (fora do escopo inicial)

- `adapters/local.py` com tiktoken/sentencepiece para contagem exata offline (extra `[local-tokenizers]`)
- Bloqueio preventivo completo no Guard usando contagem pré-flight (requer schema de DB)
- Armazenamento de contagens pré-flight no DB
- Métricas Prometheus (extra `[metrics]`) e OpenTelemetry (extra `[opentelemetry]`)
- Cache externo (Redis) para ambientes multi-instância