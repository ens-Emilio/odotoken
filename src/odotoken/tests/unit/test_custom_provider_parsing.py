"""Tests for the generic SSE usage accumulation in custom OpenAI-compatible providers.

Regression guard for the bug where verboo/kodeo/pegasus streams recorded zero
tokens: the accumulator only handled anthropic/openai/gemini provider names.
"""

from __future__ import annotations

from odotoken.parser import StreamUsageAccumulator


def _final_chunk(prompt: int, completion: int, total: int, model: str = "deepseek-v4.1-flash") -> str:
    return (
        f'data: {{"model":"{model}","choices":[{{"delta":{{}}}}],'
        f'"usage":{{"prompt_tokens":{prompt},"completion_tokens":{completion},"total_tokens":{total}}}}}\n\n'
    )


class TestCustomProviderSSE:
    """Custom OpenAI-compatible providers (verboo, kodeo, pegasus…)."""

    def _accumulate(self, provider: str, chunks: list[str]) -> StreamUsageAccumulator:
        acc = StreamUsageAccumulator(provider=provider, default_model="")
        for chunk in chunks:
            acc.process_chunk(chunk)
        acc.process_chunk("data: [DONE]\n\n")
        return acc

    def test_verboo_stream_usage_in_final_chunk(self):
        acc = self._accumulate(
            "verboo",
            [
                'data: {"model":"glm-5.3-flash","choices":[{"delta":{"content":"olá"}}]}\n\n',
                _final_chunk(12, 30, 42, model="glm-5.3-flash"),
            ],
        )
        usage = acc.finalize()
        assert usage.prompt_tokens == 12
        assert usage.completion_tokens == 30
        assert usage.total_tokens == 42
        assert usage.model == "glm-5.3-flash"

    def test_kodeo_stream_usage(self):
        acc = self._accumulate("kodeo", [_final_chunk(100, 50, 150, model="gpt-6-sol")])
        usage = acc.finalize()
        assert usage.total_tokens == 150
        assert usage.model == "gpt-6-sol"

    def test_pegasus_stream_anthropic_style_keys_fallback(self):
        # Some routers translate to Anthropic-style usage keys inside an
        # OpenAI-shaped SSE chunk; the generic branch falls back to them.
        acc = self._accumulate(
            "pegasus",
            [
                'data: {"model":"pegasus-claude-thinking","usage":{'
                '"input_tokens":8,"output_tokens":7,"total_tokens":15}}\n\n',
            ],
        )
        usage = acc.finalize()
        assert usage.prompt_tokens == 8
        assert usage.completion_tokens == 7
        assert usage.total_tokens == 15

    def test_stream_with_cached_tokens_detail(self):
        acc = self._accumulate(
            "verboo",
            [
                'data: {"usage":{"prompt_tokens":50,"completion_tokens":10,"total_tokens":60,'
                '"prompt_tokens_details":{"cached_tokens":40}}}\n\n',
            ],
        )
        usage = acc.finalize()
        assert usage.prompt_tokens == 50
        assert usage.completion_tokens == 10
        assert usage.cache_read_tokens == 40

    def test_stream_without_usage_stays_zero(self):
        acc = self._accumulate(
            "verboo",
            [
                'data: {"model":"glm-5.3-flash","choices":[{"delta":{"content":"hi"}}]}\n\n',
            ],
        )
        usage = acc.finalize()
        assert usage.total_tokens == 0

    def test_multiple_usage_chunks_last_wins(self):
        # OpenAI streams can carry a usage chunk per keep-alive; the final one wins.
        acc = self._accumulate(
            "useoneai",
            [
                'data: {"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}\n\n',
                _final_chunk(10, 4, 14),
            ],
        )
        usage = acc.finalize()
        assert usage.prompt_tokens == 10
        assert usage.completion_tokens == 4
        assert usage.total_tokens == 14


class TestJSONCustomProvider:
    """JSON (non-stream) responses from custom providers."""

    def test_json_response_generic_provider(self):
        from odotoken.parser import UsageParser

        res = UsageParser.parse_json_response(
            "verboo",
            {
                "model": "deepseek-v4-flash",
                "usage": {"prompt_tokens": 23, "completion_tokens": 71, "total_tokens": 94},
            },
            fallback_model="x",
        )
        assert res.prompt_tokens == 23
        assert res.completion_tokens == 71
        assert res.total_tokens == 94

    def test_json_response_anthropic_style_keys(self):
        from odotoken.parser import UsageParser

        res = UsageParser.parse_json_response(
            "pegasus",
            {"usage": {"input_tokens": 8, "output_tokens": 7}},
        )
        assert res.prompt_tokens == 8
        assert res.completion_tokens == 7
        assert res.total_tokens == 15
