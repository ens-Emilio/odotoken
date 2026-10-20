"""Import service: iter usages -> pricing -> db."""

from __future__ import annotations

import json
from dataclasses import dataclass

from odotoken.db import Database, TokenEvent
from odotoken.importers.registry import get_importers
from odotoken.pricing import PricingCalculator


@dataclass
class ImportStats:
    scanned: int = 0
    inserted: int = 0
    skipped_dup: int = 0
    skipped_since: int = 0
    cost_usd: float = 0.0
    errors: int = 0
    by_cli: dict | None = None


def import_sessions(
    db: Database,
    pricing: PricingCalculator,
    config: dict | None = None,
    clis: list[str] | None = None,
    since: str | None = None,
    dry_run: bool = False,
) -> ImportStats:
    stats = ImportStats(by_cli={})
    importers = get_importers(config)
    if clis:
        wanted = set(clis)
        importers = [i for i in importers if i.name in wanted]
    BATCH = 2000
    for imp in importers:
        if not imp.detect():
            continue
        cli_scanned = 0
        cli_inserted = 0
        cli_skipped = 0
        cli_cost = 0.0
        batch: list[TokenEvent] = []
        batch_costs: list[float] = []
        for ru in imp.iter_usages(since=since):
            cli_scanned += 1
            stats.scanned += 1
            try:
                cost = pricing.calculate_cost(
                    ru.model, ru.prompt_tokens, ru.completion_tokens,
                    ru.cache_read_tokens, ru.cache_write_tokens,
                )
            except Exception:
                cost = 0.0
                stats.errors += 1
            if cost == 0 and ru.raw_cost not in (None, 0, 0.0):
                try:
                    cost = float(ru.raw_cost)
                except Exception:
                    pass
            if dry_run:
                cli_cost += cost
                continue
            meta = {"raw_cost": ru.raw_cost} if ru.raw_cost is not None else {}
            event = TokenEvent(
                timestamp=ru.timestamp or "",
                provider=ru.provider,
                model=ru.model,
                client_app=ru.cli,
                prompt_tokens=ru.prompt_tokens,
                completion_tokens=ru.completion_tokens,
                total_tokens=ru.prompt_tokens + ru.completion_tokens,
                cache_read_tokens=ru.cache_read_tokens,
                cache_write_tokens=ru.cache_write_tokens,
                cost_usd=cost,
                project_id=ru.project or "default",
                metadata=json.dumps(meta),
                source=f"import:{ru.cli}",
                session_id=ru.session_id,
                session_file=ru.session_file,
                entry_id=ru.entry_id,
            )
            batch.append(event)
            batch_costs.append(cost)
            if len(batch) >= BATCH:
                ins, skip = db.record_events_ignore_dup(batch)
                # exact inserted cost: we don't know which rows skipped, approximate by average
                # for first full import skip==0 so exact; for dedup rerun ins==0.
                b_cost = sum(batch_costs)
                inserted_cost = b_cost if skip == 0 else (b_cost * ins / len(batch) if batch else 0)
                cli_inserted += ins
                cli_skipped += skip
                stats.inserted += ins
                stats.skipped_dup += skip
                stats.cost_usd += inserted_cost
                cli_cost += inserted_cost
                batch.clear()
                batch_costs.clear()
        if batch:
            b_cost = sum(batch_costs)
            ins, skip = db.record_events_ignore_dup(batch)
            inserted_cost = b_cost if skip == 0 else (b_cost * ins / len(batch) if batch else 0)
            cli_inserted += ins
            cli_skipped += skip
            stats.inserted += ins
            stats.skipped_dup += skip
            stats.cost_usd += inserted_cost
            cli_cost += inserted_cost
        if stats.by_cli is not None:
            stats.by_cli[imp.name] = {
                "scanned": cli_scanned,
                "inserted": cli_inserted,
                "skipped_dup": cli_skipped,
                "cost_usd": round(cli_cost, 7),
            }
        if dry_run and cli_scanned:
            stats.cost_usd += cli_cost
    return stats
