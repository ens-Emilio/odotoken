"""FastAPI application factory for token counting API."""

from __future__ import annotations

from contextlib import asynccontextmanager

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
        version="0.1.0",
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