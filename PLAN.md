# Plano de Implementação: Token Counter para odotoken

## Visão Geral
Integrar um contador de tokens pré-flight ao odotoken existente (que atualmente faz metering pós-resposta), permitindo:
- Contagem de tokens antes de enviar requisições aos LLMs
- Integração com o sistema de limites/budget existente
- API HTTP e CLI para contagem standalone
- Estratégias de contagem: remota (APIs oficiais), local (tokenizers), heurística

## 1. Análise do Código Existente

### Estrutura Atual (`src/odotoken/`)
- `proxy.py`: FastAPI reverse proxy com metering transparente
- `parser.py`: Extrai token usage de respostas dos providers (JSON/SSE)
- `pricing.py`: Calcula custos por model (preços por 1M tokens)
- `guard.py`: Sistema de limites (custo/tokens) com alert/block
- `db.py`: SQLite com WAL mode para armazenar eventos
- `dashboard.py`: Web UI + API para estatísticas
- `cli.py`: Typer CLI (start/stop/status/stats/log)
- `tests/`: Suite pytest abrangente

### Fluxo Atual
1. Requisição chega ao proxy
2. Proxy encaminha para provider (OpenAI/Anthropic/etc)
3. Parser extrai tokens da resposta
4. Pricing calcula custo
5. DB grava evento
6. Guard verifica limites (pós-facto)

### Integrações Necessárias para o Token Counter
- **Config**: Nova seção `token_counter` em `config.json`
- **DB**: Reutilizar conexão/evento (opcional: armazenar contagens pré-flight)
- **Pricing**: Reutilizar `PricingCalculator` para estimativa de custo
- **Guard**: Integrar contagem pré-flight para alertas preventivos
- **Logging**: Reutilizar estrutura de logging existente

## 2. Estrutura de Novos Módulos

```
src/odotoken/
├── domain/
│   ├── entities.py      # CountRequest, CountResult, ModelInfo
│   └── errors.py        # Domain exceptions
├── adapters/
│   ├── base.py          # Abstract adapter interface
│   ├── heuristic.py     # Character/word-based estimation
│   ├── remote.py        # Provider API integrations
│   └── local.py         # Pluggable tokenizer interface (tiktoken, etc)
├── services/
│   ├── token_counter.py # Main service orchestrating adapters
│   ├── model_catalog.py # Model information + resolution
│   └── cache.py         # In-memory cache with TTL
├── infra/
│   ├── config.py        # Config loader (token_counter section)
│   └── logging.py       # Logging/metrics setup (reusable)
├── api/
│   ├── routes/
│   │   ├── count.py     # POST /count, POST /count/batch
│   │   ├── models.py    # GET /models
│   │   └── health.py    # GET /health, /ready, /metrics
│   └── app.py           # FastAPI app factory
├── cli/
│   ├── commands/
│   │   ├── count.py     # count text/chat/batch
│   │   └── models.py    # models list
│   └── main.py          # Typer app integration
└── catalog/
    └── models.json      # Model information database
```

## 3. Plano de Execução por Fases

### Fase 0: Preparação (JÁ FEITA)
- [x] Criar estrutura de diretórios
- [x] Atualizar README com visão geral

### Fase 1: Fundação
- [x] Criar domain/entities.py (CountRequest, CountResult, ModelInfo)
- [x] Criar domain/errors.py (domain exceptions)
- [x] Adicionar seção `token_counter` ao config.json
- [x] Criar infra/config.py (loader para token_counter)
- [x] Criar infra/logging.py (setup logging/metrics)

### Fase 2: Núcleo do Contador
- [ ] Criar adapters/base.py (AdapterProtocol)
- [ ] Criar adapters/heuristic.py (implemenetação simples)
- [ ] Criar services/model_catalog.py (carregar models.json)
- [ ] Criar services/token_counter.py (orquestra adapters + cache)
- [ ] Criar services/cache.py (cache em memória com TTL)
- [ ] Criar catalog/models.json (dados iniciais de modelos)

### Fase 3: Adaptadores Remotos
- [ ] Criar adapters/remote.py (httpx + retry logic)
- [ ] Implementar suporte para OpenAI, Anthropic, Gemini
- [ ] Adapar endpoints de contagem específicos de cada provider
- [ ] Tratar autenticação via API key do config

### Fase 4: API HTTP
- [ ] Criar api/routes/count.py (endpoints /count, /count/batch)
- [ ] Criar api/routes/models.py (endpoint /models)
- [ ] Criar api/routes/health.py (endpoints /health, /ready, /metrics)
- [ ] Criar api/app.py (FastAPI factory)
- [ ] Atualizar cli.py para incluir comando `serve`

### Fase 5: CLI
- [ ] Criar cli/commands/count.py (comandos count text/chat/batch)
- [ ] Criar cli/commands/models.py (comando models list)
- [ ] Integrar ao app Typer existente em cli.py

### Fase 6: Integração com Proxy
- [ ] Criar middleware de contagem pré-flight (opcional)
- [ ] Adicionar headers de contagem pré-flight às respostas do proxy
- [ ] Integrar com Guard para alertas preventivos

### Fase 7: Testes e Qualidade
- [ ] Criar testes unitários para todos os adaptadores
- [ ] Criar testes de integração para API
- [ ] Criar testes para CLI
- [ ] Criar testes de contrato (fixtures gold)
- [ ] Atualizar pyproject.toml com extras opcionais

### Fase 8: Documentação e Empacotamento
- [ ] Atualizar README com exemplos de uso
- [ ] Adicionar docstrings detalhadas
- [ ] Empacotar com uv (já configurado)

## 4. Critérios de Aceite por Fase

### Fase 1: Fundação
- Config carrega token_counter section sem quebrar proxy existente
- Domínio definido (entidades, errors) passa em lint/type check
- Logging configurado funciona com structlog ou padrão

### Fase 2: Núcleo
- HeuristicAdapter retorna contagens razoáveis (±20% de estimativa básica)
- ModelCatalog resolve aliases e retorna ModelInfo
- TokenCounter orquestra adapters e retorna CountResult
- Cache armazena e recupera com TTL

### Fase 3: Adaptadores Remotos
- RemoteAdapter chama APIs de contagem quando configurado
- Retry e timeout funcionam
- Erros são convertidos para domain exceptions

### Fase 4: API HTTP
- Endpoint /count aceita texto e mensagens
- Endpoint /count/batch processa lotes
- Endpoint /models lista informações dos modelos
- Health checks retornam 200 quando saudável

### Fase 5: CLI
- Comando `odotoken count text "hello"` retorna contagem
- Comando `odotoken count chat --file conv.json` funciona
- Comando `odotoken models list` mostra tabela Rich
- Erros são exibidos de forma amigável

### Fase 6: Integração com Proxy
- Middleware opcional não quebra fluxo existente
- Headers adicionados quando habilitado
- Guard pode usar contagem pré-flight para alertas

### Fase 7: Testes
- >80% de cobertura de código nos novos módulos
- Testes unitários passam em isolamento
- Testes de integração não requerem redes reais
- Testes de CLI cobrem comandos principais

### Fase 8: Documentação
- README atualizado com exemplos
- Docstrings em funções públicas
- pyproject.toml inclui extras para tokenizers locais