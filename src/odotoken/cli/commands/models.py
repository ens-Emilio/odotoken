"""CLI commands for model listing."""

from __future__ import annotations

import asyncio

import typer
from rich.console import Console
from rich.table import Table

from odotoken.domain.errors import ModelNotSupportedError
from odotoken.infra.config import get_token_counter_config, load_config
from odotoken.services.cache import TokenCache
from odotoken.services.model_catalog import ModelCatalog
from odotoken.services.token_counter import TokenCounter

console = Console()
models_app = typer.Typer(name="models", help="List and manage models")


async def _get_token_counter() -> TokenCounter:
    """Initialize token counter service."""
    config = load_config()
    tc_config = get_token_counter_config(config)
    catalog = ModelCatalog()
    cache = TokenCache(tc_config)
    counter = TokenCounter(tc_config, catalog, cache)
    await asyncio.sleep(0.1)
    return counter


@models_app.command("list")
def list_models(
    provider: str | None = typer.Option(None, "--provider", "-p", help="Filter by provider"),
    output: str = typer.Option("table", "--output", "-o", help="Output format: table, json"),
) -> None:
    """List all supported models."""

    async def _run():
        counter = await _get_token_counter()

        models = counter.list_models()

        if provider:
            models = [m for m in models if m.provider.lower() == provider.lower()]

        if output == "json":
            data = [m.__dict__ for m in models]
            console.print_json(data=data)
        else:
            table = Table(title=f"Supported Models ({len(models)} total)")
            table.add_column("ID", style="cyan")
            table.add_column("Provider", style="green")
            table.add_column("Context", style="yellow", justify="right")
            table.add_column("Max Output", style="yellow", justify="right")
            table.add_column("Method", style="magenta")
            table.add_column("Features", style="blue")

            for m in models:
                features = []
                if m.supports_messages:
                    features.append("chat")
                if m.supports_tools:
                    features.append("tools")
                if m.supports_vision:
                    features.append("vision")
                features_str = ", ".join(features)

                table.add_row(
                    m.id,
                    m.provider,
                    f"{m.context_window:,}" if m.context_window else "N/A",
                    f"{m.max_output:,}" if m.max_output else "N/A",
                    m.counting_method,
                    features_str,
                )

            console.print(table)

    asyncio.run(_run())


@models_app.command("info")
def model_info(
    model_id: str = typer.Argument(..., help="Model identifier"),
) -> None:
    """Get detailed information about a specific model."""

    async def _run():
        counter = await _get_token_counter()

        try:
            m = counter.get_model_info(model_id)
        except ModelNotSupportedError as e:
            console.print(f"[red]Model not found: {model_id}[/red]")
            raise typer.Exit(1) from e

        table = Table(title=f"Model Info: {model_id}")
        table.add_column("Property", style="cyan")
        table.add_column("Value", style="green")

        table.add_row("ID", m.id)
        table.add_row("Provider", m.provider)
        table.add_row("Family", m.family)
        table.add_row("Context Window", str(m.context_window) if m.context_window else "N/A")
        table.add_row("Max Output", str(m.max_output) if m.max_output else "N/A")
        table.add_row("Counting Method", m.counting_method)
        table.add_row("Tokenizer", m.tokenizer or "N/A")
        table.add_row("Supports Messages", "YES" if m.supports_messages else "NO")
        table.add_row("Supports Tools", "YES" if m.supports_tools else "NO")
        table.add_row("Supports Vision", "YES" if m.supports_vision else "NO")
        table.add_row("Chat Overhead", str(m.chat_overhead))
        table.add_row("Message Overhead", str(m.message_overhead))
        table.add_row("Tool Overhead", str(m.tool_overhead))
        table.add_row("Aliases", ", ".join(m.aliases) if m.aliases else "None")
        table.add_row("Notes", m.notes)

        console.print(table)

    asyncio.run(_run())


def get_models_app() -> typer.Typer:
    return models_app
