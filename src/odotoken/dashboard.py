"""Web Dashboard and API server for odotoken metrics and statistics."""

from __future__ import annotations

from pathlib import Path

from fastapi import FastAPI
from fastapi.responses import FileResponse, HTMLResponse, JSONResponse
from fastapi.staticfiles import StaticFiles

from odotoken.db import Database
from odotoken.guard import Guard

DASHBOARD_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>odotoken - AI Token & Cost Dashboard</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
  <style>
    body { background-color: #0f172a; color: #f8fafc; font-family: ui-sans-serif, system-ui, sans-serif; }
    .glass { background: rgba(30, 41, 59, 0.7); backdrop-filter: blur(12px); border: 1px solid rgba(255, 255, 255, 0.08); }
    .neon-border { border: 1px solid rgba(56, 189, 248, 0.2); }
  </style>
</head>
<body class="min-h-screen p-4 md:p-8">
  <div class="max-w-7xl mx-auto space-y-6">
    <!-- Header -->
    <header class="flex flex-col md:flex-row justify-between items-start md:items-center pb-6 border-b border-slate-800 gap-4">
      <div>
        <div class="flex items-center gap-3">
          <div class="h-9 w-9 rounded-xl bg-gradient-to-tr from-cyan-500 to-indigo-500 flex items-center justify-center font-bold text-lg text-white shadow-lg shadow-cyan-500/20">
            &omega;
          </div>
          <h1 class="text-2xl md:text-3xl font-bold tracking-tight bg-gradient-to-r from-white via-slate-200 to-slate-400 bg-clip-text text-transparent">
            odotoken
          </h1>
          <span class="px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            Active
          </span>
        </div>
        <p class="text-xs text-slate-400 mt-1">Autonomous AI Token Metering & Reverse Proxy</p>
      </div>
      <div class="flex items-center gap-3">
        <select id="timeFilter" onchange="fetchData()" class="glass text-xs text-slate-200 rounded-lg px-3 py-2 border border-slate-700 focus:outline-none focus:border-cyan-500">
          <option value="today">Today</option>
          <option value="7d">Last 7 Days</option>
          <option value="30d">Last 30 Days</option>
          <option value="all">All Time</option>
        </select>
        <button onclick="fetchData()" class="glass hover:bg-slate-700/60 transition px-3 py-2 rounded-lg text-xs font-medium flex items-center gap-1.5 border border-slate-700">
          <svg class="w-3.5 h-3.5 text-cyan-400 animate-spin-hover" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
      </div>
    </header>

    <!-- Top KPI Cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="glass p-5 rounded-2xl neon-border relative overflow-hidden">
        <div class="text-xs font-semibold uppercase tracking-wider text-slate-400">Total Spent</div>
        <div class="text-3xl font-extrabold text-cyan-400 mt-2" id="kpi-cost">$0.00</div>
        <div class="text-[11px] text-slate-500 mt-1" id="kpi-cost-sub">estimated USD</div>
      </div>

      <div class="glass p-5 rounded-2xl neon-border relative overflow-hidden">
        <div class="text-xs font-semibold uppercase tracking-wider text-slate-400">Total Tokens</div>
        <div class="text-3xl font-extrabold text-indigo-400 mt-2" id="kpi-tokens">0</div>
        <div class="text-[11px] text-slate-500 mt-1" id="kpi-tokens-sub">prompt + completion</div>
      </div>

      <div class="glass p-5 rounded-2xl neon-border relative overflow-hidden">
        <div class="text-xs font-semibold uppercase tracking-wider text-slate-400">Cache Savings</div>
        <div class="text-3xl font-extrabold text-emerald-400 mt-2" id="kpi-cache">0</div>
        <div class="text-[11px] text-slate-500 mt-1" id="kpi-cache-sub">cached read tokens</div>
      </div>

      <div class="glass p-5 rounded-2xl neon-border relative overflow-hidden">
        <div class="text-xs font-semibold uppercase tracking-wider text-slate-400">Total Requests</div>
        <div class="text-3xl font-extrabold text-purple-400 mt-2" id="kpi-requests">0</div>
        <div class="text-[11px] text-slate-500 mt-1" id="kpi-latency">avg latency: 0ms</div>
      </div>
    </div>

    <!-- Charts Row -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="glass p-5 rounded-2xl lg:col-span-2 space-y-4">
        <h3 class="text-sm font-semibold text-slate-200">Daily Consumption History</h3>
        <div class="h-64">
          <canvas id="dailyChart"></canvas>
        </div>
      </div>
      <div class="glass p-5 rounded-2xl space-y-4">
        <h3 class="text-sm font-semibold text-slate-200">Cost by Model</h3>
        <div class="h-64 flex items-center justify-center">
          <canvas id="modelPieChart"></canvas>
        </div>
      </div>
    </div>

    <!-- Tables Row: Breakdown by App & Breakdown by Model -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Client Apps -->
      <div class="glass p-5 rounded-2xl space-y-3">
        <h3 class="text-sm font-semibold text-slate-200">Usage by Client App (pi, omp, opencode)</h3>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="text-slate-400 border-b border-slate-700/60 pb-2">
              <tr>
                <th class="py-2">Client App</th>
                <th class="py-2">Requests</th>
                <th class="py-2">Total Tokens</th>
                <th class="py-2 text-right">Cost (USD)</th>
              </tr>
            </thead>
            <tbody id="clientsTableBody" class="divide-y divide-slate-800 text-slate-300">
              <tr><td colspan="4" class="py-4 text-center text-slate-500">No events recorded</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Models Table -->
      <div class="glass p-5 rounded-2xl space-y-3">
        <h3 class="text-sm font-semibold text-slate-200">Model Breakdown</h3>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="text-slate-400 border-b border-slate-700/60 pb-2">
              <tr>
                <th class="py-2">Model</th>
                <th class="py-2">Requests</th>
                <th class="py-2">Tokens</th>
                <th class="py-2 text-right">Cost (USD)</th>
              </tr>
            </thead>
            <tbody id="modelsTableBody" class="divide-y divide-slate-800 text-slate-300">
              <tr><td colspan="4" class="py-4 text-center text-slate-500">No events recorded</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Recent Events Log -->
    <div class="glass p-5 rounded-2xl space-y-3">
      <div class="flex justify-between items-center">
        <h3 class="text-sm font-semibold text-slate-200">Recent Request Stream</h3>
        <span class="text-xs text-slate-500">Last 50 events</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs whitespace-nowrap">
          <thead class="text-slate-400 border-b border-slate-700/60">
            <tr>
              <th class="py-2.5 px-3">Time</th>
              <th class="py-2.5 px-3">App</th>
              <th class="py-2.5 px-3">Provider</th>
              <th class="py-2.5 px-3">Model</th>
              <th class="py-2.5 px-3">Prompt</th>
              <th class="py-2.5 px-3">Compl</th>
              <th class="py-2.5 px-3">Cache R/W</th>
              <th class="py-2.5 px-3">Latency</th>
              <th class="py-2.5 px-3 text-right">Cost</th>
            </tr>
          </thead>
          <tbody id="eventsTableBody" class="divide-y divide-slate-800 text-slate-300 font-mono text-[11px]">
            <tr><td colspan="9" class="py-4 text-center text-slate-500 font-sans">No activity recorded yet</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <script>
    let dailyChart = null;
    let modelPieChart = null;

    function formatNumber(num) {
      return new Intl.NumberFormat().format(num || 0);
    }

    function formatCurrency(num) {
      return '$' + (num || 0).toFixed(4);
    }

    async function fetchData() {
      const filter = document.getElementById('timeFilter').value;
      let since = '';
      const now = new Date();
      if (filter === 'today') {
        since = now.toISOString().slice(0, 10) + 'T00:00:00';
      } else if (filter === '7d') {
        const d = new Date(now.getTime() - 7 * 86400000);
        since = d.toISOString();
      } else if (filter === '30d') {
        const d = new Date(now.getTime() - 30 * 86400000);
        since = d.toISOString();
      }

      try {
        const url = since ? `/api/stats?since=${encodeURIComponent(since)}` : '/api/stats';
        const res = await fetch(url);
        const data = await res.json();
        renderDashboard(data);
      } catch (err) {
        console.error('Failed to load stats:', err);
      }
    }

    function renderDashboard(data) {
      const summary = data.summary || {};
      document.getElementById('kpi-cost').innerText = formatCurrency(summary.total_cost_usd);
      document.getElementById('kpi-tokens').innerText = formatNumber(summary.total_tokens);
      document.getElementById('kpi-cache').innerText = formatNumber(summary.total_cache_read_tokens);
      document.getElementById('kpi-requests').innerText = formatNumber(summary.total_requests);
      document.getElementById('kpi-latency').innerText = `avg latency: ${(summary.avg_latency_ms || 0).toFixed(0)}ms`;

      // Render Clients Table
      const clientsBody = document.getElementById('clientsTableBody');
      if (data.by_client && data.by_client.length > 0) {
        clientsBody.innerHTML = data.by_client.map(c => `
          <tr class="hover:bg-slate-800/40">
            <td class="py-2 font-medium text-slate-100">${c.client_app}</td>
            <td class="py-2">${formatNumber(c.requests)}</td>
            <td class="py-2">${formatNumber(c.total_tokens)}</td>
            <td class="py-2 text-right font-mono text-cyan-400">${formatCurrency(c.cost_usd)}</td>
          </tr>
        `).join('');
      } else {
        clientsBody.innerHTML = '<tr><td colspan="4" class="py-4 text-center text-slate-500">No client data</td></tr>';
      }

      // Render Models Table
      const modelsBody = document.getElementById('modelsTableBody');
      if (data.by_model && data.by_model.length > 0) {
        modelsBody.innerHTML = data.by_model.map(m => `
          <tr class="hover:bg-slate-800/40">
            <td class="py-2 font-medium text-slate-100">${m.model}</td>
            <td class="py-2">${formatNumber(m.requests)}</td>
            <td class="py-2">${formatNumber(m.total_tokens)}</td>
            <td class="py-2 text-right font-mono text-cyan-400">${formatCurrency(m.cost_usd)}</td>
          </tr>
        `).join('');
      } else {
        modelsBody.innerHTML = '<tr><td colspan="4" class="py-4 text-center text-slate-500">No model data</td></tr>';
      }

      // Render Events Table
      const eventsBody = document.getElementById('eventsTableBody');
      if (data.recent_events && data.recent_events.length > 0) {
        eventsBody.innerHTML = data.recent_events.map(e => `
          <tr class="hover:bg-slate-800/40">
            <td class="py-2 px-3 text-slate-400">${e.timestamp.slice(11, 19)}</td>
            <td class="py-2 px-3"><span class="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-slate-300">${e.client_app}</span></td>
            <td class="py-2 px-3">${e.provider}</td>
            <td class="py-2 px-3 text-indigo-300 font-semibold">${e.model}</td>
            <td class="py-2 px-3">${formatNumber(e.prompt_tokens)}</td>
            <td class="py-2 px-3">${formatNumber(e.completion_tokens)}</td>
            <td class="py-2 px-3 text-emerald-400">${formatNumber(e.cache_read_tokens)} / ${formatNumber(e.cache_write_tokens)}</td>
            <td class="py-2 px-3 text-slate-400">${e.latency_ms.toFixed(0)}ms</td>
            <td class="py-2 px-3 text-right text-cyan-400">${formatCurrency(e.cost_usd)}</td>
          </tr>
        `).join('');
      } else {
        eventsBody.innerHTML = '<tr><td colspan="9" class="py-4 text-center text-slate-500 font-sans">No recent events</td></tr>';
      }

      // Render Daily Chart
      renderDailyChart(data.daily || []);
      renderModelPieChart(data.by_model || []);
    }

    function renderDailyChart(dailyData) {
      const ctx = document.getElementById('dailyChart').getContext('2d');
      const labels = dailyData.map(d => d.day);
      const tokens = dailyData.map(d => d.total_tokens);
      const costs = dailyData.map(d => d.cost_usd);

      if (dailyChart) dailyChart.destroy();

      dailyChart = new Chart(ctx, {
        type: 'bar',
        data: {
          labels: labels.length ? labels : ['Today'],
          datasets: [
            {
              label: 'Tokens',
              data: tokens.length ? tokens : [0],
              backgroundColor: 'rgba(99, 102, 241, 0.6)',
              borderColor: 'rgb(99, 102, 241)',
              borderWidth: 1,
              yAxisID: 'y',
            },
            {
              label: 'Cost (USD)',
              data: costs.length ? costs : [0],
              type: 'line',
              borderColor: 'rgb(56, 189, 248)',
              backgroundColor: 'rgba(56, 189, 248, 0.2)',
              yAxisID: 'y1',
              tension: 0.3,
            }
          ]
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          scales: {
            x: { grid: { color: 'rgba(255, 255, 255, 0.05)' }, ticks: { color: '#94a3b8' } },
            y: { position: 'left', grid: { color: 'rgba(255, 255, 255, 0.05)' }, ticks: { color: '#818cf8' } },
            y1: { position: 'right', grid: { drawOnChartArea: false }, ticks: { color: '#38bdf8' } }
          },
          plugins: {
            legend: { labels: { color: '#cbd5e1' } }
          }
        }
      });
    }

    function renderModelPieChart(modelData) {
      const ctx = document.getElementById('modelPieChart').getContext('2d');
      if (modelPieChart) modelPieChart.destroy();

      const labels = modelData.map(m => m.model);
      const costs = modelData.map(m => m.cost_usd);

      modelPieChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
          labels: labels.length ? labels : ['None'],
          datasets: [{
            data: costs.length ? costs : [1],
            backgroundColor: [
              '#38bdf8', '#818cf8', '#a855f7', '#ec4899', '#f43f5e', '#10b981', '#f59e0b'
            ],
            borderWidth: 0
          }]
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { position: 'bottom', labels: { color: '#94a3b8', boxWidth: 12 } }
          }
        }
      });
    }

    // Auto-refresh every 10 seconds
    fetchData();
    setInterval(fetchData, 10000);
  </script>
</body>
</html>
"""


def register_dashboard_routes(app: FastAPI, db: Database, guard: Guard | None = None) -> None:
    # Serve the built React dashboard (frontend/ → pnpm build → src/odotoken/static)
    static_dir = Path(__file__).parent / "static"
    if (static_dir / "index.html").exists():
        app.mount("/app", StaticFiles(directory=str(static_dir), html=True), name="dashboard")

        @app.get("/dashboard", response_class=HTMLResponse)
        async def dashboard_view():
            return FileResponse(str(static_dir / "index.html"))

        @app.get("/", response_class=HTMLResponse)
        async def root_view():
            return FileResponse(str(static_dir / "index.html"))

    @app.get("/api/config")
    def config_api():
        """Exposed subset of config.json for dashboard."""
        import json as _jc
        from pathlib import Path as _Pc
        cfg = {}
        for cand in [_Pc("config.json"), _Pc(__file__).parent.parent.parent.parent / "config.json"]:
            if cand.exists():
                try:
                    cfg = _jc.loads(cand.read_text())
                    break
                except Exception:
                    pass
        providers = sorted((cfg.get("providers") or {}).keys())
        sessions = cfg.get("sessions") or {}
        guard_cfg = cfg.get("limits") or {}
        return JSONResponse({"providers": providers, "sessions": sessions, "limits": guard_cfg})

    @app.get("/api/stats")
    async def stats_api(since: str | None = None, project: str | None = None, source: str | None = None, group_by: str | None = None):
        summary = db.get_summary(since=since, project=project, source=source)
        by_model = db.get_summary_by_model(since=since, project=project, source=source)
        by_client = db.get_summary_by_client(since=since, project=project, source=source)
        by_project = db.get_summary_by_project(since=since, source=source)
        by_source = db.get_summary_by_source(since=since, project=project)
        recent_events = db.get_recent_events(limit=50, since=since, project=project, source=source)
        daily_grouped = None
        if group_by == "source":
            daily_grouped = db.get_daily_usage_grouped(days=30, since=since)
        daily = daily_grouped if daily_grouped is not None else db.get_daily_usage(days=30, since=since, source=source)
        guard_info = {}
        if guard:
            check = guard.check_allowance()
            guard_info = {
                "limits": {
                    "daily_cost_limit_usd": guard.limits.daily_cost_limit_usd,
                    "monthly_cost_limit_usd": guard.limits.monthly_cost_limit_usd,
                    "daily_tokens_limit": guard.limits.daily_tokens_limit,
                    "monthly_tokens_limit": guard.limits.monthly_tokens_limit,
                    "action_on_breach": guard.limits.action_on_breach,
                },
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
        # Drop noise rows (no model name and no usage: favicon 404s, failed
        # connections) so dashboard tables stay clean.
        by_model = [m for m in by_model if m.get("model") or m.get("total_tokens")]
        by_client = [c for c in by_client if c.get("total_tokens") or c.get("requests")]

        savings = {
            "actual_usd": summary.get("total_cost_usd", 0.0),
            "shadow_usd": summary.get("total_shadow_cost_usd", 0.0),
            "saved_usd": max(0.0, summary.get("total_shadow_cost_usd", 0.0) - summary.get("total_cost_usd", 0.0)),
        }

        return JSONResponse(
            {
                "summary": summary,
                "by_model": by_model,
                "by_client": by_client,
                "by_project": by_project,
                "savings": savings,
                "recent_events": recent_events,
                "daily": daily,
                "by_source": by_source,
                "guard": guard_info,
            }
        )
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
        cfg["limits"] = limits
        cfg_path.write_text(_jg.dumps(cfg, indent=2) + "\n")
        if guard:
            from odotoken.guard import GuardLimits
            guard.update_limits(GuardLimits(
                daily_cost_limit_usd=limits.get("daily_cost_limit_usd"),
                monthly_cost_limit_usd=limits.get("monthly_cost_limit_usd"),
                daily_tokens_limit=limits.get("daily_tokens_limit"),
                monthly_tokens_limit=limits.get("monthly_tokens_limit"),
                action_on_breach=limits.get("action_on_breach") or "alert",
                webhook_url=limits.get("webhook_url"),
            ))
        # webhook on manual breach check
        if guard and limits.get("webhook_url"):
            try:
                import httpx
                check = guard.check_allowance()
                if check.status in ("warning","breached"):
                    httpx.post(limits["webhook_url"], json={"status": check.status, "reason": check.reason, "daily_cost": check.current_daily_cost}, timeout=3)
            except Exception:
                pass
        return JSONResponse({"limits": limits})

    @app.get("/api/stats.csv")
    async def stats_csv(since: str | None = None, project: str | None = None, source: str | None = None):
        import csv
        import io
        rows = db.get_recent_events(limit=5000, since=since, project=project, source=source)
        buf = io.StringIO()
        w = csv.DictWriter(buf, fieldnames=["id","timestamp","provider","model","client_app","prompt_tokens","completion_tokens","total_tokens","cost_usd","latency_ms","source"])
        w.writeheader()
        for r in rows:
            w.writerow({k: r.get(k,"") for k in ["id","timestamp","provider","model","client_app","prompt_tokens","completion_tokens","total_tokens","cost_usd","latency_ms","source"]})
        from fastapi.responses import PlainTextResponse
        return PlainTextResponse(buf.getvalue(), media_type="text/csv", headers={"Content-Disposition": "attachment; filename=odotoken.csv"})


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

    # Move catch-all wildcard routes to the end so /dashboard and /api/stats are matched first
    app.router.routes.sort(key=lambda r: 1 if "{" in getattr(r, "path", "") else 0)
