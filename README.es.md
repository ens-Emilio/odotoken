# odotoken

Proxy transparente de medición de tokens de IA entre tus agentes CLI y los proveedores LLM. Mide uso, calcula costo real y aplica límites de presupuesto.

**v0.8.1** · dashboard en 20 idiomas · importación de sesiones locales · [Português (Brasil)](README.pt-BR.md) · [English](README.md) · [todos los quick starts](docs/i18n/README.md)

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
| `server.client_keys` | claves de cliente con `client`, `project` y cuotas opcionales; se gestionan por `PUT /api/client-keys` y se muestran enmascaradas | vacío |
| `server.cors_allowlist` | orígenes de navegador permitidos | vacío |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | límites de costo, `0` desactiva | `0` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | límites de tokens, `0` desactiva | `0` |
| `guard.warning_threshold_pct` | umbral de aviso | `80` |
| `guard.action_on_breach` | `alert` o `block` (429) | `block` |
| `guard.timezone` | zona IANA de las ventanas diaria/mensual | `UTC` |
| `limits.rate_limit` | `enabled`, `rps`, `burst_size`, con `projects`/`clients` por `X-Odo-Project` / `X-Odo-Client` | desactivado |
| `alerts.webhook_url` / `alerts.slack_url` / `discord_url` / `telegram` / `email` | avisos de breach | vacío |
| `sessions.enabled` / `sessions.paths` | fuentes de importación local | `true` / vacío |
| `providers.<nombre>.fallbacks` / `fallback_on_status` | reintenta en otro proveedor ante 429/5xx (por defecto 429/500/502/503/504) | ninguno |
| `guard.projects.<id>` | límites propios por proyecto | hereda el global |
| `alerts.retry` | backoff de la cola duradera: `max_attempts`, `base_delay_seconds`, `max_delay_seconds` | 5 / 30 / 3600 |
| `storage.token_events_retention_days` / `vacuum_after_retention` | retención de eventos y VACUUM opcional | `0` / `false` |
| `scheduled_import.enabled` / `interval` / `clis` | importación programada con el daemon activo | `false` / `24h` |
| `otel.enabled` / `endpoint` / `interval_seconds` | envía métricas OTLP/HTTP JSON a `endpoint/v1/metrics` con el daemon activo | `false` / vacío / `60` |

El puerto es una flag, no una clave de config: `--port` vale `8080` por defecto y `--host` sobrescribe `server.bind_host`. El bloque legado `limits` de presupuesto todavía se lee y se pliega en `guard`, así que un config antiguo conserva sus límites.

Las claves del provider nunca viven en ese archivo: tu CLI envía `Authorization` o `x-api-key` y el proxy reenvía el header intacto. Las claves de cliente en `server.client_keys` son secretos de trabajo en `config.json` (escrito como `0600`); mantén el archivo privado y revoca una clave expuesta con `DELETE /api/client-keys/{clave-enmascarada}`.

Los dos límites se comprueban siempre, y cada uno se mide donde aplica: los
límites del proyecto contra todo lo que gastó el proyecto, la cuota de una clave
contra el gasto de esa clave dentro del proyecto. Una clave dentro de su cuota
sigue bloqueada cuando el proyecto entero se pasó, y una clave sin cuota no
supera los límites del proyecto.

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
- Prometheus — `GET /metrics` expone contadores de costo, tokens y peticiones; el envío OTLP/HTTP es opcional
- Resiliencia — fallback de proveedor en 429/5xx configurables, rate limit por proyecto/cliente, reintento duradero de alertas e importación programada
- Operación — `odotoken doctor` revisa configuración, base, alertas y agenda; `odotoken export` y `odotoken prices` funcionan sin el daemon

## CLI

```bash
odotoken start [--host] [--port] [--config] [--db]
odotoken stop
odotoken status
odotoken stats
odotoken export [--format csv|json] [--since RFC3339] [--until RFC3339] [--project ID] [--source NOMBRE] [--tag TAG] [--limit N] [--out RUTA|-] [--json]
odotoken prices list [--query TEXTO] [--limit N] [--json]
odotoken prices set --model NOMBRE --input N --output N [--cache-read N] [--cache-write N]
odotoken prices delete --model NOMBRE
odotoken import [cli...] [--since RFC3339] [--cli NOMBRE] [--dry-run] [--discover] [--json] [--lang LOCALE]
odotoken doctor [--config] [--db] [--json] [--flush-alerts] [--vacuum]
odotoken mcp [--config] [--db]
```

`--lang` elige el idioma de la salida humana. Sin él, la CLI lee `LC_ALL`, luego `LC_MESSAGES`, luego `LANG`, y cae en inglés. La salida `--json` nunca se traduce.

`import` sin argumentos lee todos los CLIs detectados. `--discover` lista lo que existe en la máquina sin escribir nada. Los costos se recalculan desde `prices.json` en vez de confiarse al archivo de origen.

`export` lee la base directamente, así que funciona con el daemon detenido. Usa la misma consulta y el mismo contrato CSV que `GET /api/events.csv`. `--out -` escribe en la salida estándar; con `--out ARCHIVO`, la bandera `--json` imprime un resumen con el recuento de filas en vez de las filas.

`prices` edita `prices.json` (`--prices` apunta a otra ruta). `list` marca cada fila como `override` o `builtin`. `delete` solo elimina overrides — un precio builtin se rechaza, porque el modelo caería a $0 y dejaría de medirse.

## API HTTP

Proxy (`:8080`):

```
GET   /health             # liveness del propio odotoken: sin clave, nunca proxeado
GET   /metrics            # exposición Prometheus
GET   /api/events.csv
GET   /api/sessions
GET   /api/sessions/{id}
GET   /api/cache-savings
POST  /api/simulate
GET|PUT /api/client-keys
DELETE /api/client-keys/{clave-enmascarada}
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

Conteo standalone (`/v1`):

```
POST /v1/count        {"model","text"}
POST /v1/count/batch  {"items":[...]}
GET  /v1/models
GET  /v1/health       # health de la API; exige la clave cuando server.api_key está definido
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
