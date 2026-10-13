"""Main token counting service orchestrating adapters, cache, and catalog."""

from __future__ import annotations

import asyncio

from odotoken.adapters.base import TokenCountAdapter
from odotoken.adapters.heuristic import HeuristicAdapter
from odotoken.adapters.remote import create_remote_adapters
from odotoken.domain.entities import (
    BatchCountResult,
    CountRequest,
    CountResult,
    ModelInfo,
)
from odotoken.domain.errors import TokenCountError
from odotoken.infra.config import TokenCounterConfig
from odotoken.infra.logging import get_logger, get_metrics, measure_time
from odotoken.services.cache import TokenCache
from odotoken.services.model_catalog import ModelCatalog

logger = get_logger(__name__)


class TokenCounter:
    """Main service for token counting with multiple adapter strategies."""

    def __init__(
        self,
        config: TokenCounterConfig,
        catalog: ModelCatalog,
        cache: TokenCache,
    ):
        self.config = config
        self.catalog = catalog
        self.cache = cache
        self._adapters: list[TokenCountAdapter] = []

        # Register built-in heuristic adapter
        self._adapters.append(HeuristicAdapter(config.heuristic))

        # Register remote adapters from config (pure constructor calls — no I/O)
        for adapter in create_remote_adapters(self.config.providers):
            self._adapters.append(adapter)
            logger.info("Registered remote adapter: %s", adapter.name)

        # Update cache versions
        self.cache.set_catalog_version(catalog.version)

    def register_adapter(self, adapter: TokenCountAdapter) -> None:
        """Register a new adapter (e.g., remote, local tokenizer)."""
        self._adapters.append(adapter)

    def _select_adapter(self, model_info: ModelInfo) -> TokenCountAdapter:
        """Select the best adapter for the given model."""
        method = model_info.counting_method

        # Try to find adapter by method name
        for adapter in self._adapters:
            if adapter.name == method and adapter.supports_model(model_info.id):
                return adapter

        # Fallback to first adapter that supports the model
        for adapter in self._adapters:
            if adapter.supports_model(model_info.id):
                logger.warning(
                    "No adapter for method '%s', falling back to '%s' for model '%s'",
                    method,
                    adapter.name,
                    model_info.id,
                )
                return adapter

        # Ultimate fallback: heuristic
        return self._adapters[0]

    @measure_time("token_counter.count.duration_ms")
    async def count(self, request: CountRequest) -> CountResult:
        """Count tokens for a single request."""
        metrics = get_metrics()
        metrics.increment("token_counter.count.requests")

        # Resolve model
        model_info = self.catalog.resolve_model(request.model, self.config.default_provider)

        # Validate request size
        self._validate_request(request, model_info)

        # Check cache
        cached = self.cache.get(request, model_info)
        if cached:
            metrics.increment("token_counter.cache.hits")
            logger.debug("Cache hit for model '%s'", model_info.id)
            return cached

        metrics.increment("token_counter.cache.misses")

        # Select adapter and count
        adapter = self._select_adapter(model_info)
        logger.debug("Using adapter '%s' for model '%s'", adapter.name, model_info.id)

        try:
            result = await adapter.count(request, model_info)
        except Exception as e:
            metrics.increment("token_counter.errors", labels={"adapter": adapter.name})
            logger.error("Adapter '%s' failed: %s", adapter.name, e)
            raise TokenCountError(f"Token counting failed: {e}") from e

        # Store in cache
        self.cache.set(request, model_info, result)

        metrics.increment("token_counter.count.success", labels={"method": adapter.name})
        return result

    @measure_time("token_counter.count_batch.duration_ms")
    async def count_batch(self, requests: list[CountRequest]) -> BatchCountResult:
        """Count tokens for multiple requests concurrently."""
        metrics = get_metrics()
        metrics.increment("token_counter.batch.requests")

        if len(requests) > self.config.max_batch_items:
            raise TokenCountError(f"Batch size {len(requests)} exceeds maximum {self.config.max_batch_items}")

        semaphore = asyncio.Semaphore(10)  # Limit concurrency

        async def count_one(req: CountRequest) -> tuple[str, CountResult | Exception]:
            async with semaphore:
                try:
                    return req.model, await self.count(req)
                except Exception as e:
                    return req.model, e

        results = await asyncio.gather(*[count_one(r) for r in requests])

        batch_result = BatchCountResult(
            total_items=len(requests),
            successful_items=0,
            failed_items=0,
            total_tokens=0,
        )

        for model_name, result in results:
            if isinstance(result, Exception):
                batch_result.failed_items += 1
                batch_result.errors.append({"model": model_name, "error": str(result)})
            else:
                batch_result.successful_items += 1
                batch_result.total_tokens += result.total_tokens
                batch_result.results.append(result)

        return batch_result

    def _validate_request(self, request: CountRequest, model_info: ModelInfo) -> None:
        """Validate request against limits."""
        # Rough estimate of input size
        total_chars = len(request.text or "")
        if request.messages:
            for msg in request.messages:
                content = msg.get("content", "")
                if isinstance(content, str):
                    total_chars += len(content)
                elif isinstance(content, list):
                    for part in content:
                        if part.get("type") == "text":
                            total_chars += len(part.get("text", ""))

        if total_chars > self.config.max_request_tokens * 4:  # rough chars-to-tokens
            raise TokenCountError(f"Request too large: ~{total_chars // 4} tokens estimated")

    def get_model_info(self, model: str) -> ModelInfo:
        """Get model information."""
        return self.catalog.get(model)

    def list_models(self) -> list[ModelInfo]:
        """List all known models."""
        return self.catalog.list_models()

    def get_cache_stats(self) -> dict:
        """Get cache statistics."""
        return self.cache.stats()
