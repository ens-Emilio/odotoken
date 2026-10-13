"""CLI commands for token counting."""

from __future__ import annotations

import asyncio
import json
import sys
from pathlib import Path

import typer
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.table import Table

from odotoken.domain.entities import CountRequest
from odotoken.infra.config import get_token_counter_config, load_config
from odotoken.services.cache import TokenCache
from odotoken.services.model_catalog import ModelCatalog
from odotoken.services.token_counter import TokenCounter

console = Console()
count_app = typer.Typer(name="count", help="Count tokens for text or chat messages")


async def _get_token_counter() -> TokenCounter:
    """Initialize token counter service."""
    config = load_config()
    tc_config = get_token_counter_config(config)
    catalog = ModelCatalog()
    cache = TokenCache(tc_config)
    counter = TokenCounter(tc_config, catalog, cache)
    await asyncio.sleep(0.1)  # Allow remote adapters to register
    return counter


@count_app.command("text")
def count_text(
    text: str = typer.Argument(..., help="Text to count tokens for"),
    model: str = typer.Option("gpt-4o-mini", "--model", "-m", help="Model identifier"),
    provider: str | None = typer.Option(None, "--provider", "-p", help="Override provider"),
    output: str = typer.Option("table", "--output", "-o", help="Output format: table, json"),
    file: str | None = typer.Option(None, "--file", "-f", help="Read text from file"),
) -> None:
    """Count tokens for plain text."""

    async def _run():
        counter = await _get_token_counter()

        content = text
        if file:
            content = Path(file).read_text(encoding="utf-8")

        request = CountRequest(
            model=model,
            text=content,
            provider=provider,
        )

        result = await counter.count(request)

        if output == "json":
            console.print_json(data=result.__dict__)
        else:
            table = Table(title=f"Token Count for Model: {model}")
            table.add_column("Property", style="cyan")
            table.add_column("Value", style="green")

            table.add_row("Model", f"{result.model} -> {result.resolved_model}")
            table.add_column("Provider", result.provider)
            table.add_column("Method", result.method)
            table.add_column("Tokenizer", result.tokenizer_used)
            table.add_row("Input Tokens", str(result.input_tokens))
            table.add_row("Output Tokens", str(result.output_tokens))
            table.add_row("Total Tokens", str(result.total_tokens))
            table.add_row("Cache Hit", "YES" if result.cache_hit else "NO")
            table.add_row("Duration", f"{result.duration_ms:.2f} ms")

            if result.context_window:
                util = result.context_utilization or 0
                table.add_row("Context Window", str(result.context_window))
                table.add_row("Context Utilization", f"{util:.2%}")

            if result.warnings:
                table.add_row("Warnings", ", ".join(result.warnings))

            console.print(table)

    asyncio.run(_run())


@count_app.command("chat")
def count_chat(
    model: str = typer.Option("gpt-4o-mini", "--model", "-m", help="Model identifier"),
    provider: str | None = typer.Option(None, "--provider", "-p", help="Override provider"),
    output: str = typer.Option("table", "--output", "-o", help="Output format: table, json"),
    file: Path = typer.Option(Path("-"), "--file", "-f", help="JSON file with messages (or stdin)"),
    tools: bool = typer.Option(False, "--tools", help="Include empty tools array"),
) -> None:
    """Count tokens for chat messages."""

    async def _run():
        counter = await _get_token_counter()

        # Read messages from file or stdin
        if str(file) == "-":
            if not sys.stdin.isatty():
                messages = json.load(sys.stdin)
            else:
                console.print("[red]Error: No input provided on stdin[/red]")
                raise typer.Exit(1)
        else:
            messages = json.loads(Path(file).read_text(encoding="utf-8"))

        request = CountRequest(
            model=model,
            messages=messages,
            provider=provider,
            tools=[] if tools else None,
        )

        result = await counter.count(request)

        if output == "json":
            console.print_json(data=result.__dict__)
        else:
            table = Table(title=f"Token Count for Chat: {model}")
            table.add_column("Property", style="cyan")
            table.add_column("Value", style="green")

            table.add_row("Model", f"{result.model} -> {result.resolved_model}")
            table.add_column("Provider", result.provider)
            table.add_column("Method", result.method)
            table.add_column("Tokenizer", result.tokenizer_used)
            table.add_row("Input Tokens", str(result.input_tokens))
            table.add_row("Output Tokens", str(result.output_tokens))
            table.add_row("Total Tokens", str(result.total_tokens))
            table.add_row("Cache Hit", "YES" if result.cache_hit else "NO")
            table.add_row("Duration", f"{result.duration_ms:.2f} ms")

            if result.context_window:
                util = result.context_utilization or 0
                table.add_row("Context Window", str(result.context_window))
                table.add_row("Context Utilization", f"{util:.2%}")

            if result.tokens_per_message:
                msg_table = Table(title="Tokens per Message")
                msg_table.add_column("Index", style="dim")
                msg_table.add_column("Tokens", style="green")
                for i, tokens in enumerate(result.tokens_per_message):
                    msg_table.add_row(str(i), str(tokens))
                console.print(msg_table)

            if result.warnings:
                table.add_row("Warnings", ", ".join(result.warnings))

            console.print(table)

    asyncio.run(_run())


@count_app.command("batch")
def count_batch(
    file: Path = typer.Argument(..., help="JSONL file with requests (one per line)"),
    output: str = typer.Option("table", "--output", "-o", help="Output format: table, json"),
    max_items: int = typer.Option(100, "--max-items", help="Maximum batch size"),
) -> None:
    """Count tokens for batch of requests from JSONL file."""

    async def _run():
        counter = await _get_token_counter()

        requests = []
        with open(file, encoding="utf-8") as f:
            for i, line in enumerate(f):
                if i >= max_items:
                    console.print(f"[yellow]Warning: Truncating to {max_items} items[/yellow]")
                    break
                if line.strip():
                    data = json.loads(line)
                    if "text" in data:
                        requests.append(
                            CountRequest(
                                model=data.get("model", "gpt-4o-mini"),
                                text=data["text"],
                                provider=data.get("provider"),
                            )
                        )
                    elif "messages" in data:
                        requests.append(
                            CountRequest(
                                model=data.get("model", "gpt-4o-mini"),
                                messages=data["messages"],
                                provider=data.get("provider"),
                                tools=data.get("tools"),
                            )
                        )

        if not requests:
            console.print("[red]Error: No valid requests found[/red]")
            raise typer.Exit(1)

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            transient=True,
        ) as progress:
            progress.add_task(description="Processing batch...", total=None)
            batch_result = await counter.count_batch(requests)

        if output == "json":
            console.print_json(data=batch_result.__dict__)
        else:
            table = Table(title="Batch Token Count Results")
            table.add_column("Metric", style="cyan")
            table.add_column("Value", style="green")

            table.add_row("Total Items", str(batch_result.total_items))
            table.add_row("Successful", str(batch_result.successful_items))
            table.add_row("Failed", str(batch_result.failed_items))
            table.add_row("Total Tokens", str(batch_result.total_tokens))
            table.add_row("Duration", f"{batch_result.duration_ms:.2f} ms")

            console.print(table)

            if batch_result.results:
                results_table = Table(title="Results")
                results_table.add_column("Index", style="dim")
                results_table.add_column("Model", style="cyan")
                results_table.add_column("Tokens", style="green")
                results_table.add_column("Method", style="yellow")

                for i, result in enumerate(batch_result.results):
                    results_table.add_row(str(i), result.model, str(result.total_tokens), result.method)
                console.print(results_table)

            if batch_result.errors:
                errors_table = Table(title="Errors")
                errors_table.add_column("Index", style="dim")
                errors_table.add_column("Model", style="cyan")
                errors_table.add_column("Error", style="red")

                for i, error in enumerate(batch_result.errors):
                    errors_table.add_row(str(i), error.get("model", "unknown"), error.get("error", "unknown"))
                console.print(errors_table)

    asyncio.run(_run())


def get_count_app() -> typer.Typer:
    return count_app
