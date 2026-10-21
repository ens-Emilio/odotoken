"""odotoken sessions — scan & import CLI usage histories."""

from __future__ import annotations

import json
from pathlib import Path

import typer
from rich.console import Console
from rich.table import Table

from odotoken.db import Database
from odotoken.importers.registry import scan_all
from odotoken.importers.service import import_sessions
from odotoken.pricing import PricingCalculator

app = typer.Typer(name="sessions", help="Import token usage from local CLI sessions.", no_args_is_help=True)
console = Console()


def _load_config(path: Path) -> dict:
    if path.exists():
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    return {}


def _resolve_db_path(config: dict) -> Path:
    return Path(config.get("database", {}).get("path", "odotoken.db"))


@app.command("scan")
def scan(
    config_path: Path = typer.Option(Path("config.json"), "--config", help="Config file"),
    since: str | None = typer.Option(None, "--since", help="ISO date filter e.g. 2026-10-01"),
    json_out: bool = typer.Option(False, "--json", help="JSON output"),
):
    """Scan local CLI session storages (read-only)."""
    config = _load_config(config_path)
    results = scan_all(config, since=since)
    if json_out:
        console.print(json.dumps([r.__dict__ for r in results], indent=2))
        return
    table = Table(title="CLI sessions scan", show_header=True)
    table.add_column("CLI")
    table.add_column("Available")
    table.add_column("Files", justify="right")
    table.add_column("Entries", justify="right")
    table.add_column("Path", overflow="fold")
    for r in results:
        table.add_row(
            r.cli, "yes" if r.available else "no", str(r.session_files), str(r.total_entries), r.path
        )
    console.print(table)


@app.command("import")
def import_cmd(
    config_path: Path = typer.Option(Path("config.json"), "--config", help="Config file"),
    cli: str | None = typer.Option(None, "--cli", help="Comma-separated CLIs: claude,codex,omp,crush"),
    since: str | None = typer.Option(None, "--since", help="ISO date filter e.g. 2026-10-01T00:00:00Z"),
    dry_run: bool = typer.Option(False, "--dry-run", help="Scan + price without writing to DB"),
    json_out: bool = typer.Option(False, "--json", help="JSON output"),
):
    """Import sessions into odotoken.db (idempotent, deduped)."""
    config = _load_config(config_path)
    db_path = _resolve_db_path(config)
    db = Database(db_path)
    pricing = PricingCalculator(
        price_file=Path("prices.json") if Path("prices.json").exists() else "prices.json"
    )
    clis = [c.strip() for c in cli.split(",")] if cli else None
    stats = import_sessions(db, pricing, config=config, clis=clis, since=since, dry_run=dry_run)
    if json_out:
        console.print(json.dumps(stats.__dict__, indent=2, default=str))
        return
    mode = "DRY-RUN" if dry_run else "IMPORT"
    table = Table(title=f"Sessions {mode}", show_header=True)
    table.add_column("Metric")
    table.add_column("Value", justify="right")
    table.add_row("Scanned", str(stats.scanned))
    table.add_row("Inserted", str(stats.inserted))
    table.add_row("Skipped dup", str(stats.skipped_dup))
    table.add_row("Cost USD", f"${stats.cost_usd:.6f}")
    if stats.by_cli:
        for k, v in stats.by_cli.items():
            table.add_row(f"  {k}", f"{v['inserted']}/{v['scanned']}  ${v['cost_usd']:.6f}")
    console.print(table)
    if dry_run:
        console.print("[dim]Dry run — no rows written.[/dim]")


@app.command("stats")
def stats(
    config_path: Path = typer.Option(Path("config.json"), "--config", help="Config file"),
    by_cli: bool = typer.Option(False, "--by-cli", help="Breakdown by source/cli"),
    since: str | None = typer.Option(None, "--since", help="ISO date filter"),
    json_out: bool = typer.Option(False, "--json", help="JSON output"),
):
    """Show stats breakdown by import source."""
    config = _load_config(config_path)
    db_path = _resolve_db_path(config)
    db = Database(db_path)
    summary = db.get_summary(since=since)
    by_source = db.get_summary_by_source(since=since)
    by_model = db.get_summary_by_model(since=since)
    if json_out:
        console.print(
            json.dumps({"summary": summary, "by_source": by_source, "by_model": by_model}, indent=2)
        )
        return
    console.print(
        f"[bold]Total:[/bold] {summary.get('total_requests', 0)} req  "
        f"{summary.get('total_tokens', 0)} tok  ${summary.get('total_cost_usd', 0.0):.6f}"
    )
    if by_cli or True:
        table = Table(title="By source", show_header=True)
        table.add_column("Source")
        table.add_column("Requests", justify="right")
        table.add_column("Tokens", justify="right")
        table.add_column("Cost", justify="right")
        for row in by_source:
            table.add_row(
                row["source"], str(row["requests"]), str(row["total_tokens"]), f"${row['cost_usd']:.6f}"
            )
        console.print(table)


def get_sessions_app() -> typer.Typer:
    return app

@app.command("watch")
def watch(
    config_path: Path = typer.Option(Path("config.json"), "--config", help="Config file"),
    interval: int = typer.Option(60, "--interval", "-i", help="Seconds between scans"),
):
    """Poll and import sessions every interval seconds (Ctrl+C to stop)."""
    import time
    config = _load_config(config_path)
    db_path = _resolve_db_path(config)
    db = Database(db_path)
    pricing = PricingCalculator(
        price_file=Path("prices.json") if Path("prices.json").exists() else "prices.json"
    )
    console.print(f"[dim]Watching every {interval}s — db={db_path}[/dim]")
    while True:
        stats = import_sessions(db, pricing, config=config)
        console.print(
            f"[green]import[/green] scanned={stats.scanned} inserted={stats.inserted} "
            f"skipped={stats.skipped_dup} cost=${stats.cost_usd:.4f}"
        )
        try:
            time.sleep(interval)
        except KeyboardInterrupt:
            break
