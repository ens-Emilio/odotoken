# odotoken

Proxy transparente de medición de tokens de IA entre tus agentes CLI y los proveedores LLM. Mide uso, calcula costo real y aplica límites de presupuesto.

**v0.7.0** · dashboard en 20 idiomas · importación de sesiones locales · [Português (Brasil)](README.pt-BR.md) · [English](README.md) · [todos los quick starts](docs/i18n/README.md)

## Cómo funciona

```
agentes CLI → odotoken :8080 → proveedores configurados
                  │
                  ├─ medición (JSON + SSE)
                  ├─ costo desde prices.json
                  ├─ guarda de presupuesto (alerta / bloqueo 429)
                  └─ historial SQLite → dashboard (/app/)
```

Cada clave en `providers` se convierte en una ruta. Una petición a `/openai/chat/completions` va al `upstream_url` de ese proveedor con el prefijo eliminado; sin prefijo coincidente, el proxy detecta el proveedor por el cuerpo. Los tokens se leen de JSON y SSE, y cada petición se guarda con tokens de prompt, completion y cache, costo, latencia y estado.

## Instalación

Binario listo en [Releases](https://github.com/ens-Emilio/odotoken/releases/latest). Desde el código fuente:

```bash
git clone https://github.com/ens-Emilio/odotoken.git && cd odotoken
go build -o bin/odotoken ./cmd/odotoken
cd frontend && pnpm install && pnpm build
```

El dashboard se lee de `internal/server/static` relativo al directorio de trabajo, así que inicia el binario desde la raíz del proyecto o desde un directorio de instalación que contenga esa carpeta.

## Configuración

```bash
cp config.json.example config.json
```

| Clave | Significado | Valor por defecto |
|-------|-------------|-------------------|
| `server.bind_host` | dirección de bind | `127.0.0.1` |
| `server.api_key` | exige clave en `/api` y `/v1` | vacío (abierto) |
| `server.cors_allowlist` | orígenes de navegador permitidos | vacío |
| `providers.<nombre>.upstream_url` | destino de la ruta por prefijo | — |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | límites de costo, `0` desactiva | `0` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | límites de tokens, `0` desactiva | `0` |
| `guard.warning_threshold_pct` | umbral de aviso | `80` |
| `guard.action_on_breach` | `alert` o `block` (429) | `block` |
| `guard.timezone` | zona IANA de las ventanas diaria/mensual | `UTC` |
| `limits.rate_limit` | `enabled`, `rps`, `burst_size` | desactivado |
| `alerts.slack_url` / `discord_url` / `telegram` / `email` | avisos de breach | vacío |
| `sessions.enabled` / `sessions.paths` | fuentes de importación local | `true` / vacío |

El puerto es una flag, no una clave de config: `--port` vale `8080` por defecto y `--host` sobrescribe `server.bind_host`. El bloque legado `limits` de presupuesto todavía se lee y se pliega en `guard`, así que un config antiguo conserva sus límites.

Las claves de API nunca viven en ese archivo. Tu CLI envía `Authorization` o `x-api-key` y el proxy reenvía el header intacto.

```bash
odotoken start --config config.json --db odotoken.db
```

Dashboard: `http://127.0.0.1:8080/app/` — el uso aparece en hasta 10s. Ofrece 20 idiomas en **Ajustes → Idioma** o con `?lang=es` en un enlace compartido; árabe y persa invierten el diseño a RTL.

## Funcionalidades

- Medición transparente — cero cambios en el flujo de los CLIs
- Precios reales — 2.700+ modelos en `prices.json`, actualizados con `node scripts/sync-prices.mjs`
- Guarda de presupuesto — límites diarios y mensuales de costo y tokens, alerta o bloqueo con 429
- Dashboard — KPIs, gráficos, tablas ordenables, stream de eventos, temas claro/oscuro/sistema, 21 paletas de colores
- Throughput — TPS y TTFT por modelo (`GET /api/tps`)
- Forecast y anomalías — proyección lineal más detección de anomalías
- Alertas — Slack, Discord, Telegram, SMTP en breach; export CSV y SVG
- Runs en vivo — peticiones en vuelo agrupadas por conversación con `X-Odo-Session`
- Import de sesiones locales — la vista Importar y `odotoken import` comparten un solo motor y leen sesiones ya en disco de los CLIs listados en `sessions.paths`
- Historial de importación — conteos por CLI, insertados y duplicados, tokens, costo y estado terminal

## CLI

```bash
odotoken start [--host] [--port] [--config] [--db]
odotoken stop
odotoken status
odotoken stats
odotoken import [cli...] [--since RFC3339] [--cli NOMBRE] [--dry-run] [--discover] [--json] [--lang LOCALE]
```

`--lang` elige el idioma de la salida humana. Sin él, la CLI lee `LC_ALL`, luego `LC_MESSAGES`, luego `LANG`, y cae en inglés. La salida `--json` nunca se traduce.

`import` sin argumentos lee todos los CLIs detectados. `--discover` lista lo que existe en la máquina sin escribir nada. Los costos se recalculan desde `prices.json` en vez de confiarse al archivo de origen.

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

Conteo standalone (`/v1`):

```
POST /v1/count        {"model","text"}
POST /v1/count/batch  {"items":[...]}
GET  /v1/models
GET  /v1/health
```

## Troubleshooting

Un CLI fallando con `ECONNREFUSED 127.0.0.1:8080` significa que el proxy está caído, no el proveedor:

```bash
odotoken status
curl http://127.0.0.1:8080/health
```

Una respuesta con estado HTTP (503 y similares) es un problema del upstream. Un error de conexión sin estado es el proxy local.

## Seguridad

- Proceso único — SQLite y la cache in-memory asumen un worker
- Bind en `127.0.0.1` por defecto
- Las claves pasan en headers; solo se guardan tokens, modelo, client, costo y latencia
- Colócalo detrás de autenticación y reverse proxy antes de exponerlo en red

## Desarrollo

```bash
make build
make test
cd frontend && pnpm install && pnpm test && pnpm build
```

## Licencia

GPL-3.0-or-later. Ver [LICENSE](LICENSE).
