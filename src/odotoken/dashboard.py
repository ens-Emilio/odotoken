"""Web Dashboard and API server for odotoken metrics and statistics."""
from __future__ import annotations

import base64
import os
import time
from pathlib import Path
from typing import Any

import httpx
from fastapi import FastAPI, Request
from fastapi.responses import FileResponse, HTMLResponse, JSONResponse, PlainTextResponse, StreamingResponse
from fastapi.staticfiles import StaticFiles

from odotoken.db import Database
from odotoken.guard import Guard


def _auth_config() -> dict[str, Any]:
    import json as _j
    for cand in [Path("config.json"), Path(__file__).parent.parent.parent.parent / "config.json"]:
        if cand.exists():
            try:
                return _j.loads(cand.read_text())
            except Exception:
                pass
    return {}


def _check_auth(request: Request) -> bool:
    auth = _auth_config()
    if not auth:
        return True
    bearer = auth.get("bearer_token")
    env = auth.get("bearer_token_env")
    if not bearer and env:
        bearer = os.environ.get(env)
    basic = auth.get("basic") or {}
    b_user = basic.get("user") if isinstance(basic, dict) else None
    b_pass = basic.get("pass") if isinstance(basic, dict) else None
    b_env = basic.get("pass_env") if isinstance(basic, dict) else None
    if not b_pass and b_env:
        b_pass = os.environ.get(b_env)
    hdr = request.headers.get("authorization", "")
    if bearer and hdr.startswith("Bearer ") and hdr[7:].strip() == str(bearer):
        return True
    if b_user and b_pass and hdr.startswith("Basic "):
        try:
            import base64 as _b64
            decoded = _b64.b64decode(hdr[6:]).decode()
            u, p = decoded.split(":", 1)
            return u == b_user and p == b_pass
        except Exception:
            pass
    has_creds = bool(bearer or (b_user and b_pass))
    if not has_creds:
        return True
    return False


DASHBOARD_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<meta name="color-scheme" content="light dark" />
<title>odotoken — Token &amp; Cost Dashboard</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='8' fill='%23202020'/%3E%3Ctext x='16' y='22' font-size='18' font-weight='bold' text-anchor='middle' fill='%2387ea5c' font-family='sans-serif'%3E%CF%89%3C/text%3E%3C/svg%3E" />
<script>
const m = localStorage.getItem('odt-theme') || 'system'
const dark = m === 'dark' || (m === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)
if (dark) document.documentElement.classList.add('dark')
if (location.pathname === '/app') location.replace('/app/' + location.hash)
</script>
<script type="module" crossorigin src="/app/assets/index-BaMM1x8W.js"></script>
<link rel="stylesheet" crossorigin href="/app/assets/index-bCuhMTug.css">
</head>
<body>
<div id="root"></div>
</body>
</html>"""


def register_dashboard_routes(app: FastAPI, db: Database, guard: Guard | None = None) -> None:
    @app.middleware("http")
    async def add_security_headers(request: Request, call_next):
        response = await call_next(request)
        response.headers["X-Content-Type-Options"] = "nosniff"
        response.headers["X-Frame-Options"] = "DENY"
        return response

    @app.get("/health")
    async def health_check():
        return {"status": "ok"}

    @app.get("/favicon.ico")
    async def favicon():
        return FileResponse(
            Path(__file__).parent.parent.parent.parent / "frontend" / "public" / "favicon.svg",
            media_type="image/svg+xml"
        )

    @app.get("/")
    async def root_redirect():
        from fastapi.responses import RedirectResponse
        return RedirectResponse(url="/app/", status_code=307)

    @app.get("/app/{path:path}")
    async def serve_spa(path: str):
        static_dir = Path(__file__).parent / "static"
        file_path = static_dir / path
        if file_path.exists() and file_path.is_file():
            return FileResponse(file_path)
        return FileResponse(static_dir / "index.html")

    # --- Proxy helpers -----------------------------------------------------------
    def _load_proxy_config() -> dict:
        cfg: dict = {}
        for cand in [Path("config.json"), Path(__file__).parent.parent.parent.parent / "config.json"]:
            if cand.exists():
                try:
                    cfg = __import__("json").loads(cand.read_text())
                    break
                except Exception:
                    pass
        return cfg

    async def _proxy_request(request: Request, upstream_url: str, provider_name: str, source_label: str):
        """Forward the incoming request to *upstream_url*, preserving body/headers/model.
        Records token usage into the database afterwards (best-effort, non-blocking)."""
        body = await request.body()
        headers = {k: v for k, v in request.headers.items() if k.lower() != "host"}

        start = time.monotonic()
        async with httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(upstream_url, headers=headers, content=body)
        elapsed = round((time.monotonic() - start) * 1000, 1)

        # Record metrics best-effort
        try:
            from odotoken.db import TokenEvent
            import json as _j
            payload = _j.loads(body) if body else {}
            model = payload.get("model", "unknown")
            prompt_tokens = 0
            completion_tokens = 0
            cost_usd = 0.0
            # Try to parse usage from response if available
            try:
                rbody = resp.json()
                usage = rbody.get("usage", {})
                prompt_tokens = usage.get("prompt_tokens", 0) or 0
                completion_tokens = usage.get("completion_tokens", 0) or 0
                total = usage.get("total_tokens", 0) or 0
                if not prompt_tokens and not completion_tokens and total:
                    prompt_tokens = int(total * 0.8)
                    completion_tokens = total - prompt_tokens
            except Exception:
                pass
            event = TokenEvent(
                timestamp=__import__("datetime").datetime.now(__import__("datetime").timezone.utc).isoformat(),
                provider=provider_name,
                model=model,
                client_app=source_label,
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                total_tokens=prompt_tokens + completion_tokens,
                cost_usd=cost_usd,
                latency_ms=elapsed,
                ttfb_ms=0.0,
                streaming_ms=0.0,
                status_code=resp.status_code,
                source=f"proxy:{source_label}",
                session_file="",
                project_id="default",
                tags="",
                metadata="{}",
                shadow_cost=0.0,
                session_id="",
                entry_id="",
                cache_read_tokens=0,
                cache_write_tokens=0,
            )
            db.record_event(event)
        except Exception:
            pass

        # Return response faithfully
        out_headers = dict(resp.headers)
        out_headers.pop("transfer-encoding", None)
        return StreamingResponse(
            iter([resp.content]),
            status_code=resp.status_code,
            headers=out_headers,
        )
    @app.api_route("/kodeo/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
    async def proxy_kodeo(request: Request, path: str):
        cfg = _load_proxy_config()
        upstream = cfg.get("providers", {}).get("kodeo", {}).get("upstream_url", "https://api.kodeo.com.br/v1")
        clean_path = path.lstrip("/")
        if clean_path.startswith("v1/"):
            clean_path = clean_path[3:]
        url = f"{upstream.rstrip('/')}/{clean_path}"
        return await _proxy_request(request, url, "kodeo", "kodeo")

    @app.api_route("/pegasus/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
    async def proxy_pegasus(request: Request, path: str):
        cfg = _load_proxy_config()
        upstream = cfg.get("providers", {}).get("pegasus", {}).get("upstream_url", "https://api.pegasusrelay.com/v1")
        clean_path = path.lstrip("/")
        if clean_path.startswith("v1/"):
            clean_path = clean_path[3:]
        url = f"{upstream.rstrip('/')}/{clean_path}"
        return await _proxy_request(request, url, "pegasus", "pegasus")

    # --- Existing API routes -----------------------------------------------------
    @app.get("/api/stats")
    async def stats_api(since: str | None = None, project: str | None = None, source: str | None = None, group_by: str | None = None, tag: str | None = None):
        from datetime import datetime, timedelta, UTC

        def _resolve_since(s: str | None) -> str | None:
            now = datetime.now(UTC)
            if s == "today":
                return now.replace(hour=0, minute=0, second=0, microsecond=0).isoformat()
            if s and s.endswith("d"):
                try:
                    days = int(s[:-1])
                    return (now - timedelta(days=days)).isoformat()
                except ValueError:
                    pass
            return s

        resolved_since = _resolve_since(since)
        summary = db.get_summary(since=resolved_since, project=project, source=source, tag=tag)
        by_model = db.get_summary_by_model(since=resolved_since, project=project, source=source)
        by_client = db.get_summary_by_client(since=resolved_since, project=project, source=source)
        by_project = db.get_summary_by_project(since=resolved_since, source=source)
        by_source = db.get_summary_by_source(since=resolved_since, project=project)
        recent_events = db.get_recent_events(limit=50, since=resolved_since, project=project, source=source, tag=tag)
        tps = db.get_tps_summary(since=resolved_since, project=project, source=source)
        daily_grouped = None
        if group_by == "source":
            daily_grouped = db.get_daily_usage_grouped(days=30, since=resolved_since)
        daily = daily_grouped if daily_grouped is not None else db.get_daily_usage(days=30, since=resolved_since, source=source)
        if guard:
            check = guard.check_allowance()
            guard_info = {
                "limits": guard.limits.to_dict(),
                "status": check.status,
                "allowed": check.allowed,
                "reason": check.reason,
                "current": {
                    "daily_cost_usd": check.current_daily_cost,
                    "monthly_cost_usd": check.current_monthly_cost,
                    "daily_tokens": check.current_daily_tokens,
                    "monthly_tokens": check.current_monthly_tokens,
                },
            }
            proj_status: dict = {}
            for pid in guard.limits.projects:
                pc = guard.check_project(pid)
                proj_status[pid] = {
                    "limits": guard.limits.projects[pid].to_dict(),
                    "status": pc.status,
                    "allowed": pc.allowed,
                    "reason": pc.reason,
                    "current": {
                        "daily_cost_usd": pc.current_daily_cost,
                        "monthly_cost_usd": pc.current_monthly_cost,
                        "daily_tokens": pc.current_daily_tokens,
                        "monthly_tokens": pc.current_monthly_tokens,
                    },
                }
            guard_info["projects"] = proj_status
        by_model = [m for m in by_model if m.get("model") or m.get("total_tokens")]
        by_client = [c for c in by_client if c.get("total_tokens") or c.get("requests")]
        by_tag = db.get_summary_by_tag(since=resolved_since, project=project, source=source)
        savings = {
            "actual_usd": summary.get("total_cost_usd", 0.0),
            "shadow_usd": summary.get("total_shadow_cost_usd", 0.0),
            "saved_usd": max(0.0, summary.get("total_shadow_cost_usd", 0.0) - summary.get("total_cost_usd", 0.0)),
        }
        return JSONResponse({
            "summary": summary,
            "by_model": by_model,
            "by_client": by_client,
            "by_project": by_project,
            "savings": savings,
            "by_tag": by_tag,
            "recent_events": recent_events,
            "daily": daily,
            "by_source": by_source,
            "tps": tps,
            "guard": guard_info if guard else None,
        })

    @app.patch("/api/guard/limits")
    def patch_limits(body: dict):
        import json as _jg
        from pathlib import Path as _Pg
        cfg_path = None
        for cand in [_Pg("config.json"), _Pg(__file__).parent.parent.parent.parent / "config.json"]:
            if cand.exists():
                cfg_path = cand
                break
        if not cfg_path:
            cfg_path = _Pg("config.json")
        cfg: dict = {}
        if cfg_path.exists():
            try:
                cfg = _jg.loads(cfg_path.read_text())
            except Exception:
                cfg = {}
        limits = cfg.get("limits") or {}
        for k in ["daily_cost_limit_usd","monthly_cost_limit_usd","daily_tokens_limit","monthly_tokens_limit","action_on_breach","webhook_url"]:
            if k in body:
                limits[k] = body[k]
        if "alerts" in body:
            if body["alerts"] is None:
                limits.pop("alerts", None)
            elif isinstance(body["alerts"], dict):
                limits["alerts"] = body["alerts"]
        if "projects" in body:
            if body["projects"] is None:
                limits.pop("projects", None)
            elif isinstance(body["projects"], dict):
                existing = limits.get("projects") or {}
                for pid, pv in body["projects"].items():
                    if pv is None:
                        existing.pop(pid, None)
                    else:
                        existing[pid] = pv
                if existing:
                    limits["projects"] = existing
                else:
                    limits.pop("projects", None)
        cfg_path.write_text(_jg.dumps(cfg, indent=2) + "\n")
        if guard:
            from odotoken.guard import GuardLimits
            guard.update_limits(GuardLimits.from_dict(limits))
        if guard:
            try:
                from odotoken.guard import dispatch_alerts
                check = guard.check_allowance()
                if check.status in ("warning","breached"):
                    dispatch_alerts(guard.limits, check)
                    for pid, pl in guard.limits.projects.items():
                        if pl.webhook_url or pl.alerts:
                            pr = guard.check_project(pid)
                            if pr.status in ("warning","breached"):
                                dispatch_alerts(pl, pr)
            except Exception:
                pass
        return JSONResponse({"limits": limits})

    def _export_rows(since, project, source, tag=None):
        return db.get_recent_events(limit=5000, since=since, project=project, source=source, tag=tag)
    def _csv_response(rows):
        import csv
        import io
        buf = io.StringIO()
        fields = ["id","timestamp","provider","model","client_app","prompt_tokens","completion_tokens","total_tokens","cost_usd","latency_ms","ttfb_ms","streaming_ms","source"]
        w = csv.DictWriter(buf, fieldnames=fields)
        w.writeheader()
        for r in rows:
            w.writerow({k: r.get(k,"") for k in fields})
        return PlainTextResponse(buf.getvalue(), media_type="text/csv", headers={"Content-Disposition": "attachment; filename=odotoken.csv"})

    @app.get("/api/export")
    async def export_api(since: str | None = None, project: str | None = None, source: str | None = None, format: str = "csv"):
        rows = _export_rows(since, project, source)
        if format == "json":
            return JSONResponse(rows)
        if format not in ("csv", "json"):
            format = "csv"
        return _csv_response(rows)

    @app.get("/api/stats.csv")
    async def stats_csv(since: str | None = None, project: str | None = None, source: str | None = None, tag: str | None = None):
        return _csv_response(_export_rows(since, project, source, tag))

    @app.get("/api/forecast")
    def forecast_api(tag: str | None = None, project: str | None = None):
        import math
        from datetime import date, timedelta
        daily = db.get_daily_usage(days=30, source=None)
        if tag:
            with db.connection() as conn:
                rows = conn.execute(
                    "SELECT substr(timestamp,1,10) as day, SUM(cost_usd) as cost_usd FROM token_events WHERE (',' || tags || ',') LIKE ? GROUP BY day ORDER BY day ASC",
                    [f"%,{tag},%"],
                ).fetchall()
                daily = [dict(r) for r in rows]
                for r in daily:
                    r.setdefault("requests", 0)
                    r.setdefault("total_tokens", 0)
        costs = [float(r.get("cost_usd") or 0) for r in daily]
        n = len(costs)
        if n >= 2:
            xs = list(range(n))
            mx = sum(xs) / n
            my = sum(costs) / n
            denom = sum((x - mx) ** 2 for x in xs) or 1
            a = sum((x - mx) * (y - my) for x, y in zip(xs, costs)) / denom
            b = my - a * mx
        elif n == 1:
            a, b = 0, costs[0]
        else:
            a, b = 0, 0
        forecast = []
        last_day = daily[-1]["day"] if daily else date.today().isoformat()
        try:
            base = date.fromisoformat(last_day)
        except Exception:
            base = date.today()
        for i in range(1, 8):
            d = (base + timedelta(days=i)).isoformat()
            val = max(0, a * (n + i - 1) + b)
            forecast.append({"day": d, "cost_usd": round(val, 6)})
        anomalies: list[dict] = []
        if n >= 3:
            mean = sum(costs) / n
            var = sum((c - mean) ** 2 for c in costs) / n
            std = math.sqrt(var)
            thresh = mean + 2 * std
            for r in daily:
                if float(r.get("cost_usd") or 0) > thresh and thresh > 0:
                    anomalies.append({"day": r["day"], "cost_usd": float(r["cost_usd"]), "threshold": round(thresh, 6)})

    @app.get("/api/events")
    def events_api(since: str | None = None, project: str | None = None, source: str | None = None, tag: str | None = None, limit: int = 50):
        """Return latest N raw token events, optionally filtered."""
        if source == "all":
            source = None
        rows = db.get_recent_events(limit=limit, since=since, project=project, source=source, tag=tag)
        return JSONResponse(rows)
    @app.get("/api/tps")
    def tps_api(since: str | None = None, interval: str = "1h", project: str | None = None, source: str | None = None):
        """Throughput time-series + summary + breakdowns by model/project."""
        if source == "all":
            source = None
        series = db.get_tps_series(since=since, interval=interval, project=project, source=source)
        summary = db.get_tps_summary(since=since, project=project, source=source)
        by_model = db.get_tps_by_model(since=since, project=project, source=source)
        by_project = db.get_tps_by_project(since=since, source=source)
        # live_tps: most recent non-zero bucket avg_tps
        live_tps = 0.0
        for b in reversed(series):
            v = float(b.get("avg_tps") or 0)
            if v > 0:
                live_tps = v
                break
        return JSONResponse({
            "series": series,
            "summary": summary,
            "by_model": by_model,
            "by_project": by_project,
            "live_tps": live_tps,
        })

    @app.post("/api/sessions/import")
    def sessions_import(since: str | None = None, dry_run: bool = False):
        """Trigger import of local CLI sessions. Synchronous, returns stats."""
        import json as _json
        from pathlib import Path as _Path
        from odotoken.importers.service import import_sessions as _import
        from odotoken.pricing import PricingCalculator as _PC
        cfg = {}
        for cand in [_Path("config.json"), _Path(__file__).parent.parent.parent.parent / "config.json"]:
            if cand.exists():
                try:
                    cfg = _json.loads(cand.read_text())
                    break
                except Exception:
                    pass
        pricing = _PC(_Path("prices.json") if _Path("prices.json").exists() else "prices.json")
        stats = _import(db, pricing, config=cfg, since=since, dry_run=dry_run)
        return JSONResponse({"scanned": stats.scanned, "inserted": stats.inserted, "skipped_dup": stats.skipped_dup, "cost_usd": stats.cost_usd, "by_cli": stats.by_cli, "dry_run": dry_run})

    @app.get("/api/sessions/scan")
    async def sessions_scan():
        import json as _json2
        from pathlib import Path as _Path2
        from odotoken.importers.registry import scan_all as _scan
        cfg2 = {}
        for cand in [_Path2("config.json"), _Path2(__file__).parent.parent.parent.parent / "config.json"]:
            if cand.exists():
                try:
                    cfg2 = _json2.loads(cand.read_text())
                    break
                except Exception:
                    pass
        results = _scan(cfg2)
        return JSONResponse([r.__dict__ for r in results])

    @app.get("/api/sessions/live")
    def sessions_live():
        """Currently-open sessions across agents (active in last 2h), with provider/model."""
        from datetime import UTC, datetime, timedelta

        live: list[dict] = []
        cutoff_epoch = int((datetime.now(UTC) - timedelta(hours=2)).timestamp())
        with db.connection() as c:
            rows = c.execute(
                "SELECT source, session_file, project_id, provider, model,"
                " SUM(prompt_tokens) AS p, SUM(completion_tokens) AS o,"
                " SUM(cost_usd) AS ccost, MAX(timestamp) AS last_ts"
                " FROM token_events"
                " WHERE timestamp != ''"
                " AND strftime('%s', timestamp) >= ?"
                " GROUP BY source, session_file, provider, model"
                " ORDER BY last_ts DESC LIMIT 50",
                (str(cutoff_epoch),)
            )
            for r in rows:
                src = r["source"]
                cli_label = src.split(":", 1)[-1] if ":" in src else src
                live.append({
                    "cli": cli_label,
                    "session_id": r["session_file"],
                    "provider": r["provider"],
                    "model": r["model"],
                    "tokens_input": int(r["p"] or 0),
                    "tokens_output": int(r["o"] or 0),
                    "cost_usd": float(r["ccost"] or 0.0),
                    "project": r["project_id"] or "default",
                    "last_updated": r["last_ts"] or "",
                })
        return JSONResponse(live)

    app.router.routes.sort(key=lambda r: 1 if "{" in getattr(r, "path", "") else 0)


# Module-level initialization
app = FastAPI()
db = Database()
guard: Guard | None = None
try:
    guard = Guard.from_config(Path("config.json"))
except Exception:
    pass
register_dashboard_routes(app, db, guard)

import asyncio
from contextlib import asynccontextmanager

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Background sniffer task
    async def sniffer_loop():
        from odotoken.importers.service import import_sessions
        from odotoken.pricing import PricingCalculator
        pricing = PricingCalculator()
        while True:
            try:
                await asyncio.to_thread(import_sessions, db, pricing)
            except Exception:
                pass
            await asyncio.sleep(30)
    
    task = asyncio.create_task(sniffer_loop())
    yield
    task.cancel()

app.router.lifespan_context = lifespan