"""Health and metrics API routes."""

from __future__ import annotations

from datetime import UTC, datetime

from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel

from odotoken.infra.logging import get_metrics
from odotoken.services.token_counter import TokenCounter

router = APIRouter(tags=["health"])


class HealthResponse(BaseModel):
    status: str = "ok"
    version: str = "0.2.0"
    timestamp: str


class ReadinessResponse(BaseModel):
    status: str
    checks: dict[str, bool]


class MetricsResponse(BaseModel):
    counters: dict[str, int]
    histograms: dict[str, dict]
    gauges: dict[str, float]


def get_token_counter(request: Request) -> TokenCounter:
    return request.app.state.token_counter


@router.get("/health", response_model=HealthResponse)
async def health_check() -> HealthResponse:
    """Liveness probe - service is running."""
    return HealthResponse(
        timestamp=datetime.now(UTC).isoformat(),
    )


@router.get("/ready", response_model=ReadinessResponse)
async def readiness_check(
    token_counter: TokenCounter = Depends(get_token_counter),
) -> ReadinessResponse:
    """Readiness probe - service can handle requests."""

    checks = {
        "catalog_loaded": len(token_counter.catalog.list_models()) > 0,
        "cache_initialized": True,
        "heuristic_adapter": any(a.name == "heuristic" for a in token_counter._adapters),
    }

    # Check if any remote adapters are configured (not required for readiness)
    remote_adapters = [a for a in token_counter._adapters if a.name in ("openai", "anthropic", "gemini")]
    if remote_adapters:
        checks["remote_adapters"] = True

    all_ok = all(checks.values())
    return ReadinessResponse(
        status="ready" if all_ok else "not_ready",
        checks=checks,
    )


@router.get("/metrics", response_model=MetricsResponse)
async def metrics_endpoint() -> MetricsResponse:
    """Prometheus-style metrics endpoint."""
    metrics = get_metrics()
    return MetricsResponse(**metrics.get_metrics())
