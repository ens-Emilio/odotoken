"""Command Line Interface for odotoken.
Inspect usage, manage server daemon, check limits.
"""

from __future__ import annotations

import json
import os
from pathlib import Path

import typer
from rich.console import Console
from rich.table import Table

from odotoken.api.app import create_app
from odotoken.cli.commands.count import get_count_app
from odotoken.cli.commands.models import get_models_app
from odotoken.db import Database
from odotoken.guard import Guard, GuardLimits
from odotoken.infra.config import get_token_counter_config
from odotoken.proxy import create_proxy_app
from odotoken.pricing import PricingCalculator

app = typer.Typer(
    name="odotoken",
    help="odotoken: Transparent AI token metering proxy, usage tracker and budget guard.",
    no_args_is_help=True,
)
console = Console()

app.add_typer(get_count_app(), name="count")
app.add_typer(get_models_app(), name="models")


DEFAULT_CONFIG_PATH = Path("config.json")
DEFAULT_DB_PATH = Path("odotoken.db")


def load_config_file(path: Path = DEFAULT_CONFIG_PATH) -> dict:
    if path.exists():
        try:
            with open(path, "r", encoding="utf-8") as f:
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
    pricing = PricingCalculator(cfg.get("prices", "prices.json"))

    limits_cfg = cfg.get("limits", {})
    guard_limits = GuardLimits(
        daily_cost_limit_usd=limits_cfg.get("daily_budget_usd"),
        monthly_cost_limit_usd=limits_cfg.get("monthly_budget_usd"),
        daily_tokens_limit=limits_cfg.get("daily_tokens_limit"),
        monthly_tokens_limit=limits_cfg.get("monthly_tokens_limit"),
        action_on_breach=limits_cfg.get("action_on_breach", "alert"),
    )
    guard = Guard(database, guard_limits)

    proxy_app = create_proxy_app(database, pricing, guard, cfg)
    # Dashboard routes would be registered here if needed

    console.print(f"[green]Starting odotoken proxy on {actual_host}:{actual_port}[/green]")
    uvicorn.run(proxy_app, host=actual_host, port=actual_port, workers=workers, reload=reload, log_level="info")


@app.command()
def serve(
    host: str = typer.Option("127.0.0.1", "--host", "-h", help="Host to bind"),
    port: int = typer.Option(8081, "--port", "-p", help="Port to bind"),
    config_path: str = typer.Option("config.json", "--config", "-c", help="Config file path"),
    reload: bool = typer.Option(False, "--reload", help="Enable auto-reload"),
):
    """Start the token counter API server."""
    import uvicorn

    config_file = Path(config_path)
    if not config_file.exists():
        console.print(f"[bold red]Config file not found: {config_file}[/bold red]")
        raise typer.Exit(1)

    cfg = load_config_file(config_file)
    tc_config = get_token_counter_config(cfg)

    if not tc_config.enabled:
        console.print("[yellow]Token counter is disabled in config. Enable it to use this command.[/yellow]")
        raise typer.Exit(1)

    console.print(f"[green]Starting token counter API on {host}:{port}[/green]")

    api_app = create_app(config_path)
    uvicorn.run(api_app, host=host, port=port, reload=reload, log_level="info")


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
    """Check service status and port connectivity."""
    import socket

    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    port = cfg.get("proxy", {}).get("port", 8080)

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


@app.command()
def stats(
    since: str = typer.Option(None, "--since", help="ISO date filter (e.g., 2024-01-01)"),
    by_model: bool = typer.Option(False, "--by-model", help="Group by model"),
    by_client: bool = typer.Option(False, "--by-client", help="Group by client app"),
    days: int = typer.Option(30, "--days", help="Daily usage history days"),
):
    """Display comprehensive token usage and cost metrics in the terminal."""
    cfg = load_config_file(DEFAULT_CONFIG_PATH)
    db_path = cfg.get("database", {}).get("path", "odotoken.db")
    database = Database(db_path)
    pricing = PricingCalculator(cfg.get("prices", "prices.json"))

    if by_model:
        results = database.get_summary_by_model(since=since)
        table = Table(title="Token Usage by Model")
        table.add_column("Model", style="cyan")
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

    if by_client:
        results = database.get_summary_by_client(since=since)
        table = Table(title="Token Usage by Client")
        table.add_column("Client", style="cyan")
        table.add_column("Requests", justify="right")
        table.add_column("Total Tokens", justify="right")
        table.add_column("Cost (USD)", justify="right")
        for row in results:
            table.add_row(row["client_app"], str(row["request_count"]), f"{row['total_tokens']:,}", f"${row['total_cost_usd']:.6f}")
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
                day_table.add_row(row["date"], str(row["request_count"]), f"{row['total_tokens']:,}", f"${row['total_cost_usd']:.6f}")
            console.print(day_table)


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