# odotoken

Proxy transparente de medición de tokens de IA, rastreador de uso y guarda de presupuesto.

**[Português (BR)](README.ptbr.md) · [English](README.md) · [Español](README.es.md)**

## Descripción general

odotoken es un reverse proxy ligero que se ubica entre tus agentes CLI de IA (Claude Code, crush, opencode, omp…) y los proveedores LLM, midiendo de forma transparente el uso de tokens y estimando costos — con control de presupuesto, un dashboard React y una API independiente de conteo de tokens.

## Cómo funciona

```
tus agentes CLI ──→ odotoken :8090 ──→ tus proxies/routers existentes ──→ proveedores LLM
                        │
                        ├── medición de tokens (JSON + streaming SSE)
                        ├── cálculo de costo real (2.700+ precios de modelos)
                        ├── guarda de presupuesto (alerta / bloqueo)
                        └── historial de uso en SQLite
                                │
                     Dashboard React (/dashboard)
```

El proxy enruta por **prefijo de path** — cada clave de proveedor en `config.json` se convierte en una ruta:

| Path de la petición | Upstream |
|---------------------|----------|
| `/anthropic/v1/messages` | tu endpoint compatible con Anthropic |
| `/openai/v1/chat/completions` | tu endpoint compatible con OpenAI |
| `/verboo/...`, `/kodeo/...` | cualquier proveedor personalizado que configures |
| `/v1/messages`, `/v1/chat/completions` | detección automática por formato de petición |

El uso de tokens se extrae tanto de respuestas JSON como de streams SSE (incluyendo proveedores personalizados compatibles con OpenAI). Cada petición se registra en SQLite con tokens de prompt/completion/cache, costo calculado, latencia y código de estado.

## Características

- **Medición transparente** — sin cambios en el flujo de tus CLIs, solo apúntalos al proxy
- **Precios reales** — 2.700+ modelos con tarifas por 1M de tokens de una API pública de precios y configs de routers locales
- **Guarda de presupuesto** — límites diarios/mensuales de costo y tokens con alerta o bloqueo (HTTP 429)
- **Dashboard React** — KPIs en vivo, gráficos, tablas ordenables, stream de eventos con diálogo de detalle por petición, temas claro/oscuro/sistema
- **CLI** — stats en terminal (`odotoken stats --by-model`), log reciente, gestión del servicio
- **API de conteo de tokens** — servicio independiente opcional para conteo pre-vuelo (heurística + APIs remotas de proveedores)
- **Catálogo de modelos** — alias, ventanas de contexto, estrategias de conteo por modelo
- **Caché con TTL** — versionada, invalidada cuando cambian catálogo/tokenizer

## Instalación

```bash
git clone https://github.com/ens-Emilio/odotoken.git
cd odotoken

# backend
python -m venv .venv
.venv/bin/pip install -e .

# frontend (opcional, el dashboard viene pre-compilado en los releases)
cd frontend && pnpm install && pnpm build && cd ..
```

## Inicio rápido

### 1. Configuración

```bash
cp config.json.example config.json  # edita upstreams / limits / sessions
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
    // rutas por prefijo: /<clave>/... → upstream_url
    "anthropic": { "upstream_url": "https://api.anthropic.com" },
    "openai": { "upstream_url": "https://api.openai.com" },
    "mirouter": { "upstream_url": "https://mi-router.ejemplo/v1" }
  },
  "limits": {
    "monthly_budget_usd": 50.0,
    "block_on_budget_exceeded": false
  },
  "database": { "path": "odotoken.db" }
}
```

Las claves de API **nunca** se almacenan en el config — tus CLIs envían sus propios headers `Authorization`/`x-api-key` y el proxy los reenvía sin tocarlos.

### 2. Apunta tus agentes CLI al proxy

| CLI | Cómo |
|-----|------|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | proveedor `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | proveedor `baseURL: "http://127.0.0.1:8090/<clave-de-proveedor>"` |
| Otros | configura su base URL al proxy; usa prefijo `/<clave-de-proveedor>/` para setups multi-proveedor |

Reinicia el CLI después de cambiar su config (las configs se cargan al arrancar).

### 3. Ejecución

```bash
odotoken start              # proxy + dashboard
# o como servicio systemd de usuario (genera el unit con rutas absolutas correctas):
odotoken install-service
systemctl --user daemon-reload
systemctl --user enable --now odotoken
```

Abre **http://127.0.0.1:8090/dashboard** — el uso aparece en hasta 10 segundos.

## Referencia de la CLI

```bash
odotoken start [--host] [--port] [--reload]   # inicia proxy + dashboard
odotoken serve [--port 8081]                  # API independiente de conteo
odotoken status                               # verifica servicio/puerto
odotoken stats [--by-model] [--by-client] [--since ISO-DATE] [--days N]
odotoken log [--limit 20]                     # log de peticiones recientes
odotoken count text "hola" [--model X] [--output json]
odotoken count chat --file mensajes.json      # array JSON de mensajes
odotoken count batch requests.jsonl
odotoken models list [--provider X]
odotoken models info <id-de-modelo>
odotoken sessions scan [--since ISO-DATE]
odotoken sessions import [--dry-run] [--cli omp|claude|codex|crush] [--since ISO-DATE]
odotoken sessions stats [--since ISO-DATE] [--by-cli] [--by-model] [--by-project]
```

## API HTTP

Endpoints del proxy (mismo puerto que el dashboard):

```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/api/stats?since=2026-10-01T00:00:00
```

API de conteo de tokens (`odotoken serve`, puerto 8081):

```bash
curl -X POST http://127.0.0.1:8081/v1/count \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Hola mundo"}'

curl -X POST http://127.0.0.1:8081/v1/count/batch -d '{"items": [...]}'
curl http://127.0.0.1:8081/v1/models
curl http://127.0.0.1:8081/health      # + /ready, /metrics
```

## Precios de modelos

El cálculo de costos usa `prices.json` — **2.700+ modelos** con tarifas reales por 1M de tokens en USD, sincronizadas de dos fuentes (la última gana):

| Fuente | Cobertura | Prioridad |
|--------|-----------|-----------|
| API de precios (por defecto `http://154.29.76.173`, sobrescribe con `PEGASUS_PRICING_API`) | 3.500+ modelos, REST pública | 1 |
| Configs de routers (`~/.local/share/crush/providers.json`, si existe) | precios exactos que cobran tus routers locales | 2 (gana) |

Actualiza los precios cuando quieras:

```bash
node scripts/sync-prices.mjs
systemctl --user restart odotoken
```

Overrides manuales: agrega `"override": true` en cualquier entrada y el script de sync nunca la tocará:

```json
"mi-modelo-custom": { "input": 1.5, "output": 6.0, "cache_read": 0.2, "cache_write": 0, "override": true }
```

La API pública de precios no necesita clave para lecturas (rate limit: 10 req/s por IP en nginx, 60 req/min en la app). Despliega tu propia instancia y apunta `PEGASUS_PRICING_API` a ella si lo prefieres.

## Importando sesiones locales

Usa odotoken para leer sesiones ya guardadas en la máquina (sin proxy) y tarificar con `prices.json`:

```bash
odotoken sessions scan
odotoken sessions import --dry-run
odotoken sessions import
odotoken sessions import --since 2026-10-01
```

| CLI | Origen | Qué se lee |
|-----|--------|-----------|
| OMP | `~/.omp/stats.db` | SQLite `messages` |
| Claude Code | `~/.claude/projects/**/*.jsonl` | JSONL `type=assistant` |
| Codex | `~/.codex/sessions/**` | `token_usage_record` |
| Crush | `~/.local/share/crush/projects.json` | JSONL `usage` |

- Precio único vía `PricingCalculator`.
- Dedup `UNIQUE(source, session_file, entry_id)`.
- Dashboard: filtro **Source**, `GET /api/stats?source=...`, `POST /api/sessions/import`.

## Referencia de configuración

| Setting | Descripción | Por defecto |
|---------|-------------|-------------|
| `proxy.host` / `proxy.port` | dirección de bind | `127.0.0.1:8090` |
| `providers.<clave>.upstream_url` | upstream de la ruta por prefijo | — |
| `proxy.upstream_openai` / `upstream_anthropic` | upstreams de fallback para peticiones detectadas automáticamente | APIs oficiales |
| `limits.daily_cost_limit_usd` / `limits.monthly_cost_limit_usd` | límites diarios/mensuales de costo (0 = desactivado) | `null` |
| `limits.daily_tokens_limit` / `limits.monthly_tokens_limit` | límites diarios/mensuales de tokens (0 = desactivado) | `null` |
| `limits.action_on_breach` | `alert` o `block` (429) | `alert` |
| `limits.webhook_url` | webhook notificado en `warning`/`breached` | `null` |
| `database.path` | archivo SQLite (modo WAL) | `odotoken.db` |
| `token_counter.enabled` | middleware de conteo pre-vuelo en el proxy | `true` |
| `sessions.enabled` / `sessions.paths` | importación de sesiones locales (claude/codex/omp/crush) | `true` / `~/.claude/...` |
## Etiquetas de proyecto, shadow pricing y privacidad

- **Etiquetas de proyecto** — envía `X-Odo-Project: <nombre>` (o `X-Project`) en cualquier petición; costos segmentados por proyecto en el dashboard, `/api/stats?project=<id>` y `odotoken stats --by-project`.
- **Shadow pricing** — modelos locales (precio cero) reciben un `shadow_cost` calculado en el modelo `cloud_equivalent` configurado; el dashboard muestra el KPI "Saved vs Cloud" cuando hay ahorro.
- **Privacidad** — los logs pasan por un redactor de PII (claves, tokens, emails, CPF/CNPJ, rutas home; extiende con `privacy.custom_patterns`). `privacy.store_prompts` (por defecto **false**) permite almacenar solo hash SHA-256 + preview de 200 chars redactado por evento.
- **Prometheus** — `GET /metrics` en el puerto del proxy sirve métricas en text format; importa `examples/grafana-dashboard.json` o ejecuta `docker compose -f examples/docker-compose.observability.yml up -d`.
- **Mock y caos** — define el upstream de un proveedor como `mock://local` para respuestas sintéticas (sin red, con medición). Activa `chaos: {enabled: true, rate: 0.05, errors: [429,500], latency_ms: 3000}` para probar resiliencia del cliente; `X-Odo-No-Chaos: true` lo desvía.

## Solución de problemas

**Todos los CLIs fallan a la vez con `ECONNREFUSED 127.0.0.1:8090`** — el proxy se cayó, no tus proveedores. Triaje rápido:

```bash
odotoken status          # verifica el unit de systemd Y el puerto
curl http://127.0.0.1:8090/health
journalctl --user -u odotoken -n 50 --no-pager
```

| Síntoma | Significado | Corrección |
|---------|-------------|------------|
| `status=203/EXEC` en journal | la ruta del `ExecStart` del unit no existe (proyecto movido/clonado en otro lugar) | ejecuta `odotoken install-service` en la raíz del proyecto |
| `activating (auto-restart)` para siempre | servicio en loop de crash — mira el motivo en el journal | corrige la causa, luego `systemctl --user restart odotoken` |
| nada en :8090, servicio inactive | nunca se inició | `systemctl --user enable --now odotoken` |
| `/health` responde pero los CLIs fallan | el proxy está bien — las configs de los CLIs están desactualizadas | reinicia el CLI (las configs cargan al arrancar) |

**Regla de triaje**: si el error tiene status HTTP (ej. 503 del proveedor), el proxy entregó la petición y el problema está más arriba. Si es un error de conexión sin ningún status (`ECONNREFUSED`, "Unable to connect"), el problema es el propio proxy.

## Notas de seguridad

- **Un solo proceso** — SQLite y la caché en memoria asumen un worker. Ejecuta `odotoken start` sin `--workers` (por defecto 1); multi-worker dividiría la medición entre procesos sin estado compartido.

- El proxy hace bind en `127.0.0.1` por defecto — solo local
- Las claves de API pasan en los headers; **nada se registra ni almacena** — solo conteos de tokens, nombres de modelos, nombre del app cliente (del User-Agent), costo y latencia
- Colócalo detrás de autenticación/reverse-proxy antes de exponerlo en red

## Desarrollo

.venv/bin/pip install -e ".[dev]" 2>/dev/null || .venv/bin/pip install -e . pytest pytest-asyncio
.venv/bin/pytest                          # 125 tests (unit, integración, CLI, golden)
cd frontend && pnpm install && pnpm build  # compila el dashboard → src/odotoken/static/
```

## Licencia

GPL-3.0-or-later. Ver [LICENSE](LICENSE).

## Contribuir

Los PR son bienvenidos — ejecuta la suite de tests y regenera `prices.json` vía el script de sync en lugar de editarlo a mano (excepto entradas con `"override": true`).