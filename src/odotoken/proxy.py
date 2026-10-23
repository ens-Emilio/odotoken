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

from odotoken.api.chaos import ChaosMiddleware, is_mock_upstream, synthesize_response
from odotoken.db import Database, TokenEvent
from odotoken.guard import Guard
from odotoken.infra.redact import build_redactor
from odotoken.parser import StreamUsageAccumulator, UsageParser
from odotoken.pricing import PricingCalculator
from odotoken.services.model_catalog import ModelCatalog

logger = logging.getLogger("odotoken.proxy")

# otel helper — lazy, never crashes if deps missing
def _otel_span(name, attrs=None):
    try:
        from odotoken.infra.otel import span as _span
        return _span(name, attrs)
    except Exception:
        import contextlib
        return contextlib.nullcontext(None)


def _semantic_cfg(config: dict[str, Any]) -> tuple[bool, int]:
    c = config.get("cache", {}).get("semantic", {}) if isinstance(config.get("cache"), dict) else {}
    enabled = bool(c.get("enabled", False))
    ttl = int(c.get("ttl_seconds", 3600) or 3600)
    return enabled, ttl


def _semantic_key(provider: str, model: str, body: bytes) -> str | None:
    import hashlib as _hl
    try:
        j = json.loads(body) if body else {}
        messages = j.get("messages", "") if isinstance(j, dict) else ""
        payload = f"{provider}|{model}|{json.dumps(messages, sort_keys=True, ensure_ascii=False)}"
    except Exception:
        payload = f"{provider}|{model}|{body.decode(errors='ignore')[:4000]}"
    return _hl.sha256(payload.encode()).hexdigest()[:32]


def _make_rate_limiter(config: dict[str, Any]):
    rl = config.get("limits", {}).get("rate_limit", {}) if isinstance(config.get("limits"), dict) else {}
    return __import__("odotoken.infra.rate_limit", fromlist=["TokenBucket"]).TokenBucket(
        requests_per_min=int(rl.get("requests_per_min", 0) or 0),
        tokens_per_min=int(rl.get("tokens_per_min", 0) or 0),
    )

def create_proxy_app(
    db: Database,
    pricing: PricingCalculator,
    guard: Guard,
    config: dict[str, Any],
) -> FastAPI:
    # OTel tracing — graceful no-op when not configured
    try:
        from odotoken.infra.otel import init_tracing

        init_tracing(config)
    except Exception:
        pass
    client_timeout = httpx.Timeout(connect=15.0, read=300.0, write=60.0, pool=30.0)
    http_client = httpx.AsyncClient(timeout=client_timeout, follow_redirects=True)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        yield
        await http_client.aclose()

    app = FastAPI(title="odotoken - AI Token Proxy", version="0.2.0", lifespan=lifespan)
    if config.get("chaos", {}).get("enabled", False):
        app.add_middleware(ChaosMiddleware, chaos_cfg=config.get("chaos", {}))
        logger.info("Chaos middleware enabled (rate=%s)", config["chaos"].get("rate"))

    catalog = ModelCatalog()
    privacy_cfg = config.get("privacy", {})
    redactor = build_redactor(config)

    providers_config = config.get(
        "providers",
        {
            "anthropic": {"upstream_url": "https://api.anthropic.com"},
            "openai": {"upstream_url": "https://api.openai.com"},
            "gemini": {"upstream_url": "https://generativelanguage.googleapis.com"},
            "ollama": {"upstream_url": "http://localhost:11434"},
        },
    )

    # ponytail: naive failover + round-robin — per-provider parity not checked,
    # not lock-free across workers, one retry per hop. Upgrade to health-checked
    # weighted balancing when one endpoint saturates.
    _rr_idx: dict[str, int] = {}


    def _primary_url(p_name: str) -> str:
        cfg = providers_config.get(p_name, {})
        urls = cfg.get("upstream_urls")
        if isinstance(urls, list) and urls:
            i = _rr_idx.get(p_name, 0)
            url = urls[i % len(urls)]
            _rr_idx[p_name] = (i + 1) % len(urls)
            return url
        return cfg.get("upstream_url", "")

    def _fallback_urls(p_name: str) -> list[str]:
        cfg = providers_config.get(p_name, {})
        out: list[str] = []
        fu = cfg.get("fallback_url")
        if isinstance(fu, str) and fu:
            out.append(fu)
        fbs = cfg.get("fallbacks")
        if isinstance(fbs, list):
            out.extend([s for s in fbs if isinstance(s, str) and s])
        # upstream_urls already handled via round-robin for primary; not duplicated as fallback
        return out

    def determine_upstream(path: str, headers: dict[str, str]) -> tuple[str, str]:
        """Returns (provider_name, target_base_url)."""
        # 1. Explicit path prefix routing: /anthropic/v1/..., /openai/v1/...,
        #    or any custom provider key: /<provider_name>/v1/...
        for p_name in providers_config:
            if path.startswith(f"/{p_name}/"):
                return p_name, _primary_url(p_name)

        # 2. Inferred from headers or standard endpoints
        inferred = UsageParser.identify_provider(path, headers)
        target = _primary_url(inferred) or providers_config.get(inferred, {}).get("upstream_url", "https://api.openai.com")
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
        if path == "health":
            return JSONResponse({"status": "ok", "service": "odotoken"})
        if path == "favicon.ico":
            raise HTTPException(status_code=404, detail="Not a proxy route")

        with _otel_span("odotoken.proxy.request", {"http.method": request.method, "http.route": path}):
            headers = {k.lower(): v for k, v in request.headers.items()}
            headers.pop("host", None)
            headers.pop("content-length", None)
            project_id = headers.get("x-odo-project") or headers.get("x-project") or "default"
            raw_tags = headers.get("x-odo-tags") or headers.get("x-tags") or ""
            tags = ",".join(s.strip() for s in raw_tags.split(",") if s.strip())

            # budget check — alert on warning/breached (all channels), never crash proxy
            check = guard.check_allowance(project_id=project_id)
            if check.status != "ok":
                try:
                    from odotoken.guard import dispatch_alerts
                    dispatch_alerts(guard.limits, check)
                    if project_id and project_id != "default":
                        pl = guard.limits.projects.get(project_id)
                        if pl and (pl.webhook_url or pl.alerts):
                            pr = guard.check_project(project_id)
                            if pr.status != "ok":
                                dispatch_alerts(pl, pr)
                except Exception:
                    pass
            if not check.allowed:
                logger.warning("Request blocked by guard: %s", check.reason)
                return JSONResponse(status_code=429, content={"error": {"type": "odotoken_budget_limit_exceeded", "message": f"odotoken Guard: {check.reason}"}})
            rl = getattr(app.state, "rate_limiter", None)
            if rl is None:
                rl = _make_rate_limiter(config)
                app.state.rate_limiter = rl
            if rl.enabled and not rl.consume(project_id):
                return JSONResponse(status_code=429, content={"error": {"type": "rate_limit_exceeded", "message": f"Rate limit exceeded for project {project_id}"}}, headers={"Retry-After": str(int(rl.retry_after(project_id)) + 1)})
            provider, upstream_base = determine_upstream(request.url.path, headers)
            client_app = UsageParser.identify_client_app(headers)
            forward_path = prepare_forward_path(request.url.path, provider)
            target_url = urljoin(upstream_base.rstrip("/") + "/", forward_path.lstrip("/"))
            if request.url.query:
                target_url = f"{target_url}?{request.url.query}"

            body = await request.body()
            if body and privacy_cfg.get("block_mode"):
                _text = body.decode(errors="ignore")
                if _text and redactor.redact(_text) != _text:
                    return JSONResponse(status_code=400, content={"error": {"type": "pii_detected", "message": "Request blocked: PII detected in prompt"}})
            prompt_meta: dict[str, Any] = {"incognito": True}
            if body and privacy_cfg.get("store_prompts", False):
                import hashlib
                try:
                    body_preview = json.loads(body).get("messages", "")
                    preview_src = json.dumps(body_preview)[:200]
                except Exception:
                    preview_src = ""
                prompt_meta = {
                    "incognito": False,
                    "prompt_sha256": hashlib.sha256(body).hexdigest()[:16],
                    "prompt_preview": redactor.redact(preview_src) if preview_src else "",
                }
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

            # Semantic cache: exact prompt hash, no upstream
            sem_enabled, sem_ttl = _semantic_cfg(config)
            sem_key: str | None = None
            if sem_enabled and body and not is_stream and request.method in ("POST", "PUT"):
                sem_key = _semantic_key(provider, fallback_model, body)
                if sem_key:
                    cached = await asyncio.to_thread(db.semantic_cache_get, sem_key, sem_ttl)
                    if cached is not None:
                        # Record cache hit as zero-cost proxy event (optional visibility)
                        # Return cached response directly
                        resp = cached if isinstance(cached, dict) else {}
                        # ensure header signals cache hit
                        return JSONResponse(content=resp, status_code=200, headers={"X-Odotoken-Cache": "HIT", "X-Cache": "HIT"})

            start_time = time.perf_counter()

            # Mock upstream: synthesize a deterministic response, no network.
            if is_mock_upstream(upstream_base):
                try:
                    prompt_text = " ".join(
                        m.get("content", "") if isinstance(m.get("content"), str) else ""
                        for m in json.loads(body).get("messages", [])
                    )
                except Exception:
                    prompt_text = ""
                mock = synthesize_response(fallback_model or "mock-model", prompt_text)
                usage = mock["usage"]
                mock_cost = pricing.calculate_cost(
                    fallback_model or "mock-model",
                    usage["prompt_tokens"],
                    usage["completion_tokens"],
                )
                with _otel_span("odotoken.db.write", {"provider": "mock"}):
                    await asyncio.to_thread(
                        db.record_event,
                        TokenEvent(
                            provider="mock",
                        model=fallback_model or "mock-model",
                        client_app=client_app,
                        prompt_tokens=usage["prompt_tokens"],
                        completion_tokens=usage["completion_tokens"],
                        total_tokens=usage["total_tokens"],
                        cost_usd=mock_cost,
                        shadow_cost=mock_cost,
                        project_id=project_id,
                        tags=tags,
                        latency_ms=0.0,
                        status_code=200,
                        metadata=json.dumps(prompt_meta),
                    ),
                )
                if sem_key:
                    try:
                        await asyncio.to_thread(db.semantic_cache_set, sem_key, mock)
                    except Exception:
                        pass
                return JSONResponse(content=mock, status_code=200)
            # OTel: forward span (no-op if not enabled)
            with _otel_span("odotoken.proxy.forward", {"provider": provider, "http.url": target_url}):
                fallbacks = _fallback_urls(provider)
                attempt_bases = [upstream_base] + fallbacks
                response = None  # type: ignore[assignment]
                used_fallback_url: str | None = None
                last_exc: Exception | None = None
                for _idx, _base in enumerate(attempt_bases):
                    if is_mock_upstream(_base):
                        # mock handled via synthesizer above; shouldn't reach here for fallback
                        upstream_base = _base
                        break
                    _target = urljoin(_base.rstrip("/") + "/", forward_path.lstrip("/"))
                    if request.url.query:
                        _target = f"{_target}?{request.url.query}"
                    try:
                        _req = http_client.build_request(method=request.method, url=_target, headers=headers, content=body)
                        _resp = await http_client.send(_req, stream=True)
                        if _resp.status_code >= 400 and _resp.status_code not in (401, 403) and _idx < len(attempt_bases) - 1:
                            logger.warning("Upstream %s returned %s, trying fallback %s", _base, _resp.status_code, attempt_bases[_idx + 1])
                            await _resp.aclose()
                            continue
                        response = _resp
                        target_url = _target
                        upstream_base = _base
                        if _idx > 0:
                            used_fallback_url = _base
                            logger.info("Fallback succeeded: provider=%s fallback=%s", provider, _base)
                        break
                    except Exception as exc:
                        last_exc = exc
                        logger.warning("Failed to forward to %s: %s", _target, exc)
                        if _idx == len(attempt_bases) - 1:
                            return JSONResponse(status_code=502, content={"error": {"type": "odotoken_proxy_error", "message": str(exc)}})
                        continue
                if response is None:
                    # primary mock already handled; fallback loop exhausted
                    if last_exc is not None:
                        return JSONResponse(status_code=502, content={"error": {"type": "odotoken_proxy_error", "message": str(last_exc)}})
                    return JSONResponse(status_code=502, content={"error": {"type": "odotoken_proxy_error", "message": "no upstream available"}})
            # annotate tags/metadata if fallback was used
            if used_fallback_url:
                tags = f"{tags},fallback".strip(",") if tags else "fallback"
                # add fallback info to prompt_meta so it persists in metadata JSON
                try:
                    prompt_meta = {**prompt_meta, "fallback": True, "fallback_url": used_fallback_url}
                except Exception:
                    pass

            resp_headers = dict(response.headers)
            # Remove hop-by-hop & compression headers that cause streaming issues
            resp_headers.pop("content-encoding", None)
            resp_headers.pop("content-length", None)
            resp_headers.pop("transfer-encoding", None)
            content_type = resp_headers.get("content-type", "")

            is_sse = "text/event-stream" in content_type or is_stream

            if is_sse:
                # Handle streaming response — TTFB = start -> first chunk
                accumulator = StreamUsageAccumulator(provider=provider, default_model=fallback_model)

                async def stream_generator():
                    try:
                        async for chunk in response.aiter_bytes():
                            if accumulator.t_first_chunk_ms is None:
                                accumulator.mark_first_chunk((time.perf_counter() - start_time) * 1000.0)
                            accumulator.process_chunk(chunk)
                            yield chunk
                    finally:
                        await response.aclose()
                        latency_ms = (time.perf_counter() - start_time) * 1000.0
                        ttfb = round(accumulator.t_first_chunk_ms or latency_ms, 2)
                        streaming = round(max(0.0, latency_ms - (accumulator.t_first_chunk_ms or 0)), 2)
                        usage = accumulator.finalize()
                        # Post-hoc provider correction: if URL heuristic said "openai"
                        # but model name indicates a different provider, use the real one.
                        inferred = UsageParser.infer_provider_from_model(usage.model)
                        if inferred and provider == "openai":
                            provider = inferred
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
                        with _otel_span("odotoken.db.write", {"provider": provider}):
                            await asyncio.to_thread(
                                db.record_event,
                                TokenEvent(
                                    provider=provider,
                                    model=usage.model,
                                client_app=client_app,
                                shadow_cost=shadow,
                                project_id=project_id,
                                tags=tags,
                                metadata=json.dumps(prompt_meta),
                                prompt_tokens=usage.prompt_tokens,
                                completion_tokens=usage.completion_tokens,
                                total_tokens=usage.total_tokens,
                                cache_read_tokens=usage.cache_read_tokens,
                                cache_write_tokens=usage.cache_write_tokens,
                                cost_usd=cost,
                                latency_ms=round(latency_ms, 2),
                                ttfb_ms=ttfb,
                                streaming_ms=streaming,
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
                # Post-hoc provider correction: if URL heuristic said "openai"
                # but model name indicates a different provider, use the real one.
                inferred = UsageParser.infer_provider_from_model(model_name)
                if inferred and provider == "openai":
                    provider = inferred

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
                    with _otel_span("odotoken.db.write", {"provider": provider}):
                        await asyncio.to_thread(
                            db.record_event,
                            TokenEvent(
                                provider=provider,
                                model=model_name,
                            client_app=client_app,
                            shadow_cost=shadow_cost,
                            project_id=project_id,
                            tags=tags,
                            metadata=json.dumps(prompt_meta),
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

                if sem_key and response.status_code == 200 and res_json:
                    try:
                        await asyncio.to_thread(db.semantic_cache_set, sem_key, res_json)
                    except Exception:
                        pass

                return JSONResponse(
                    content=res_json if res_json else None,
                    status_code=response.status_code,
                    headers=resp_headers,
                )

    return app
