"""In-memory cache with TTL and size limits for token counting."""

from __future__ import annotations

import hashlib
import time
from collections import OrderedDict
from dataclasses import dataclass
from typing import Any

from odotoken.domain.entities import CountRequest, CountResult, ModelInfo
from odotoken.infra.config import TokenCounterConfig
from odotoken.infra.logging import get_logger

logger = get_logger(__name__)


@dataclass
class CacheEntry:
    result: CountResult
    expires_at: float
    model_version: str
    catalog_version: str


class TokenCache:
    """LRU cache with TTL and max size for token counting results."""

    def __init__(self, config: TokenCounterConfig):
        self.config = config
        self._cache: OrderedDict[str, CacheEntry] = OrderedDict()
        self._catalog_version = ""
        self._tokenizer_versions: dict[str, str] = {}

    def set_catalog_version(self, version: str) -> None:
        if version != self._catalog_version:
            logger.info("Catalog version changed, invalidating cache")
            self._catalog_version = version
            self.clear()

    def set_tokenizer_version(self, adapter_name: str, version: str) -> None:
        if self._tokenizer_versions.get(adapter_name) != version:
            logger.info("Tokenizer version changed for %s, invalidating cache", adapter_name)
            self._tokenizer_versions[adapter_name] = version
            self.clear()

    def _make_key(self, request: CountRequest, model_info: ModelInfo) -> str:
        """Create deterministic cache key from request content."""
        parts = [
            request.model,
            model_info.id,
            model_info.provider,
            str(request.text or ""),
            str(request.messages or []),
            str(request.tools or []),
            self._catalog_version,
            self._tokenizer_versions.get(model_info.counting_method, ""),
        ]
        content = "|".join(parts)
        return hashlib.sha256(content.encode()).hexdigest()[:32]

    def get(self, request: CountRequest, model_info: ModelInfo) -> CountResult | None:
        if not self.config.cache_enabled:
            return None

        key = self._make_key(request, model_info)
        entry = self._cache.get(key)

        if entry is None:
            return None

        if time.time() > entry.expires_at:
            del self._cache[key]
            return None

        # Move to end (LRU)
        self._cache.move_to_end(key)

        # Mark as cache hit
        result = entry.result
        result.cache_hit = True
        return result

    def set(self, request: CountRequest, model_info: ModelInfo, result: CountResult) -> None:
        if not self.config.cache_enabled:
            return

        key = self._make_key(request, model_info)
        expires_at = time.time() + self.config.cache_ttl_seconds

        entry = CacheEntry(
            result=result,
            expires_at=expires_at,
            model_version=model_info.id,
            catalog_version=self._catalog_version,
        )

        self._cache[key] = entry
        self._cache.move_to_end(key)

        # Evict if over size limit
        while len(self._cache) > self.config.cache_max_size:
            self._cache.popitem(last=False)

    def clear(self) -> None:
        self._cache.clear()

    def stats(self) -> dict[str, Any]:
        return {
            "size": len(self._cache),
            "max_size": self.config.cache_max_size,
            "ttl_seconds": self.config.cache_ttl_seconds,
            "catalog_version": self._catalog_version,
            "tokenizer_versions": dict(self._tokenizer_versions),
        }