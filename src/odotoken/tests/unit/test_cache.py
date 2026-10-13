"""Unit tests for token cache."""

from __future__ import annotations

import pytest

from odotoken.domain.entities import CountRequest, CountResult, ModelInfo
from odotoken.infra.config import HeuristicConfig, TokenCounterConfig
from odotoken.services.cache import TokenCache


@pytest.fixture
def cache_config():
    return TokenCounterConfig(
        cache_enabled=True,
        cache_ttl_seconds=3600,
        cache_max_size=100,
        heuristic=HeuristicConfig(),
    )


@pytest.fixture
def token_cache(cache_config):
    return TokenCache(cache_config)


@pytest.fixture
def model_info():
    return ModelInfo(
        id="gpt-4o-mini",
        provider="openai",
        context_window=128000,
        max_output=16384,
        supports_messages=True,
        supports_tools=True,
        supports_vision=True,
        counting_method="heuristic",
        chat_overhead=3,
        message_overhead=3,
        tool_overhead=0,
    )


@pytest.fixture
def sample_request():
    return CountRequest(model="gpt-4o-mini", text="Hello world")


@pytest.fixture
def sample_result(model_info):
    return CountResult(
        model="gpt-4o-mini",
        resolved_model="gpt-4o-mini",
        provider="openai",
        tokenizer_used="heuristic",
        method="heuristic",
        input_tokens=10,
        output_tokens=0,
        total_tokens=10,
        context_window=128000,
    )


class TestTokenCache:
    """Tests for TokenCache."""

    def test_cache_disabled(self, cache_config):
        cache_config.cache_enabled = False
        cache = TokenCache(cache_config)

        result = cache.get(sample_request, model_info)
        assert result is None

        cache.set(sample_request, model_info, sample_result)
        result = cache.get(sample_request, model_info)
        assert result is None

    def test_cache_set_and_get(self, token_cache, sample_request, model_info, sample_result):
        token_cache.set(sample_request, model_info, sample_result)

        cached = token_cache.get(sample_request, model_info)
        assert cached is not None
        assert cached.cache_hit is True
        assert cached.total_tokens == 10

    def test_cache_miss_different_content(self, token_cache, model_info, sample_result):
        req1 = CountRequest(model="gpt-4o-mini", text="Hello")
        req2 = CountRequest(model="gpt-4o-mini", text="World")

        token_cache.set(req1, model_info, sample_result)

        cached = token_cache.get(req2, model_info)
        assert cached is None

    def test_cache_miss_different_model(self, token_cache, sample_result):
        req1 = CountRequest(model="gpt-4o-mini", text="Hello")
        req2 = CountRequest(model="gpt-4o", text="Hello")

        model1 = ModelInfo(
            id="gpt-4o-mini", provider="openai", context_window=128000, counting_method="heuristic"
        )
        model2 = ModelInfo(id="gpt-4o", provider="openai", context_window=128000, counting_method="heuristic")

        token_cache.set(req1, model1, sample_result)
        cached = token_cache.get(req2, model2)
        assert cached is None

    def test_cache_invalidation_on_catalog_version(
        self, token_cache, sample_request, model_info, sample_result
    ):
        token_cache.set_catalog_version("v1")
        token_cache.set(sample_request, model_info, sample_result)

        cached = token_cache.get(sample_request, model_info)
        assert cached is not None

        # Change catalog version - should invalidate
        token_cache.set_catalog_version("v2")
        cached = token_cache.get(sample_request, model_info)
        assert cached is None

    def test_cache_invalidation_on_tokenizer_version(
        self, token_cache, sample_request, model_info, sample_result
    ):
        token_cache.set_tokenizer_version("heuristic", "v1")
        token_cache.set(sample_request, model_info, sample_result)

        cached = token_cache.get(sample_request, model_info)
        assert cached is not None

        # Change tokenizer version - should invalidate
        token_cache.set_tokenizer_version("heuristic", "v2")
        cached = token_cache.get(sample_request, model_info)
        assert cached is None

    def test_cache_ttl_expiry(self, token_cache, sample_request, model_info, sample_result):
        token_cache.config.cache_ttl_seconds = 0  # Immediate expiry
        token_cache.set(sample_request, model_info, sample_result)

        # Should be expired
        cached = token_cache.get(sample_request, model_info)
        assert cached is None

    def test_cache_lru_eviction(self, cache_config):
        cache_config.cache_max_size = 2
        cache = TokenCache(cache_config)

        model = ModelInfo(id="test", provider="test", context_window=1000, counting_method="heuristic")

        # Fill cache
        for i in range(3):
            req = CountRequest(model="test", text=f"Text {i}")
            res = CountResult(total_tokens=i, model="test")
            cache.set(req, model, res)

        # First item should be evicted
        req_first = CountRequest(model="test", text="Text 0")
        assert cache.get(req_first, model) is None

        # Last two should remain
        req_second = CountRequest(model="test", text="Text 1")
        assert cache.get(req_second, model) is not None

        req_third = CountRequest(model="test", text="Text 2")
        assert cache.get(req_third, model) is not None

    def test_cache_stats(self, token_cache, sample_request, model_info, sample_result):
        token_cache.set(sample_request, model_info, sample_result)

        stats = token_cache.stats()
        assert stats["size"] == 1
        assert stats["max_size"] == 100
        assert stats["ttl_seconds"] == 3600
