# odotoken
Proxy transparente de medición de tokens de IA entre tus agentes CLI y los proveedores LLM. Mide uso, calcula costo real y aplica límites de presupuesto.
**v0.6.0** · 21 paletas de colores inspiradas en frutas del Acre · [Português](README.md) · [English](README.en.md)
## Cómo funciona
```
agentes CLI → odotoken :8090 → tus proxies/routers → proveedores LLM
│
├─ medición (JSON + SSE)
├─ costo real (2.700+ modelos)
├─ guarda de presupuesto (alerta / bloqueo 429)
└─ historial SQLite → dashboard React (/app/)
```
Enrutamiento por prefijo de path: cada clave en `providers` se convierte en una ruta (`/anthropic/...`, `/openai/...`, o cualquier custom). Tokens extraídos de JSON y SSE. Cada petición registrada con tokens de prompt/completion/cache, costo, latencia y estado.
## Instalación
Binario listo en [Releases](https://github.com/ens-Emilio/odotoken/releases/latest), o desde fuente:
```bash
git clone https://github.com/ens-Emilio/odotoken.git && cd odotoken
go build -o bin/odotoken ./cmd/odotoken # Go 1.26+
cd frontend && pnpm install && pnpm build # opcional; los releases ya incluyen el dashboard
```
## Configuración mínima
```bash
cp config.json.example config.json
```
```jsonc
{
"proxy": { "host": "127.0.0.1", "port": 8090 },
"providers": {
"anthropic": { "upstream_url": "https://api.anthropic.com" },
"openai": { "upstream_url": "https://api.openai.com" }
},
"guard": {
"daily_cost_limit_usd": 10,
"monthly_cost_limit_usd": 100,
"action_on_breach": "block"
},
"database": { "path": "odotoken.db" }
}
```
Las claves de API nunca se guardan en el config — tus CLIs envían `Authorization`/`x-api-key` y el proxy las reenvía intactas.
Apunta tus CLIs a `http://127.0.0.1:8090`:
| CLI | Cómo |
|-----|------|
| Claude Code | `~/.claude/settings.json` → `"env": {"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"}` |
| crush | `--base-url "http://127.0.0.1:8090/openai/v1"` |
| opencode | `baseURL: "http://127.0.0.1:8090/<clave>"` |
```bash
odotoken start # proxy + dashboard
# o como servicio:
odotoken install-service && systemctl --user enable --now odotoken
```
Dashboard en http://127.0.0.1:8090/app/ — el uso aparece en hasta 10s.
## Paletas de colores
21 paletas extraídas de los tonos reales de cáscara, pulpa y hojas de frutas nativas del Acre (Açaí, Cupuaçu, Pupunha, Tucumã, Bacuri, Biribá, Buriti, Bacaba, Patauá, Ingá, Taperebá, Abiu, Uxi, Araçá-boi, Banana, Abacaxi, Graviola, Maracujá, Manga, Mamão, Melancia). Selector en el header del dashboard (círculo de color junto al tema), persistido en `localStorage`, contraste AA en modos claro y oscuro.
## Funcionalidades
- Medición transparente — cero cambios en el flujo de los CLIs
- Precios reales — 2.700+ modelos, sync vía `node scripts/sync-prices.mjs`
- Guarda de presupuesto — límites diarios/mensuales de costo y tokens, alerta o bloqueo (429)
- Dashboard React — KPIs, gráficos, tablas ordenables, stream de eventos, temas claro/oscuro/sistema
- CLI — `odotoken stats --by-model`, `log`, `count`, `models`, `sessions`
- API de conteo standalone — `odotoken serve` en puerto 8081
- Throughput — TPS/TTFT por modelo (`GET /api/tps`)
- Replay & optimize — repricing histórico, sugiere modelos más baratos
- Tags & forecast — `X-Odo-Tags`, forecast lineal + detección de anomalías
- Alertas — Slack/Discord/Telegram/SMTP en breach; export CSV/SVG
- Runs en vivo — `GET /api/sessions/live`; `X-Odo-Session` agrupa llamadas de una conversación
- Import de sesiones locales — `odotoken sessions import` lee OMP/Claude/Codex/Crush ya guardados en disco
## CLI
```bash
odotoken start [--host] [--port] [--reload]
odotoken serve [--port 8081]
odotoken status
odotoken stats [--by-model] [--by-client] [--since ISO] [--days N] [--tps]
odotoken log [--limit 20]
odotoken count text "..." [--model X] [--output json]
odotoken count chat --file msgs.json
odotoken count batch requests.jsonl
odotoken models list | info <id>
odotoken sessions scan | import [--dry-run] [--cli omp|claude|codex|crush] [--since ISO] | stats
odotoken replay
odotoken optimize
```
## HTTP API
Proxy (puerto 8090):
```
GET /health
GET /api/stats?since=ISO&project=&source=&tag=&tz=
GET /api/guard/status
GET /api/filters
GET /api/tps
GET /api/sessions/live
GET /api/export?format=csv
GET /api/stats.svg
GET /metrics
```
Conteo standalone (puerto 8081):
```
POST /v1/count {"model","text"}
POST /v1/count/batch {"items":[...]}
GET /v1/models
GET /health | /ready | /metrics
```
## Configuración
| Clave | Descripción | Por defecto |
|-------|-------------|-------------|
| `proxy.host` / `proxy.port` | bind | `127.0.0.1:8090` |
| `providers.<k>.upstream_url` | upstream de la ruta por prefijo | — |
| `proxy.upstream_openai` / `upstream_anthropic` | fallback para peticiones auto-detectadas | APIs oficiales |
| `guard.daily_cost_limit_usd` / `monthly_cost_limit_usd` | límites de costo (0 = off) | `null` |
| `guard.daily_tokens_limit` / `monthly_tokens_limit` | límites de tokens (0 = off) | `null` |
| `guard.warning_threshold_pct` | % para warning | `80` |
| `guard.action_on_breach` | `alert` o `block` (429) | `block` |
| `guard.timezone` | zona IANA de los límites | UTC |
| `limits.webhook_url` | webhook en warning/breach | `null` |
| `database.path` | SQLite WAL | `odotoken.db` |
| `token_counter.enabled` | middleware pre-vuelo | `true` |
| `sessions.enabled` / `sessions.paths` | import local | `true` |
Overrides de precio: `"override": true` en una entrada de `prices.json` impide que el sync la sobrescriba.
## Troubleshooting
Todos los CLIs fallan con `ECONNREFUSED 127.0.0.1:8090` → el proxy cayó, no los providers.
```bash
odotoken status
curl http://127.0.0.1:8090/health
journalctl --user -u odotoken -n 50 --no-pager
```
- `status=203/EXEC` → `ExecStart` apunta a path inexistente; ejecuta `odotoken install-service` desde la raíz del proyecto
- `activating (auto-restart)` loop → lee el journal, corrige la causa, `systemctl --user restart odotoken`
- `/health` OK pero los CLIs fallan → configs de los CLIs stale; reinicia el CLI
Regla: error con estado HTTP (503 etc.) → problema upstream. Error de conexión sin estado → el proxy cayó.
## Seguridad
- Proceso único — SQLite + caché en memoria asumen 1 worker
- Bind en `127.0.0.1` por defecto
- Las claves pasan en los headers; nada se registra más allá de tokens, modelo, cliente, costo y latencia
- Colócalo detrás de auth/reverse-proxy antes de exponerlo en red
## Desarrollo
```bash
make build
make test
cd frontend && pnpm install && pnpm build
```
## Licencia
GPL-3.0-or-later. Ver [LICENSE](LICENSE).