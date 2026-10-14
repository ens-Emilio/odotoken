"""Pricing engine for calculating token costs across providers and models."""

from __future__ import annotations

import json
from pathlib import Path

DEFAULT_PRICES: dict[str, dict[str, float]] = {
    # Anthropic models (prices per 1M tokens)
    "claude-3-7-sonnet": {"input": 3.0, "output": 15.0, "cache_read": 0.30, "cache_write": 3.75},
    "claude-3-5-sonnet": {"input": 3.0, "output": 15.0, "cache_read": 0.30, "cache_write": 3.75},
    "claude-3-5-haiku": {"input": 0.8, "output": 4.0, "cache_read": 0.08, "cache_write": 1.0},
    "claude-3-opus": {"input": 15.0, "output": 75.0, "cache_read": 1.5, "cache_write": 18.75},
    # OpenAI models (prices per 1M tokens)
    "gpt-4o": {"input": 2.5, "output": 10.0, "cache_read": 1.25, "cache_write": 0.0},
    "gpt-4o-mini": {"input": 0.15, "output": 0.60, "cache_read": 0.075, "cache_write": 0.0},
    "o1": {"input": 15.0, "output": 60.0, "cache_read": 7.5, "cache_write": 0.0},
    "o3-mini": {"input": 1.10, "output": 4.40, "cache_read": 0.55, "cache_write": 0.0},
    # Google Gemini models
    "gemini-2.0-flash": {"input": 0.10, "output": 0.40, "cache_read": 0.025, "cache_write": 0.0},
    "gemini-1.5-pro": {"input": 1.25, "output": 5.0, "cache_read": 0.3125, "cache_write": 0.0},
    # Local / Ollama / Default fallback
    "default": {"input": 0.0, "output": 0.0, "cache_read": 0.0, "cache_write": 0.0},
}


class PricingCalculator:
    def __init__(self, price_file: str | Path = "prices.json"):
        self.price_file = Path(price_file)
        self.prices: dict[str, dict[str, float]] = dict(DEFAULT_PRICES)
        self.load_prices()

    def load_prices(self) -> None:
        if self.price_file.exists():
            try:
                with open(self.price_file, encoding="utf-8") as f:
                    data = json.load(f)
                    if isinstance(data, dict):
                        self.prices.update(data)
            except Exception:
                pass

    def get_model_rates(self, model_name: str) -> dict[str, float]:
        """Find best matching price rate based on model prefix or exact name."""
        clean_name = model_name.lower().strip()
        if clean_name in self.prices:
            return self.prices[clean_name]

        # Try prefix / substring matching (e.g. claude-3-7-sonnet-20250219 -> claude-3-7-sonnet)
        for pattern, rates in self.prices.items():
            if pattern != "default" and pattern in clean_name:
                return rates

        return self.prices.get(
            "default", {"input": 0.0, "output": 0.0, "cache_read": 0.0, "cache_write": 0.0}
        )

    def calculate_cost(
        self,
        model: str,
        prompt_tokens: int,
        completion_tokens: int,
        cache_read_tokens: int = 0,
        cache_write_tokens: int = 0,
    ) -> float:
        """Calculate total USD cost for a given token usage (rates are per million tokens)."""
        rates = self.get_model_rates(model)
        # Standard input excludes cache read/write tokens if provider separates them
        effective_input = max(0, prompt_tokens - cache_read_tokens - cache_write_tokens)

        cost = (
            (effective_input * rates.get("input", 0.0))
            + (completion_tokens * rates.get("output", 0.0))
            + (cache_read_tokens * rates.get("cache_read", 0.0))
            + (cache_write_tokens * rates.get("cache_write", 0.0))
        ) / 1_000_000.0

        return round(cost, 7)

    def calculate_shadow_cost(
        self,
        model: str,
        cloud_equivalent: str = "",
        prompt_tokens: int = 0,
        completion_tokens: int = 0,
        cache_read_tokens: int = 0,
        cache_write_tokens: int = 0,
    ) -> float:
        """Theoretical cost if the request had gone to the cloud-equivalent model.

        For cloud models (paid rates) the shadow cost equals the actual cost —
        no savings. For local models (free rates, e.g. Ollama) the configured
        cloud_equivalent model's rates are used.
        """
        own_rates = self.get_model_rates(model)
        if own_rates.get("input", 0) > 0 or own_rates.get("output", 0) > 0:
            return self.calculate_cost(
                model, prompt_tokens, completion_tokens, cache_read_tokens, cache_write_tokens
            )
        target = cloud_equivalent or model
        return self.calculate_cost(
            target, prompt_tokens, completion_tokens, cache_read_tokens, cache_write_tokens
        )
