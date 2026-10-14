"""Mock provider adapter and chaos injection for testing client resilience.

The mock upstream answers synthetic OpenAI-shaped responses with plausible
usage numbers — metering works end-to-end with zero network. Chaos injects
probabilistic failures (429/500/latency) behind an explicit opt-in config.
"""

from __future__ import annotations

import asyncio
import hashlib
import logging
import random

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

logger = logging.getLogger("odotoken.chaos")


def synthesize_response(model: str, prompt_text: str) -> dict:
    """Deterministic synthetic OpenAI-shaped completion (stable per input)."""
    seed = int(hashlib.sha256(f"{model}:{prompt_text}".encode()).hexdigest()[:8], 16)
    rng = random.Random(seed)
    prompt_tokens = max(1, len(prompt_text) // 4)
    completion_tokens = rng.randint(10, 60)
    reply = "This is a deterministic mock response. "
    return {
        "id": f"chatcmpl-mock-{seed:08x}",
        "object": "chat.completion",
        "created": seed,
        "model": model,
        "choices": [
            {"index": 0, "message": {"role": "assistant", "content": reply}, "finish_reason": "stop"}
        ],
        "usage": {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "total_tokens": prompt_tokens + completion_tokens,
        },
    }


def is_mock_upstream(upstream_url: str) -> bool:
    return upstream_url.startswith("mock://")


class ChaosMiddleware(BaseHTTPMiddleware):
    """Probabilistic failure injection. Inactive unless chaos.enabled is true."""

    def __init__(self, app, chaos_cfg: dict | None = None):
        super().__init__(app)
        cfg = chaos_cfg or {}
        self.enabled = bool(cfg.get("enabled", False))
        self.rate = float(cfg.get("rate", 0.0))
        self.latency_ms = int(cfg.get("latency_ms", 0))
        self.errors = list(cfg.get("errors", [429, 500]))
        self._rng = random.Random(cfg.get("seed"))

    async def dispatch(self, request, call_next):
        if not self.enabled or self.rate <= 0:
            return await call_next(request)

        # Escape hatch for sanity tests
        if request.headers.get("x-odo-no-chaos") == "true":
            return await call_next(request)

        if self._rng.random() < self.rate:
            roll = self._rng.random()
            if roll < 0.5 and self.errors:
                status = self._rng.choice(self.errors)
                logger.info("chaos: injecting %s on %s", status, request.url.path)
                return JSONResponse(
                    status_code=status,
                    content={"error": {"type": "odotoken_chaos", "message": f"chaos-injected {status}"}},
                )
            if self.latency_ms:
                logger.info("chaos: injecting %dms latency on %s", self.latency_ms, request.url.path)
                await asyncio.sleep(self.latency_ms / 1000)

        return await call_next(request)
