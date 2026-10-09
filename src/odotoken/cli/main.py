"""Command Line Interface for odotoken.
Inspect usage, manage server daemon, check limits.
"""

from __future__ import annotations

import typer
from rich.console import Console

from odotoken.api.app import create_app
from odotoken.cli.commands.count import get_count_app
from odotoken.cli.commands.models import get_models_app
from odotoken.db import Database
from odotoken.guard import Guard, GuardLimits
from odotoken.infra.config import get_token_counter_config, load_config
from odotoken.pricing import PricingCalculator

app = typer.Typer(
    name="odotoken",
    help="odotoken: Transparent AI token metering proxy, usage tracker and budget guard.",
    no_args_is_help=True,
)
console = Console()

app.add_typer(get_count_app(), name="count")
app.add_typer(get_models_app(), name="models")