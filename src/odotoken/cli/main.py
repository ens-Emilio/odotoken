"""Command Line Interface for odotoken.
Inspect usage, manage server daemon, check limits.
"""

from __future__ import annotations

import json
import os
import re
from pathlib import Path

import typer
from rich.console import Console
from rich.table import Table

from odotoken.cli.commands.count import get_count_app
from odotoken.cli.commands.models import get_models_app
from odotoken.cli.commands.sessions import get_sessions_app
from odotoken.dashboard import register_dashboard_routes
from odotoken.db import Database
from odotoken.guard import Guard, GuardLimits
from odotoken.infra.config import get_token_counter_config
from odotoken.pricing import PricingCalculator
from odotoken.proxy import create_proxy_app

app = typer.Typer(
    name="odotoken",
    help="odotoken: Transparent AI token metering proxy, usage tracker and budget guard.",
    no_args_is_help=True,
)
console = Console()

app.add_typer(get_count_app(), name="count")
app.add_typer(get_models_app(), name="models")
app.add_typer(get_sessions_app(), name="sessions")


DEFAULT_CONFIG_PATH = Path("config.json")
DEFAULT_DB_PATH = Path("odotoken.db")


def load_config_file(path: Path = DEFAULT_CONFIG_PATH) -> dict:
    if path.exists():
        try:
            with open(path, encoding="utf-8") as f:
                return json.load(f)
        except Exception as e:
            console.print(f"[yellow]Warning: Failed to load config from {path}: {e}[/yellow]")
    return {}


@app.command()
def start(
    host: str = typer.Option(None, "--host", "-h", help="Host to bind"),
    port: int = typer.Option(None, "--port", "-p", help="Port to bind"),
    config_path: str = typer.Option("config.json", "--config", "-c", help="Config file path"),
    workers: int = typer.Option(1, "--workers", "-w", help="Number of worker processes"),
    reload: bool = typer.Option(False, "--reload", help="Enable auto-reload"),
):
    """Start the odotoken proxy and dashboard service."""
    import uvicorn

    cfg = load_config_file(Path(config_path))

    proxy_cfg = cfg.get("proxy", {})
    actual_host = host or proxy_cfg.get("host", "127.0.0.1")
    actual_port = port or proxy_cfg.get("port", 8080)

    db_path = cfg.get("database", {}).get("path", "odotoken.db")
    database = Database(db_path)

    limits_cfg = cfg.get("limits", {})
    guard_limits = GuardLimits.from_dict(limits_cfg)
    guard = Guard(database, guard_limits)

    proxy_app = create_proxy_app(database, PricingCalculator(cfg.get("prices", "prices.json")), guard, cfg)
    register_dashboard_routes(proxy_app, database, guard)

    # Prometheus text-format metrics (replaces the JSON /metrics when the
    # token counter is off; served on the proxy port)
    from odotoken.api.prometheus_metrics import register_prometheus_route

    register_prometheus_route(proxy_app, database, guard)
    console.print("[green]Prometheus: http://127.0.0.1:8090/metrics[/green]")

    # Token counting routes (/v1/count, /v1/count/batch, /v1/models) live on
    # the same app when token_counter is enabled — no separate service needed.
    if get_token_counter_config(cfg).enabled:
        from odotoken.api.app import attach_token_counter_routes

        attach_token_counter_routes(proxy_app, cfg)
        console.print("[green]Token counter enabled: /v1/count, /v1/models[/green]")

    console.print(f"[green]Starting odotoken proxy on {actual_host}:{actual_port}[/green]")
    console.print(f"[green]Dashboard: http://{actual_host}:{actual_port}/dashboard[/green]")
    uvicorn.run(
        proxy_app, host=actual_host, port=actual_port, workers=workers, reload=reload, log_level="info"
    )


@app.command()
def serve(
    host: str = typer.Option("127.0.0.1", "--host", "-h", help="Host to bind"),
    port: int = typer.Option(8081, "--port", "-p", help="Port to bind"),
    config_path: str = typer.Option("config.json", "--config", "-c", help="Config file path"),
    reload: bool = typer.Option(False, "--reload", help="Enable auto-reload"),
):
    """Start the proxy (kept as an alias of `start` — token counting routes
    live on the same app under /v1/count when token_counter.enabled is true)."""
    start(host=host, port=port, config_path=config_path, reload=reload)


@app.command()
def stop():
    """Stop the running background daemon."""
    import signal

    pid_file = Path("/tmp/odotoken.pid")
    if not pid_file.exists():
        console.print("[yellow]No PID file found. Is odotoken running?[/yellow]")
        return

    try:
        pid = int(pid_file.read_text().strip())
        os.kill(pid, signal.SIGTERM)
        pid_file.unlink()
        console.print(f"[green]Stopped odotoken daemon (PID: {pid})[/green]")
    except Exception as e:
        console.print(f"[bold red]Failed to stop process: {e}[/bold red]")


@app.command()
def status():
    """Check service status, systemd unit sanity and port connectivity."""
    import socket

    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    port = cfg.get("proxy", {}).get("port", 8080)

    # systemd unit sanity: does ExecStart point at an existing binary?
    unit_path = Path.home() / ".config/systemd/user/odotoken.service"
    if unit_path.exists():
        exec_match = re.search(r"ExecStart=(\S+)", unit_path.read_text(encoding="utf-8"))
        if exec_match:
            exec_bin = exec_match.group(1)
            if not Path(exec_bin).exists():
                console.print(
                    f"[bold red]unit ExecStart points to {exec_bin}, which does not exist.[/bold red]"
                )
                console.print("run: [green]odotoken install-service[/green] from the project directory")
            else:
                console.print(f"[green]unit ok: {exec_bin}[/green]")

    try:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(1)
            result = s.connect_ex(("127.0.0.1", port))
            if result == 0:
                console.print(f"[green]odotoken daemon is running on port {port}[/green]")
            else:
                console.print(f"[yellow]odotoken daemon is NOT running.[/yellow] (Port configured: {port})")
    except Exception as e:
        console.print(f"[red]Error checking status: {e}[/red]")


@app.command("install-service")
def install_service(
    port: int = typer.Option(None, "--port", "-p", help="Port to bind (from config.json if omitted)"),
):
    """Write ~/.config/systemd/user/odotoken.service with absolute paths from this project directory."""
    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    actual_port = port or cfg.get("proxy", {}).get("port", 8090)

    project_dir = Path.cwd()
    bin_path = project_dir / ".venv/bin/odotoken"
    if not bin_path.exists():
        console.print(f"[red]no odotoken binary at {bin_path}[/red]")
        console.print(
            "run [green]python -m venv .venv && .venv/bin/pip install -e .[/green]"
            " first, from the project root"
        )
        raise typer.Exit(1)

    unit = f"""[Unit]
Description=odotoken - AI Token Proxy and Cost Metering Service
After=network.target

[Service]
Type=simple
WorkingDirectory={project_dir}
ExecStart={bin_path} start --host 127.0.0.1 --port {actual_port}
Restart=always
RestartSec=3
Environment="PYTHONUNBUFFERED=1"

[Install]
WantedBy=default.target
"""
    unit_dir = Path.home() / ".config/systemd/user"
    unit_dir.mkdir(parents=True, exist_ok=True)
    unit_path = unit_dir / "odotoken.service"
    unit_path.write_text(unit, encoding="utf-8")
    console.print(f"[green]wrote {unit_path}[/green]")
    console.print(f"  WorkingDirectory={project_dir}")
    console.print(f"  ExecStart={bin_path} start --host 127.0.0.1 --port {actual_port}")
    console.print(
        "now run: [green]systemctl --user daemon-reload && systemctl --user enable --now odotoken[/green]"
    )


@app.command()
def stats(
    since: str = typer.Option(None, "--since", help="ISO date filter (e.g., 2024-01-01)"),
    by_model: bool = typer.Option(False, "--by-model", help="Group by model"),
    by_client: bool = typer.Option(False, "--by-client", help="Group by client app"),
    by_project: bool = typer.Option(False, "--by-project", help="Group by project tag"),
    tps: bool = typer.Option(False, "--tps", help="Show throughput (TPS) stats"),
    days: int = typer.Option(30, "--days", help="Daily usage history days"),
):
    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    db_path = cfg.get("database", {}).get("path", "odotoken.db")
    database = Database(db_path)

    if tps:
        summary = database.get_tps_summary(since=since)
        def _fmt(v: float) -> str:
            return f"{v:.1f} tok/s" if v > 0 else "—"
        table = Table(title="Throughput (TPS)")
        table.add_column("Metric", style="cyan")
        table.add_column("Value", style="green")
        table.add_row("Avg TPS", _fmt(summary.get("avg_tps", 0)))
        table.add_row("Peak TPS", _fmt(summary.get("peak_tps", 0)))
        table.add_row("Avg latency", f"{summary.get('avg_latency_ms', 0):.0f}ms")
        table.add_row("Requests", str(summary.get("requests", 0)))
        table.add_row("Completion tokens", f"{summary.get('completion_tokens', 0):,}")
        console.print(table)
        by_model = database.get_tps_by_model(since=since)
        if by_model:
            mt = Table(title="TPS by Model")
            mt.add_column("Model", style="cyan")
            mt.add_column("TPS", justify="right", style="green")
            mt.add_column("Avg ms", justify="right")
            mt.add_column("Reqs", justify="right")
            for r in by_model[:10]:
                mt.add_row(r["model"], _fmt(r.get("tps", 0)), f"{r.get('avg_latency_ms', 0):.0f}", str(r["requests"]))  # noqa: E501
            console.print(mt)
        return

    if by_model:
        results = database.get_summary_by_model(since=since)
        table = Table(title="Token Usage by Model")
        table.add_column("Requests", justify="right", style="green")
        table.add_column("Prompt Tokens", justify="right", style="yellow")
        table.add_column("Completion Tokens", justify="right", style="yellow")
        table.add_column("Total Tokens", justify="right", style="green")
        table.add_column("Cost (USD)", justify="right", style="red")
        for row in results:
            table.add_row(
                row["model"],
                str(row["request_count"]),
                f"{row['prompt_tokens']:,}",
                f"{row['completion_tokens']:,}",
                f"{row['total_tokens']:,}",
                f"${row['total_cost_usd']:.6f}",
            )
        console.print(table)
        return

    if by_project:
        results = database.get_summary_by_project(since=since)
        table = Table(title="Token Usage by Project")
        table.add_column("Project", style="cyan")
        table.add_column("Requests", justify="right")
        table.add_column("Total Tokens", justify="right")
        table.add_column("Cost (USD)", justify="right")
        table.add_column("Shadow (USD)", justify="right", style="magenta")
        table.add_column("Saved (USD)", justify="right", style="green")
        for row in results:
            saved = max(0.0, row["shadow_cost_usd"] - row["cost_usd"])
            table.add_row(
                row["project_id"],
                str(row["requests"]),
                f"{row['total_tokens']:,}",
                f"${row['cost_usd']:.6f}",
                f"${row['shadow_cost_usd']:.6f}",
                f"${saved:.6f}",
            )
        console.print(table)
        return

    if by_client:
        results = database.get_summary_by_client(since=since)
        table = Table(title="Token Usage by Client")
        table.add_column("Client", style="cyan")
        table.add_column("Requests", justify="right")
        table.add_column("Total Tokens", justify="right")
        table.add_column("Cost (USD)", justify="right")
        for row in results:
            table.add_row(
                row["client_app"],
                str(row["request_count"]),
                f"{row['total_tokens']:,}",
                f"${row['total_cost_usd']:.6f}",
            )
        console.print(table)
        return

    summary = database.get_summary(since=since)
    table = Table(title="Overall Token Usage")
    table.add_column("Metric", style="cyan")
    table.add_column("Value", style="green")
    table.add_row("Total Requests", str(summary.get("request_count", 0)))
    table.add_row("Prompt Tokens", f"{summary.get('prompt_tokens', 0):,}")
    table.add_row("Completion Tokens", f"{summary.get('completion_tokens', 0):,}")
    table.add_row("Total Tokens", f"{summary.get('total_tokens', 0):,}")
    table.add_row("Total Cost (USD)", f"${summary.get('total_cost_usd', 0.0):.6f}")
    shadow = summary.get("total_shadow_cost_usd", 0.0)
    actual = summary.get("total_cost_usd", 0.0)
    if shadow > actual:
        table.add_row("Saved vs Cloud (USD)", f"${shadow - actual:.6f}")
    console.print(table)

    if days > 0:
        daily = database.get_daily_usage(days=days)
        if daily:
            day_table = Table(title=f"Daily Usage (Last {days} Days)")
            day_table.add_column("Date", style="cyan")
            day_table.add_column("Requests", justify="right")
            day_table.add_column("Tokens", justify="right")
            day_table.add_column("Cost (USD)", justify="right")
            for row in daily:
                day_table.add_row(
                    row.get("day") or row.get("date") or "",
                    str(row.get("requests") or row.get("request_count") or 0),
                    f"{(row.get('total_tokens') or 0):,}",
                    f"${(row.get('cost_usd') or row.get('total_cost_usd') or 0):.6f}",
                )
            console.print(day_table)

@app.command()
def replay(
    since: str = typer.Option(None, "--since", help="ISO date filter"),
    project: str = typer.Option(None, "--project", help="Filter by project_id"),
    dry_run: bool = typer.Option(False, "--dry-run", help="Only print diff, do not update DB"),
    apply: bool = typer.Option(False, "--apply", help="Write recalculated costs back to DB"),
):
    """Recalculate costs with current prices.json and show shadow_cost diff."""
    from odotoken.services.model_catalog import ModelCatalog
    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    db_path = cfg.get("database", {}).get("path", "odotoken.db")
    database = Database(db_path)
    price_path = cfg.get("prices", "prices.json") if isinstance(cfg.get("prices"), str) else "prices.json"
    pricing = PricingCalculator(price_path)
    catalog = ModelCatalog()
    events = database.get_recent_events(limit=100000, since=since, project=project)
    if not events:
        console.print("[yellow]No events found.[/yellow]")
        return
    diffs = []
    total_old_cost = total_new_cost = total_old_shadow = total_new_shadow = 0.0
    changed = 0
    for e in events:
        old_cost = float(e.get("cost_usd") or 0)
        old_shadow = float(e.get("shadow_cost") or 0)
        new_cost = pricing.calculate_cost(
            e.get("model") or "",
            int(e.get("prompt_tokens") or 0),
            int(e.get("completion_tokens") or 0),
            int(e.get("cache_read_tokens") or 0),
            int(e.get("cache_write_tokens") or 0),
        )
        new_shadow = pricing.calculate_shadow_cost(
            e.get("model") or "",
            cloud_equivalent=catalog.get_cloud_equivalent(e.get("model") or ""),
            prompt_tokens=int(e.get("prompt_tokens") or 0),
            completion_tokens=int(e.get("completion_tokens") or 0),
            cache_read_tokens=int(e.get("cache_read_tokens") or 0),
            cache_write_tokens=int(e.get("cache_write_tokens") or 0),
        )
        total_old_cost += old_cost
        total_new_cost += new_cost
        total_old_shadow += old_shadow
        total_new_shadow += new_shadow
        if abs(new_cost - old_cost) > 1e-9 or abs(new_shadow - old_shadow) > 1e-9:
            changed += 1
            diffs.append((e, old_cost, new_cost, old_shadow, new_shadow))
    console.print(f"[bold]Events: {len(events)}  Changed: {changed}[/bold]")
    console.print(
        f"  cost   old=${total_old_cost:.6f}  new=${total_new_cost:.6f}  "
        f"diff=${total_new_cost - total_old_cost:+.6f}"
    )
    console.print(
        f"  shadow old=${total_old_shadow:.6f}  new=${total_new_shadow:.6f}  "
        f"diff=${total_new_shadow - total_old_shadow:+.6f}"
    )
    if diffs:
        table = Table(title="Changed rows (first 20)")
        table.add_column("id", justify="right")
        table.add_column("model")
        table.add_column("old cost", justify="right")
        table.add_column("new cost", justify="right")
        table.add_column("old shadow", justify="right")
        table.add_column("new shadow", justify="right")
        for e, oc, nc, os_, ns in diffs[:20]:
            table.add_row(
                str(e.get("id", "")),
                e.get("model", "")[:30],
                f"{oc:.6f}",
                f"{nc:.6f}",
                f"{os_:.6f}",
                f"{ns:.6f}",
            )
        console.print(table)
        if len(diffs) > 20:
            console.print(f"[dim]... and {len(diffs)-20} more[/dim]")
    should_apply = apply and not dry_run
    if should_apply and diffs:
        with database.connection() as conn:
            for e, _oc, _nc, _os_, _ns in diffs:
                conn.execute(
                    "UPDATE token_events SET cost_usd=?, shadow_cost=? WHERE id=?",
                    (_nc, _ns, e["id"]),
                )
            conn.commit()
        console.print(f"[green]Updated {changed} rows.[/green]")
    elif not dry_run and not apply and diffs:
        console.print("[yellow]Dry-run: no DB changes. Use --apply to write back.[/yellow]")

@app.command()
def optimize(
    since: str = typer.Option(None, "--since", help="ISO date filter (e.g., 2024-01-01)"),
    project: str = typer.Option(None, "--project", help="Filter by project_id"),
):
    """Analyze usage, suggest cheaper model equivalents and estimated savings."""
    from odotoken.services.model_catalog import ModelCatalog

    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    db_path = cfg.get("database", {}).get("path", "odotoken.db")
    database = Database(db_path)
    pricing = PricingCalculator(cfg.get("prices", "prices.json") if isinstance(cfg.get("prices"), str) else "prices.json")
    catalog = ModelCatalog()

    rows = database.get_summary_by_model(since=since, project=project)
    # filter out empty model key and zero-cost? keep for display but sort by cost desc already
    rows = [r for r in rows if r.get("model")]
    if not rows or sum(float(r.get("cost_usd") or 0) for r in rows) == 0:
        if not rows:
            console.print("[yellow]No usage data found.[/yellow]")
        else:
            console.print("[green]already optimal — no billable usage to optimize.[/green]")
        return

    top = sorted(rows, key=lambda r: float(r.get("cost_usd") or 0), reverse=True)[:3]

    # fallback generic cheap model (cheapest cloud model in pricing)
    generic = "gpt-4o-mini"

    suggestions: list[dict] = []
    for r in top:
        model_id: str = r["model"]
        cost = float(r.get("cost_usd") or 0)
        if cost <= 0:
            continue
        prompt = int(r.get("prompt_tokens") or 0)
        completion = int(r.get("completion_tokens") or 0)
        cache_r = int(r.get("cache_read_tokens") or 0)
        cache_w = int(r.get("cache_write_tokens") or 0)
        hints = []
        ratio = (completion / prompt) if prompt else 0
        if ratio > 4:
            hints.append(f"high completion:prompt ratio ({ratio:.1f}x)")
        tok_total = prompt + completion + cache_r + cache_w
        if tok_total and (cache_r / tok_total) > 0.5:
            hints.append(f"high cache-read usage ({cache_r/tok_total:.0%})")
        hint = "; ".join(hints)

        alt: str | None = None
        # 1) same-family cheapest via catalog
        alts = catalog.get_cheaper_alternatives(model_id)
        if alts:
            alt = alts[0].id
        else:
            # 2) cloud_equivalent if cheaper
            ce = catalog.get_cloud_equivalent(model_id)
            if ce and ce.lower() != model_id.lower():
                ce_cost = pricing.calculate_cost(ce, prompt, completion, cache_r, cache_w)
                if ce_cost < cost - 1e-9:
                    alt = ce
        # 3) generic gpt-4o-mini if cheaper and not self
        if alt is None and model_id.lower() != generic.lower():
            gen_cost = pricing.calculate_cost(generic, prompt, completion, cache_r, cache_w)
            if gen_cost < cost - 1e-9:
                # ensure generic exists pricing wise (it always does)
                alt = generic

        if alt is None:
            continue
        alt_cost = pricing.calculate_cost(alt, prompt, completion, cache_r, cache_w)
        saving = cost - alt_cost
        if saving <= 1e-9:
            continue
        pct = (saving / cost * 100) if cost else 0
        suggestions.append(
            {
                "model": model_id,
                "requests": r.get("requests") or r.get("request_count") or 0,
                "cost": cost,
                "alt": alt,
                "alt_cost": alt_cost,
                "saving": saving,
                "pct": pct,
                "prompt": prompt,
                "completion": completion,
                "hint": hint,
            }
        )

    if not suggestions:
        console.print("[green]already optimal — no cheaper alternatives found for your top models.[/green]")
        # still show what top models are for transparency
        t = Table(title="Top models by cost (no cheaper alternative)")
        t.add_column("Model", style="cyan")
        t.add_column("Requests", justify="right")
        t.add_column("Cost (USD)", justify="right", style="red")
        for r in top:
            t.add_row(str(r["model"]), str(r.get("requests") or r.get("request_count") or 0), f"${float(r.get('cost_usd') or 0):.6f}")
        console.print(t)
        return

    table = Table(title="Optimization suggestions (top 3 by cost)")
    table.add_column("Model", style="cyan")
    table.add_column("Reqs", justify="right")
    table.add_column("Cost", justify="right", style="red")
    table.add_column("Suggested", style="green")
    table.add_column("Est. cost", justify="right")
    table.add_column("Saving", justify="right", style="green")
    table.add_column("Save %", justify="right", style="green")
    table.add_column("Notes", justify="left", style="dim")
    total_saving = 0.0
    for s in suggestions:
        total_saving += s["saving"]
        table.add_row(
            s["model"][:32],
            str(s["requests"]),
            f"${s['cost']:.6f}",
            s["alt"],
            f"${s['alt_cost']:.6f}",
            f"${s['saving']:.6f}",
            f"{s['pct']:.1f}%",
            s.get("hint") or "",
        )
    console.print(table)
    if len(suggestions) == 1:
        s = suggestions[0]
        console.print(f"[dim]Example: replace [cyan]{s['model']}[/cyan] → [green]{s['alt']}[/green] to save ~${s['saving']:.6f} ({s['pct']:.1f}%).[/dim]")
    else:
        console.print(f"[bold green]Potential saving if all applied: ${total_saving:.6f}[/bold green]")
    # hint about since/project filters
    filt = []
    if since:
        filt.append(f"since={since}")
    if project:
        filt.append(f"project={project}")
    if filt:
        console.print(f"[dim]Filters: {', '.join(filt)}[/dim]")

@app.command()
def log(limit: int = typer.Option(20, "--limit", "-n", help="Number of recent records")):
    """View recent token consumption requests log."""
    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    db_path = cfg.get("database", {}).get("path", "odotoken.db")
    database = Database(db_path)

    events = database.get_recent_events(limit=limit)
    table = Table(title=f"Recent Token Events (Last {limit})")
    table.add_column("Time", style="dim")
    table.add_column("Provider", style="cyan")
    table.add_column("Model", style="green")
    table.add_column("Client", style="yellow")
    table.add_column("Prompt", justify="right", style="blue")
    table.add_column("Completion", justify="right", style="blue")
    table.add_column("Total", justify="right", style="green")
    table.add_column("Cost", justify="right", style="red")
    table.add_column("Latency (ms)", justify="right", style="magenta")

    for event in events:
        table.add_row(
            event["timestamp"][:19].replace("T", " "),
            event["provider"],
            event["model"],
            event["client_app"],
            f"{event['prompt_tokens']:,}",
            f"{event['completion_tokens']:,}",
            f"{event['total_tokens']:,}",
            f"${event['cost_usd']:.6f}",
            f"{event['latency_ms']:.1f}",
        )
    console.print(table)


if __name__ == "__main__":
    app()
