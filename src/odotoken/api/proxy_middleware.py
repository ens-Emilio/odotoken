"""Pre-flight token counting middleware for proxy integration."""

from __future__ import annotations

import asyncio
import json

from fastapi import Request, Response
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

from odotoken.domain.entities import CountRequest
from odotoken.infra.config import get_token_counter_config, load_config
from odotoken.infra.logging import get_logger
from odotoken.services.cache import TokenCache
from odotoken.services.model_catalog import ModelCatalog
from odotoken.services.token_counter import TokenCounter

logger = get_logger(__name__)


class PreflightTokenCounterMiddleware(BaseHTTPMiddleware):
    """Middleware to count tokens before forwarding to upstream providers."""

    def __init__(
        self,
        app,
        config_path: str = "config.json",
        enabled: bool = True,
        block_on_limit: bool = False,
    ):
        super().__init__(app)
        self.config_path = config_path
        self.enabled = enabled
        self.block_on_limit = block_on_limit
        self._preflight_token_limit: int | None = None
        self._token_counter: TokenCounter | None = None
        self._init_token_counter()

    def _init_token_counter(self) -> None:
        """Initialize token counter service."""
        try:
            config = load_config(self.config_path)
            tc_config = get_token_counter_config(config)
            if not tc_config.enabled:
                self.enabled = False
                return

            catalog = ModelCatalog()
            cache = TokenCache(tc_config)
            self._token_counter = TokenCounter(tc_config, catalog, cache)
            self._preflight_token_limit = config.get("preflight_token_limit")
            logger.info("Pre-flight token counter initialized")
        except Exception:
            logger.warning("Failed to initialize pre-flight token counter", exc_info=True)
            self.enabled = False

    async def _count_preflight(self, request: Request, body: bytes) -> CountRequest | None:
        """Create CountRequest from proxy request body."""
        try:
            if not body:
                return None

            body_json = json.loads(body)
            if not isinstance(body_json, dict):
                return None

            model = body_json.get("model", "")
            if not model:
                return None

            messages = body_json.get("messages")
            tools = body_json.get("tools")
            prompt = body_json.get("prompt")
            text = body_json.get("input")

            return CountRequest(
                model=model,
                text=prompt or text,
                messages=messages,
                tools=tools,
            )
        except Exception:
            logger.debug("Failed to parse request for pre-flight counting", exc_info=True)
            return None

    async def dispatch(self, request: Request, call_next) -> Response:
        if not self.enabled or not self._token_counter:
            return await call_next(request)

        # Skip non-proxy routes
        path = request.url.path
        if path in ("/health", "/favicon.ico", "/dashboard", "/api/stats", "/metrics", "/ready"):
            return await call_next(request)

        # Read body
        body = await request.body()
        if not body:
            return await call_next(request)

        # Create pre-flight count request
        count_request = await self._count_preflight(request, body)
        if not count_request:
            return await call_next(request)

        # Count tokens CONCURRENTLY with the upstream forward so the
        # pre-flight estimate never adds latency to the request path.
        async def _count():
            try:
                result = await self._token_counter.count(count_request)
                request.state.preflight_tokens = result.total_tokens
                request.state.preflight_model = result.resolved_model
                request.state.preflight_method = result.method
            except Exception:
                logger.warning("Pre-flight token counting failed", exc_info=True)

        count_task = asyncio.create_task(_count())

        if self.block_on_limit and self._preflight_token_limit:
            # Preventive guard: reject requests whose estimated size would
            # exceed the per-request allowance before spending upstream.
            try:
                await count_task
                tokens = getattr(request.state, "preflight_tokens", 0)
                if tokens > self._preflight_token_limit:
                    return JSONResponse(
                        status_code=429,
                        content={
                            "error": {
                                "type": "odotoken_preflight_limit",
                                "message": (
                                    f"Estimated {tokens} tokens exceeds per-request "
                                    f"limit {self._preflight_token_limit}"
                                ),
                            }
                        },
                    )
            except Exception:
                logger.warning("Pre-flight guard check failed", exc_info=True)
                count_task = asyncio.create_task(_count())  # re-arm for the tail await

        try:
            response = await call_next(request)
        finally:
            await count_task

        # Add pre-flight headers to response
        if hasattr(request.state, "preflight_tokens"):
            response.headers["X-Preflight-Tokens"] = str(request.state.preflight_tokens)
            response.headers["X-Preflight-Model"] = request.state.preflight_model
            response.headers["X-Preflight-Method"] = request.state.preflight_method

        return response


def create_preflight_middleware(
    config_path: str = "config.json",
    enabled: bool = True,
    block_on_limit: bool = False,
):
    """Factory to create preflight middleware with config."""

    def middleware(app):
        return PreflightTokenCounterMiddleware(
            app,
            config_path=config_path,
            enabled=enabled,
            block_on_limit=block_on_limit,
        )

    return middleware
