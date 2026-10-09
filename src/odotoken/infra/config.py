"""Configuration loader for token counter."""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class ProviderConfig:
    enabled: bool = False
    api_key_env: str | None = None
    base_url: str = ""
    timeout_seconds: float = 30.0
    max_retries: int = 3
    count_endpoint: str = ""


@dataclass
class HeuristicConfig:
    chars_per_token: float = 4.0
    words_per_token: float = 0.75


@dataclass
class TokenCounterConfig:
    enabled: bool = True
    default_model: str = "gpt-4o-mini"
    default_provider: str = "openai"
    max_request_tokens: int = 1_000_000
    max_batch_items: int = 100
    max_batch_tokens: int = 5_000_000
    cache_enabled: bool = True
    cache_ttl_seconds: int = 3600
    cache_max_size: int = 10_000
    providers: dict[str, ProviderConfig] = field(default_factory=dict)
    heuristic: HeuristicConfig = field(default_factory=HeuristicConfig)


def load_config(path: str | Path = "config.json") -> dict[str, Any]:
    """Load configuration from JSON file with environment variable overrides."""
    config_path = Path(path)
    if config_path.exists():
        with open(config_path, "r", encoding="utf-8") as f:
            config = json.load(f)
    else:
        config = {}

    # Apply environment variable overrides for token_counter
    token_counter = config.get("token_counter", {})
    providers = token_counter.get("providers", {})

    for provider_name, provider_config in providers.items():
        api_key_env = provider_config.get("api_key_env")
        if api_key_env and api_key_env in os.environ:
            provider_config["api_key"] = os.environ[api_key_env]

    return config


def get_token_counter_config(config: dict[str, Any]) -> TokenCounterConfig:
    """Parse token_counter section into typed config."""
    tc = config.get("token_counter", {})
    providers = {}
    for name, p in tc.get("providers", {}).items():
        providers[name] = ProviderConfig(
            enabled=p.get("enabled", False),
            api_key_env=p.get("api_key_env"),
            base_url=p.get("base_url", ""),
            timeout_seconds=p.get("timeout_seconds", 30.0),
            max_retries=p.get("max_retries", 3),
            count_endpoint=p.get("count_endpoint", ""),
        )
    heuristic = HeuristicConfig(
        chars_per_token=tc.get("heuristic", {}).get("chars_per_token", 4.0),
        words_per_token=tc.get("heuristic", {}).get("words_per_token", 0.75),
    )
    return TokenCounterConfig(
        enabled=tc.get("enabled", True),
        default_model=tc.get("default_model", "gpt-4o-mini"),
        default_provider=tc.get("default_provider", "openai"),
        max_request_tokens=tc.get("max_request_tokens", 1_000_000),
        max_batch_items=tc.get("max_batch_items", 100),
        max_batch_tokens=tc.get("max_batch_tokens", 5_000_000),
        cache_enabled=tc.get("cache_enabled", True),
        cache_ttl_seconds=tc.get("cache_ttl_seconds", 3600),
        cache_max_size=tc.get("cache_max_size", 10_000),
        providers=providers,
        heuristic=heuristic,
    )