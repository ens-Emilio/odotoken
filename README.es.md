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

Copia y edita `config.json`:

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
# o como servicio systemd de usuario:
mkdir -p ~/.config/systemd/user
cp odotoken.service ~/.config/systemd/user/   # ¡edita el WorkingDirectory primero!
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

## Referencia de configuración

| Setting | Descripción | Por defecto |
|---------|-------------|-------------|
| `proxy.host` / `proxy.port` | dirección de bind | `127.0.0.1:8080` |
| `providers.<clave>.upstream_url` | upstream de la ruta por prefijo | — |
| `proxy.upstream_openai` / `upstream_anthropic` | upstreams de fallback para peticiones detectadas automáticamente | APIs oficiales |
| `limits.monthly_budget_usd` | límite mensual de costo (0 = desactivado) | `0.0` |
| `limits.daily_budget_usd`, `daily/monthly_tokens_limit` | más límites | `0` (desactivado) |
| `limits.action_on_breach` | `alert` o `block` (429) | `alert` |
| `database.path` | archivo SQLite (modo WAL) | `odotoken.db` |
| `token_counter.enabled` | middleware de conteo pre-vuelo en el proxy | `false` |

## Notas de seguridad

- El proxy hace bind en `127.0.0.1` por defecto — solo local
- Las claves de API pasan en los headers; **nada se registra ni almacena** — solo conteos de tokens, nombres de modelos, nombre del app cliente (del User-Agent), costo y latencia
- Colócalo detrás de autenticación/reverse-proxy antes de exponerlo en red

## Desarrollo

```bash
.venv/bin/pip install -e . pytest pytest-asyncio
.venv/bin/pytest                          # 75 tests (unit, integración, CLI, golden)
cd frontend && pnpm install && pnpm build  # compila el dashboard → src/odotoken/static/
```

## Licencia

GPL-3.0-or-later. Ver [LICENSE](LICENSE).

## Contribuir

Los PR son bienvenidos — ejecuta la suite de tests y regenera `prices.json` vía el script de sync en lugar de editarlo a mano (excepto entradas con `"override": true`).