"""FastAPI application factory for token counting API."""

from __future__ import annotations

from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI

from odotoken.api.routes import count, health, models
from odotoken.infra.config import get_token_counter_config, load_config
from odotoken.infra.logging import setup_logging
from odotoken.services.cache import TokenCache
from odotoken.services.model_catalog import ModelCatalog
from odotoken.services.token_counter import TokenCounter


def create_app(config_path: str = "config.json") -> FastAPI:
    """Create and configure FastAPI application."""
    # Load configuration
    config = load_config(config_path)
    try:
        from odotoken.infra.otel import init_tracing

        init_tracing(config)
    except Exception:
        pass
    tc_config = get_token_counter_config(config)

    # Setup logging
    setup_logging(level="INFO")

    # Initialize services
    catalog = ModelCatalog()
    cache = TokenCache(tc_config)
    token_counter = TokenCounter(tc_config, catalog, cache)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        # Startup
        import asyncio

        await asyncio.sleep(0.1)  # Allow remote adapters to register
        yield
        # Shutdown
        for adapter in token_counter._adapters:
            if hasattr(adapter, "close"):
                await adapter.close()

    app = FastAPI(
        title="odotoken - Token Counter API",
        version="0.2.0",
        lifespan=lifespan,
    )

    # Store services in app state
    app.state.token_counter = token_counter
    app.state.config = tc_config

    # Include routers
    app.include_router(count.router)
    app.include_router(models.router)
    app.include_router(health.router)

    return app


def attach_token_counter_routes(app: FastAPI, config: dict[str, Any]) -> None:
    """Attach token-counting routes (/v1/count, /v1/models, health extras)
    onto an existing app (the proxy), sharing one process and port."""
    from odotoken.api.routes import count, health, models

    app.include_router(count.router)
    app.include_router(models.router)
    app.include_router(health.router)

    # Build the counter once and stash it in app.state for the routes' dependency
    cfg = get_token_counter_config(config)
    catalog = ModelCatalog()
    cache = TokenCache(cfg)
    app.state.token_counter = TokenCounter(cfg, catalog, cache)

    # Register the ready/metrics catch-alls last so they don't shadow /v1/*
    app.router.routes.sort(key=lambda r: 1 if "{" in getattr(r, "path", "") else 0)
