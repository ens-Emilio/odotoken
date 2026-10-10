"""Golden tests for token counting accuracy."""

from __future__ import annotations

import json
import pytest

from odotoken.adapters.heuristic import HeuristicAdapter
from odotoken.domain.entities import CountRequest, ModelInfo
from odotoken.infra.config import HeuristicConfig


@pytest.fixture
def heuristic_adapter():
    return HeuristicAdapter(HeuristicConfig())


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
def golden_cases():
    with open("src/odotoken/tests/fixtures/golden_counts.json", "r") as f:
        return json.load(f)["test_cases"]


class TestGoldenCounts:
    """Golden tests to ensure token counting stability."""

    @pytest.mark.asyncio
    async def test_golden_cases(self, heuristic_adapter, model_info, golden_cases):
        for case in golden_cases:
            if "text" in case:
                request = CountRequest(model=case["model"], text=case["text"])
            elif "messages" in case:
                request = CountRequest(model=case["model"], messages=case["messages"])
            else:
                continue

            result = await heuristic_adapter.count(request, model_info)

            expected = case["expected_tokens"]
            tolerance = case.get("tolerance", 2)
            actual = result.total_tokens

            assert abs(actual - expected) <= tolerance, (
                f"Golden test failed for '{case['name']}': "
                f"expected {expected}±{tolerance}, got {actual}"
            )

            # Verify method is as expected
            assert result.method == case["method"]