# PLAN_v2: Plano de Implementação — Gateway de IA Pessoal

> Implementação do PLAN_v2.md mapeada ao código real. Cada fase é
> autocontida, testável e envia separadamente.

## Análise do código atual (o que já existe vs. o que falta)

| Feature do PLAN_v2 | Estado atual | Gap real |
|--------------------|--------------|----------|
| Project tags | `client_app` já extraído de headers; `metadata` (JSON) existe na tabela | Colunas `project_id` + leitura do header `X-Odo-Project` + índice + queries agregadas |
| Incógnito/PII | Proxy **já** grava só metadados (nunca corpo de prompt/resposta) — o "zero-knowledge" é o comportamento default | Regex PII só tem valor em **logs**; falta `store_prompts` explícito para quem quiser gravar (e aí sim sanitizar) + `is_incognito` flag no evento |
| Shadow pricing | `PricingCalculator` calcula `cost_usd` por modelo | Campo `shadow_cost` + mapa `local→cloud equivalent` no catalog + coluna |
| Prometheus | `/metrics` existe mas em JSON (token-counter app) | Endpoint `/metrics` em **text format Prometheus** no proxy + contadores incrementais |
| Chaos/Mock | Não existe | `MockAdapter` (sem rede) + middleware probabilístico de falha |

---

## Fase 1 — Project Tags + Shadow Pricing (fim de semana 1)

### 1.1 Migration SQLite (db.py)
- [ ] `ALTER TABLE token_events ADD COLUMN project_id TEXT NOT NULL DEFAULT 'default'`
- [ ] `ADD COLUMN shadow_cost REAL NOT NULL DEFAULT 0.0`
- [ ] `CREATE INDEX idx_token_events_project ON token_events(project_id)`
- [ ] Migration idempotente: checar `PRAGMA table_info(token_events)` antes de cada ALTER
- [ ] `TokenEvent` dataclass: campos `project_id: str = "default"`, `shadow_cost: float = 0.0`
- [ ] `record_event`: incluir as 2 colunas no INSERT
- [ ] Nova query `get_summary_by_project(since=None) -> list[dict]`

### 1.2 Proxy (proxy.py)
- [ ] Extrair `x-odo-project` (fallback `x-project`) do header → `project_id` no evento
- [ ] Shadow cost: `pricing.calculate_shadow_cost(model, ...)` — usa `cloud_equivalent` do catalog; se o modelo JÁ é cloud (preço > 0), `shadow_cost = cost_usd` (economia zero)
- [ ] Gravar ambos os custos por evento

### 1.3 Catalog (catalog/models.json)
- [ ] Campo `cloud_equivalent_model_id` nos modelos locais:
  `llama3-70b → gpt-4o`, `llama3-8b → gpt-4o-mini`, etc.
- [ ] `ModelCatalog.get_cloud_equivalent(model) -> str`

### 1.4 Pricing (pricing.py)
- [ ] `calculate_shadow_cost(model, prompt, completion, ...) -> float`
  - se rates do modelo têm input>0: retorna o próprio custo (cloud)
  - senão: rates do `cloud_equivalent` (fallback `default` rates 0 → 0)

### 1.5 API + Frontend
- [ ] `/api/stats?project=<id>` — filtra summary/by_model/by_client/events
- [ ] `/api/stats` retorna `by_project: [...]` + `savings: {actual, shadow, saved}`
- [ ] Frontend: dropdown de projeto no Topbar (fica ao lado do time filter)
- [ ] Frontend: 5º KPI "Saved" (`shadow - actual`) quando > 0
- [ ] Frontend: by_project na view Clients (renomear para "Projects & Apps")

### 1.6 CLI
- [ ] `odotoken stats --by-project`
- [ ] `odotoken stats` mostra linha "Savings vs cloud: $X"

### 1.7 Testes
- [ ] Migration idempotente (rodar `_init_db` 2x)
- [ ] `get_summary_by_project` com 2 projetos distintos
- [ ] Header `X-Odo-Project` propagado ao evento (via TestClient no proxy)
- [ ] Shadow cost: modelo local → preço do equivalente cloud; modelo cloud → custo normal

---

## Fase 2 — Privacy Guard (fim de semana 2)

### 2.1 Clarificação de escopo (importante)
O proxy **nunca** grava corpos — o modo incógnito do plano é o default. O que implementamos:

### 2.2 Redação de PII nos logs (infra/logging.py)
- [ ] `PIIRedactor`: regexes para AWS keys (`AKIA[0-9A-Z]{16}`), OpenAI keys (`sk-[A-Za-z0-9]{20,}`),
  emails, CPF/CPNJ, `/home/<user>` paths, JWTs (`eyJ...`)
- [ ] Config: `privacy.redact_logs: true` (default true), `privacy.custom_patterns: []`
- [ ] Aplicado no `JsonFormatter.format()` e no formatter texto — antes de serializar
- [ ] Benchmark: garantir < 2ms por mensagem; se estourar, pré-compilar regexes (re.compile no load)

### 2.3 store_prompts explícito (opt-in, default FALSE)
- [ ] Config: `privacy.store_prompts: false`
- [ ] Quando true: `TokenEvent.metadata` recebe `{"prompt_sha256": ..., "prompt_preview": "<redado 200 chars>}`
    — preview SEMPRE passa pelo redator, nunca o texto cru
- [ ] `is_incognito` implícito: `store_prompts: false` → evento marca `metadata.incognito=true`

### 2.4 Testes
- [ ] Fixtures com PII (AWS key, email, CPF) → assert que nada de cru chega ao log/preview
- [ ] `store_prompts: false` → nenhum conteúdo no metadata
- [ ] Latência do redator < 2ms (timeit com 100 msgs)

---

## Fase 3 — Prometheus + Chaos/Mock (fim de semana 3)

### 3.1 Prometheus text format (proxy app)
- [ ] Rota `GET /metrics` no proxy (hoje só existe no token-counter JSON) em
  `text/plain; version=0.0.4`:
  ```
  # HELP odotoken_requests_total Total proxied requests
  # TYPE odotoken_requests_total counter
  odotoken_requests_total{model="glm-5.3-flash",provider="verboo",status="200",project="default"} 42
  odotoken_tokens_total{model="glm-5.3-flash",direction="prompt"} 123456
  odotoken_cost_dollars_total{model="glm-5.3-flash"} 0.98
  # HELP odotoken_request_duration_seconds
  # TYPE odotoken_request_duration_seconds histogram
  odotoken_request_duration_seconds_bucket{le="1"} 30
  ...
  odotoken_budget_remaining_percentage 87.5
  ```
- [ ] Fonte: queries SQLite agregadas por janela (ex: all-time) — estado derivado,
  não contadores em memória (sobrevive restart e mantém 1 processo simples)
- [ ] Conflito com rota `/metrics` do token-counter: no proxy app, a versão
  Prometheus vence; JSON metrics ficam em `/metrics/json`

### 3.2 Grafana & compose (examples/)
- [ ] `examples/grafana-dashboard.json` (import-ready, 4 painéis: requests, tokens, cost, latency)
- [ ] `examples/docker-compose.observability.yml` — prometheus + grafana + odotoken
- [ ] `examples/prometheus.yml` — scrape config apontando para o host

### 3.3 MockAdapter (adapters/mock.py)
- [ ] Retorna respostas sintéticas determinísticas (template por modelo)
- [ ] Provider de config: `"mock": {"upstream_url": "mock://local"}` —
  `determine_upstream` detecta scheme `mock://` e despacha pro adapter
- [ ] Gera usage sintético (tokens plausíveis) → metering funciona end-to-end sem rede

### 3.4 Chaos middleware (api/chaos_middleware.py)
- [ ] Config: `chaos: {enabled: false, rate: 0.05, latency_ms: 3000, errors: [429, 500]}`
- [ ] Probabilístico por request: injeta 429/500 ou latência artificial
- [ ] Nunca ativo sem opt-in explícito (`chaos.enabled: true`)
- [ ] Header bypass: `X-Odo-No-Chaos: true` desvia (para testes de sanidade)

### 3.5 Testes
- [ ] Prometheus: formato regex-validado, HELP/TYPE presentes, séries com labels
- [ ] MockAdapter: request → resposta sintética com usage, evento gravado
- [ ] Chaos: rate=1.0 → sempre injeta erro; rate=0 → nunca; determinístico com seed

---

## Ordem de execução e commits

| Commit | Conteúdo | Depende de |
|--------|----------|------------|
| 1 | Migration DB + project_id/shadow_cost (Fase 1.1) | — |
| 2 | Header X-Odo-Project + shadow pricing no proxy (1.2–1.4) | 1 |
| 3 | API by_project + savings + CLI --by-project (1.5–1.6) | 2 |
| 4 | Frontend: filtro projeto + KPI savings (1.5) | 3 |
| 5 | PII redactor + store_prompts (Fase 2) | — (paralelo) |
| 6 | Prometheus /metrics + grafana examples (3.1–3.2) | 1 |
| 7 | MockAdapter + chaos middleware (3.3–3.4) | — (paralelo) |
| 8 | Docs trilíngues + changelog 0.2.0 | todos |

## Critérios de aceite gerais
- Suíte de testes < 10s, CI verde
- Nenhuma query nova sem índice apropriado
- Nenhuma feature ativa sem opt-in quando puder causar custo/falha (chaos, store_prompts)
- READMEs EN/PT/ES atualizados por fase (não só no fim)
- Zero chaves/secrets em código, config versionada ou docs