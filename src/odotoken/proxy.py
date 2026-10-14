"""FastAPI Reverse Proxy with transparent token metering and budget enforcement."""

from __future__ import annotations

import asyncio
import json
import logging
import time
from contextlib import asynccontextmanager
from typing import Any
from urllib.parse import urljoin

import httpx
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse, StreamingResponse

from odotoken.db import Database, TokenEvent
from odotoken.guard import Guard
from odotoken.parser import StreamUsageAccumulator, UsageParser
from odotoken.pricing import PricingCalculator
from odotoken.services.model_catalog import ModelCatalog

logger = logging.getLogger("odotoken.proxy")


def create_proxy_app(
    db: Database,
    pricing: PricingCalculator,
    guard: Guard,
    config: dict[str, Any],
) -> FastAPI:
    client_timeout = httpx.Timeout(connect=15.0, read=300.0, write=60.0, pool=30.0)
    http_client = httpx.AsyncClient(timeout=client_timeout, follow_redirects=True)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        yield
        await http_client.aclose()

    app = FastAPI(title="odotoken - AI Token Proxy", version="0.1.0", lifespan=lifespan)
    catalog = ModelCatalog()

    providers_config = config.get(
        "providers",
        {
            "anthropic": {"upstream_url": "https://api.anthropic.com"},
            "openai": {"upstream_url": "https://api.openai.com"},
            "gemini": {"upstream_url": "https://generativelanguage.googleapis.com"},
            "ollama": {"upstream_url": "http://localhost:11434"},
        },
    )

    def determine_upstream(path: str, headers: dict[str, str]) -> tuple[str, str]:
        """Returns (provider_name, target_base_url)."""
        # 1. Explicit path prefix routing: /anthropic/v1/..., /openai/v1/...,
        #    or any custom provider key: /<provider_name>/v1/...
        for p_name in providers_config:
            if path.startswith(f"/{p_name}/"):
                target = providers_config[p_name].get("upstream_url", "")
                return p_name, target

        # 2. Inferred from headers or standard endpoints
        inferred = UsageParser.identify_provider(path, headers)
        target = providers_config.get(inferred, {}).get("upstream_url", "https://api.openai.com")
        return inferred, target

    def prepare_forward_path(path: str, provider: str) -> str:
        """Strip provider prefix if present, e.g. /anthropic/v1/messages -> /v1/messages."""
        prefix = f"/{provider}"
        if path.startswith(prefix):
            stripped = path[len(prefix) :]
            return stripped if stripped.startswith("/") else f"/{stripped}"
        return path

    @app.api_route("/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"])
    async def proxy_handler(request: Request, path: str):
        # Allow health check
        if path == "health":
            return JSONResponse({"status": "ok", "service": "odotoken"})
        if path == "favicon.ico":
            raise HTTPException(status_code=404, detail="Not a proxy route")

        headers = {k.lower(): v for k, v in request.headers.items()}
        # Remove host header to avoid SSL/SNI conflicts with upstream
        headers.pop("host", None)
        headers.pop("content-length", None)

        # Budget Check
        check = guard.check_allowance()
        if not check.allowed:
            logger.warning("Request blocked by guard: %s", check.reason)
            return JSONResponse(
                status_code=429,
                content={
                    "error": {
                        "type": "odotoken_budget_limit_exceeded",
                        "message": f"odotoken Guard: {check.reason}",
                    }
                },
            )

        provider, upstream_base = determine_upstream(request.url.path, headers)
        client_app = UsageParser.identify_client_app(headers)
        project_id = headers.get("x-odo-project") or headers.get("x-project") or "default"
        forward_path = prepare_forward_path(request.url.path, provider)
        target_url = urljoin(upstream_base.rstrip("/") + "/", forward_path.lstrip("/"))
        if request.url.query:
            target_url = f"{target_url}?{request.url.query}"

        body = await request.body()
        fallback_model = ""
        is_stream = False
        if body:
            try:
                body_json = json.loads(body)
                if isinstance(body_json, dict):
                    fallback_model = body_json.get("model", "")
                    is_stream = bool(body_json.get("stream", False))
            except Exception:
                pass

        start_time = time.perf_counter()

        try:
            req = http_client.build_request(
                method=request.method,
                url=target_url,
                headers=headers,
                content=body,
            )
            response = await http_client.send(req, stream=True)
        except Exception as exc:
            logger.error("Failed to forward request to %s: %s", target_url, exc)
            return JSONResponse(
                status_code=502,
                content={"error": {"type": "odotoken_proxy_error", "message": str(exc)}},
            )

        resp_headers = dict(response.headers)
        # Remove hop-by-hop & compression headers that cause streaming issues
        resp_headers.pop("content-encoding", None)
        resp_headers.pop("content-length", None)
        resp_headers.pop("transfer-encoding", None)
        content_type = resp_headers.get("content-type", "")

        is_sse = "text/event-stream" in content_type or is_stream

        if is_sse:
            # Handle streaming response
            accumulator = StreamUsageAccumulator(provider=provider, default_model=fallback_model)

            async def stream_generator():
                try:
                    async for chunk in response.aiter_bytes():
                        accumulator.process_chunk(chunk)
                        yield chunk
                finally:
                    await response.aclose()
                    latency_ms = (time.perf_counter() - start_time) * 1000.0
                    usage = accumulator.finalize()
                    cost = pricing.calculate_cost(
                        model=usage.model,
                        prompt_tokens=usage.prompt_tokens,
                        completion_tokens=usage.completion_tokens,
                        cache_read_tokens=usage.cache_read_tokens,
                        cache_write_tokens=usage.cache_write_tokens,
                    )
                    shadow = pricing.calculate_shadow_cost(
                        usage.model,
                        cloud_equivalent=catalog.get_cloud_equivalent(usage.model),
                        prompt_tokens=usage.prompt_tokens,
                        completion_tokens=usage.completion_tokens,
                        cache_read_tokens=usage.cache_read_tokens,
                        cache_write_tokens=usage.cache_write_tokens,
                    )
                    await asyncio.to_thread(
                        db.record_event,
                        TokenEvent(
                            provider=provider,
                            model=usage.model,
                            client_app=client_app,
                            shadow_cost=shadow,
                            project_id=project_id,
                            prompt_tokens=usage.prompt_tokens,
                            completion_tokens=usage.completion_tokens,
                            total_tokens=usage.total_tokens,
                            cache_read_tokens=usage.cache_read_tokens,
                            cache_write_tokens=usage.cache_write_tokens,
                            cost_usd=cost,
                            latency_ms=round(latency_ms, 2),
                            status_code=response.status_code,
                        ),
                    )

            return StreamingResponse(
                stream_generator(),
                status_code=response.status_code,
                headers=resp_headers,
                media_type="text/event-stream",
            )

        else:
            # Handle standard non-streaming response
            raw_content = await response.aread()
            await response.aclose()
            latency_ms = (time.perf_counter() - start_time) * 1000.0

            model_name = fallback_model
            prompt_tokens = 0
            completion_tokens = 0
            cache_read = 0
            cache_write = 0
            res_json: dict[str, Any] = {}

            try:
                res_json = json.loads(raw_content)
                if isinstance(res_json, dict):
                    usage = UsageParser.parse_json_response(provider, res_json, fallback_model=fallback_model)
                    model_name = usage.model
                    prompt_tokens = usage.prompt_tokens
                    completion_tokens = usage.completion_tokens
                    cache_read = usage.cache_read_tokens
                    cache_write = usage.cache_write_tokens
            except Exception:
                pass

            cost = pricing.calculate_cost(
                model=model_name,
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                cache_read_tokens=cache_read,
                cache_write_tokens=cache_write,
            )
            shadow_cost = pricing.calculate_shadow_cost(
                model_name,
                cloud_equivalent=catalog.get_cloud_equivalent(model_name),
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                cache_read_tokens=cache_read,
                cache_write_tokens=cache_write,
            )

            # Don't log metering noise: 404s from the catch-all (favicon, stale
            # assets) carry no usage data and pollute the dashboard.
            has_usage = prompt_tokens or completion_tokens
            if has_usage or response.status_code not in (404, 405):
                await asyncio.to_thread(
                    db.record_event,
                    TokenEvent(
                        provider=provider,
                        model=model_name,
                        client_app=client_app,
                        shadow_cost=shadow_cost,
                        project_id=project_id,
                        prompt_tokens=prompt_tokens,
                        completion_tokens=completion_tokens,
                        total_tokens=prompt_tokens + completion_tokens,
                        cache_read_tokens=cache_read,
                        cache_write_tokens=cache_write,
                        cost_usd=cost,
                        latency_ms=round(latency_ms, 2),
                        status_code=response.status_code,
                    ),
                )
            else:
                logger.debug("skipped event logging for %s %s", response.status_code, path)

            return JSONResponse(
                content=res_json if res_json else None,
                status_code=response.status_code,
                headers=resp_headers,
            )

    return app
