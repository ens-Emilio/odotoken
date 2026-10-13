"""Unit tests for token counting adapters."""

from __future__ import annotations

import pytest

from odotoken.adapters.heuristic import HeuristicAdapter
from odotoken.domain.entities import CountRequest, ModelInfo
from odotoken.infra.config import HeuristicConfig


@pytest.fixture
def heuristic_config():
    return HeuristicConfig(chars_per_token=4.0, words_per_token=0.75)


@pytest.fixture
def heuristic_adapter(heuristic_config):
    return HeuristicAdapter(heuristic_config)


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


class TestHeuristicAdapter:
    """Tests for HeuristicAdapter."""

    @pytest.mark.asyncio
    async def test_count_text_basic(self, heuristic_adapter, model_info):
        request = CountRequest(model="gpt-4o-mini", text="Hello world")
        result = await heuristic_adapter.count(request, model_info)

        assert result.total_tokens > 0
        assert result.method == "heuristic"
        assert result.provider == "openai"
        assert "approximation" in result.warnings[0].lower()
        assert result.cache_hit is False

    @pytest.mark.asyncio
    async def test_count_text_chinese(self, heuristic_adapter, model_info):
        # Chinese chars typically ~1.5-2 tokens per char
        request = CountRequest(model="gpt-4o-mini", text="你好世界")
        result = await heuristic_adapter.count(request, model_info)

        assert result.total_tokens > 0
        assert result.method == "heuristic"

    @pytest.mark.asyncio
    async def test_count_messages(self, heuristic_adapter, model_info):
        request = CountRequest(
            model="gpt-4o-mini",
            messages=[
                {"role": "system", "content": "You are a helpful assistant"},
                {"role": "user", "content": "Hello"},
                {"role": "assistant", "content": "Hi there!"},
            ],
        )
        result = await heuristic_adapter.count(request, model_info)

        assert result.total_tokens > 0
        assert result.overhead_tokens > 0  # chat overhead applied
        assert len(result.tokens_per_message) == 3

    @pytest.mark.asyncio
    async def test_count_with_tools(self, heuristic_adapter, model_info):
        request = CountRequest(
            model="gpt-4o-mini",
            text="What's the weather?",
            tools=[
                {
                    "type": "function",
                    "function": {"name": "get_weather", "description": "Get weather", "parameters": {}},
                }
            ],
        )
        result = await heuristic_adapter.count(request, model_info)

        assert result.tool_tokens > 0

    @pytest.mark.asyncio
    async def test_count_empty_text(self, heuristic_adapter, model_info):
        request = CountRequest(model="gpt-4o-mini", text="")
        result = await heuristic_adapter.count(request, model_info)

        assert result.total_tokens == 0
        assert result.overhead_tokens == 0

    @pytest.mark.asyncio
    async def test_supports_any_model(self, heuristic_adapter):
        assert heuristic_adapter.supports_model("gpt-4o")
        assert heuristic_adapter.supports_model("claude-3-opus")
        assert heuristic_adapter.supports_model("unknown-model-xyz")

    @pytest.mark.asyncio
    async def test_unicode_emoji(self, heuristic_adapter, model_info):
        request = CountRequest(model="gpt-4o-mini", text="😀🎉🚀")
        result = await heuristic_adapter.count(request, model_info)

        assert result.total_tokens > 0

    @pytest.mark.asyncio
    async def test_newlines_and_special_chars(self, heuristic_adapter, model_info):
        request = CountRequest(model="gpt-4o-mini", text="Line 1\nLine 2\tTabbed")
        result = await heuristic_adapter.count(request, model_info)

        assert result.total_tokens > 0


class TestHeuristicConfig:
    """Tests for HeuristicConfig."""

    def test_default_config(self):
        config = HeuristicConfig()
        assert config.chars_per_token == 4.0
        assert config.words_per_token == 0.75

    def test_custom_config(self):
        config = HeuristicConfig(chars_per_token=3.0, words_per_token=0.5)
        assert config.chars_per_token == 3.0
        assert config.words_per_token == 0.5
