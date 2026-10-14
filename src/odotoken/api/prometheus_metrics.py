"""Prometheus text-format metrics endpoint for the proxy.

Metrics are derived from SQLite aggregates (all-time window) rather than
in-memory counters: they survive restarts and need no extra state. With the
single-worker architecture this is both simple and exact.
"""

from __future__ import annotations

from typing import Any

from fastapi import Request
from fastapi.responses import PlainTextResponse

from odotoken.db import Database
from odotoken.guard import Guard

CONTENT_TYPE = "text/plain; version=0.0.4; charset=utf-8"


def _escape_label(value: Any) -> str:
    return str(value).replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")


def render_prometheus(db: Database, guard: Guard | None = None) -> str:
    lines: list[str] = []

    summary = db.get_summary()
    lines += [
        "# HELP odotoken_requests_total Total proxied requests.",
        "# TYPE odotoken_requests_total counter",
        f"odotoken_requests_total {summary.get('total_requests', 0)}",
        "# HELP odotoken_tokens_total Total tokens proxied.",
        "# TYPE odotoken_tokens_total counter",
        f'odotoken_tokens_total{{direction="prompt"}} {summary.get("total_prompt_tokens", 0)}',
        f'odotoken_tokens_total{{direction="completion"}} {summary.get("total_completion_tokens", 0)}',
        f'odotoken_tokens_total{{direction="cache_read"}} {summary.get("total_cache_read_tokens", 0)}',
        "# HELP odotoken_cost_dollars_total Cumulative cost in USD (actual and shadow).",
        "# TYPE odotoken_cost_dollars_total counter",
        f'odotoken_cost_dollars_total{{kind="actual"}} {summary.get("total_cost_usd", 0.0):.6f}',
        f'odotoken_cost_dollars_total{{kind="shadow"}} {summary.get("total_shadow_cost_usd", 0.0):.6f}',
        "# HELP odotoken_request_latency_ms_avg Average request latency in milliseconds.",
        "# TYPE odotoken_request_latency_ms_avg gauge",
        f"odotoken_request_latency_ms_avg {summary.get('avg_latency_ms', 0.0):.2f}",
    ]

    # Per-model series
    lines += [
        "# HELP odotoken_model_requests_total Requests per model/provider.",
        "# TYPE odotoken_model_requests_total counter",
    ]
    for row in db.get_summary_by_model():
        m = _escape_label(row.get("model") or "unknown")
        p = _escape_label(row.get("provider") or "unknown")
        lines.append(f'odotoken_model_requests_total{{model="{m}",provider="{p}"}} {row["requests"]}')
        lines.append(f'odotoken_model_tokens_total{{model="{m}",direction="total"}} {row["total_tokens"]}')

    # Per-project series
    lines += [
        "# HELP odotoken_project_cost_dollars_total Cost per project tag.",
        "# TYPE odotoken_project_cost_dollars_total counter",
    ]
    for row in db.get_summary_by_project():
        proj = _escape_label(row["project_id"])
        lines.append(f'odotoken_project_cost_dollars_total{{project="{proj}"}} {row["cost_usd"]:.6f}')

    # Guard gauges
    if guard:
        check = guard.check_allowance()
        limits = guard.limits
        pct_remaining = 100.0
        if limits.monthly_cost_limit_usd:
            used = check.current_monthly_cost
            pct_remaining = max(0.0, 100.0 * (1 - used / limits.monthly_cost_limit_usd))
        lines += [
            "# HELP odotoken_budget_remaining_percentage Remaining monthly budget percentage.",
            "# TYPE odotoken_budget_remaining_percentage gauge",
            f"odotoken_budget_remaining_percentage {pct_remaining:.2f}",
        ]

    return "\n".join(lines) + "\n"


def register_prometheus_route(app: Any, db: Database, guard: Guard | None = None) -> None:
    @app.get("/metrics", response_class=PlainTextResponse)
    async def prometheus_metrics(request: Request) -> PlainTextResponse:
        return PlainTextResponse(render_prometheus(db, guard), media_type=CONTENT_TYPE)
