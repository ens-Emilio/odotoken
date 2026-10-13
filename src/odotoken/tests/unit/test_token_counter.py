"""Unit tests for TokenCounter service."""

from __future__ import annotations

import pytest

from odotoken.adapters.heuristic import HeuristicAdapter
from odotoken.domain.entities import CountRequest, ModelInfo
from odotoken.domain.errors import ModelNotSupportedError, TokenCountError
from odotoken.infra.config import HeuristicConfig, TokenCounterConfig
from odotoken.services.cache import TokenCache
from odotoken.services.model_catalog import ModelCatalog
from odotoken.services.token_counter import TokenCounter


@pytest.fixture
def tc_config():
    return TokenCounterConfig(
        enabled=True,
        default_model="gpt-4o-mini",
        default_provider="openai",
        max_request_tokens=1000000,
        max_batch_items=100,
        max_batch_tokens=5000000,
        cache_enabled=True,
        cache_ttl_seconds=3600,
        cache_max_size=10000,
        providers={},
        heuristic=HeuristicConfig(),
    )


@pytest.fixture
def catalog():
    return ModelCatalog("src/odotoken/catalog/models.json")


@pytest.fixture
def cache(tc_config):
    return TokenCache(tc_config)


@pytest.fixture
def token_counter(tc_config, catalog, cache):
    return TokenCounter(tc_config, catalog, cache)


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


class TestTokenCounter:
    """Tests for TokenCounter service."""

    @pytest.mark.asyncio
    async def test_count_text(self, token_counter):
        request = CountRequest(model="gpt-4o-mini", text="Hello world")
        result = await token_counter.count(request)

        assert result.total_tokens > 0
        assert result.method == "heuristic"
        assert result.provider == "openai"

    @pytest.mark.asyncio
    async def test_count_messages(self, token_counter):
        request = CountRequest(
            model="gpt-4o-mini",
            messages=[
                {"role": "user", "content": "Hello"},
                {"role": "assistant", "content": "Hi there!"},
            ],
        )
        result = await token_counter.count(request)

        assert result.total_tokens > 0
        assert result.overhead_tokens > 0
        assert len(result.tokens_per_message) == 2

    @pytest.mark.asyncio
    async def test_count_batch(self, token_counter):
        requests = [
            CountRequest(model="gpt-4o-mini", text="Test 1"),
            CountRequest(model="gpt-4o-mini", text="Test 2"),
        ]
        batch_result = await token_counter.count_batch(requests)

        assert batch_result.total_items == 2
        assert batch_result.successful_items == 2
        assert batch_result.failed_items == 0
        assert batch_result.total_tokens > 0
        assert len(batch_result.results) == 2

    @pytest.mark.asyncio
    async def test_count_batch_with_error(self, token_counter, catalog):
        requests = [
            CountRequest(model="gpt-4o-mini", text="Test 1"),
            CountRequest(model="gpt-4o-mini", text="Test 2"),
        ]
        batch_result = await token_counter.count_batch(requests)

        assert batch_result.total_items == 2
        assert batch_result.successful_items == 2

    @pytest.mark.asyncio
    async def test_batch_size_limit(self, token_counter):
        requests = [CountRequest(model="gpt-4o-mini", text="x") for _ in range(101)]

        with pytest.raises(TokenCountError, match="Batch size"):
            await token_counter.count_batch(requests)

    @pytest.mark.asyncio
    async def test_cache_hit(self, token_counter):
        request = CountRequest(model="gpt-4o-mini", text="Cache test")

        result1 = await token_counter.count(request)
        assert result1.cache_hit is False

        result2 = await token_counter.count(request)
        assert result2.cache_hit is True
        assert result2.total_tokens == result1.total_tokens

    @pytest.mark.asyncio
    async def test_list_models(self, token_counter):
        models = token_counter.list_models()
        assert len(models) == 8

    @pytest.mark.asyncio
    async def test_get_model_info(self, token_counter):
        model = token_counter.get_model_info("gpt-4o-mini")
        assert model.id == "gpt-4o-mini"
        assert model.provider == "openai"

    @pytest.mark.asyncio
    async def test_get_model_info_unknown(self, token_counter):
        with pytest.raises(ModelNotSupportedError):
            token_counter.get_model_info("unknown-model-xyz")

    def test_register_adapter(self, token_counter, tc_config, catalog, cache):
        counter = TokenCounter(tc_config, catalog, cache)

        assert len(counter._adapters) >= 1

        class MockAdapter(HeuristicAdapter):
            @property
            def name(self):
                return "mock"

            def supports_model(self, model: str) -> bool:
                return True

        mock = MockAdapter(tc_config.heuristic)
        counter.register_adapter(mock)

        adapter_names = [a.name for a in counter._adapters]
        assert "heuristic" in adapter_names
        assert "mock" in adapter_names
