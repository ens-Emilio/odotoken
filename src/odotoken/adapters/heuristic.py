"""Heuristic token counting adapter (fallback estimation)."""

from __future__ import annotations

import json
import math
from odotoken.adapters.base import BaseAdapter
from odotoken.domain.entities import CountRequest, CountResult, ModelInfo
from odotoken.infra.config import HeuristicConfig
from odotoken.infra.logging import get_logger, measure_time

logger = get_logger(__name__)


class HeuristicAdapter(BaseAdapter):
    """Heuristic-based token counter using character/word ratios."""

    def __init__(self, config: HeuristicConfig | None = None):
        self.config = config or HeuristicConfig()
        self._version = "heuristic-v1"

    @property
    def name(self) -> str:
        return "heuristic"

    def supports_model(self, model: str) -> bool:
        return True

    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        with measure_time("token_counter.heuristic.duration_ms"):
            import time
            start = time.perf_counter()

            total_chars = 0
            total_words = 0

            if request.text:
                total_chars += len(request.text)
                total_words += len(request.text.split())

            if request.messages:
                for msg in request.messages:
                    content = msg.get("content", "")
                    if isinstance(content, str):
                        total_chars += len(content)
                        total_words += len(content.split())
                    elif isinstance(content, list):
                        for part in content:
                            if part.get("type") == "text":
                                text = part.get("text", "")
                                total_chars += len(text)
                                total_words += len(text.split())

            if request.tools:
                for tool in request.tools:
                    tool_str = json.dumps(tool)
                    total_chars += len(tool_str)
                    total_words += len(tool_str.split())

            char_estimate = math.ceil(total_chars / self.config.chars_per_token)
            word_estimate = math.ceil(total_words / self.config.words_per_token)
            estimated = max(char_estimate, word_estimate)

            overhead = 0
            if request.messages:
                overhead += model_info.chat_overhead
                overhead += len(request.messages) * model_info.message_overhead
            if request.tools:
                overhead += len(request.tools) * model_info.tool_overhead

            total = estimated + overhead

            duration_ms = (time.perf_counter() - start) * 1000

            return CountResult(
                model=request.model,
                resolved_model=model_info.id,
                provider=model_info.provider,
                tokenizer_used=self.name,
                method="heuristic",
                input_tokens=total,
                output_tokens=0,
                total_tokens=total,
                overhead_tokens=overhead,
                context_window=model_info.context_window,
                context_utilization=total / model_info.context_window if model_info.context_window else None,
                warnings=["Token count is an approximation (heuristic method)"],
                duration_ms=duration_ms,
            )