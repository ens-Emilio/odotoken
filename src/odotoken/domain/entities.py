"""Domain entities for token counting."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class CountRequest:
    """Request to count tokens."""
    model: str
    text: str | None = None
    messages: list[dict[str, Any]] | None = None
    provider: str | None = None  # Override provider detection
    tools: list[dict[str, Any]] | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class CountResult:
    """Result of token counting."""
    request_id: str | None = None
    model: str = ""
    resolved_model: str = ""
    provider: str = ""
    tokenizer_used: str = ""
    method: str = ""  # remote, local, heuristic
    input_tokens: int = 0
    output_tokens: int = 0  # estimated if applicable
    total_tokens: int = 0
    tokens_per_message: list[int] = field(default_factory=list)
    tokens_per_part: list[int] = field(default_factory=list)
    tool_tokens: int = 0
    overhead_tokens: int = 0
    context_window: int | None = None
    context_utilization: float | None = None
    warnings: list[str] = field(default_factory=list)
    errors: list[str] = field(default_factory=list)
    cache_hit: bool = False
    duration_ms: float = 0.0


@dataclass
class BatchCountResult:
    """Result of batch token counting."""
    total_items: int = 0
    successful_items: int = 0
    failed_items: int = 0
    total_tokens: int = 0
    duration_ms: float = 0.0
    results: list[CountResult] = field(default_factory=list)
    errors: list[dict[str, Any]] = field(default_factory=list)


@dataclass
class ModelInfo:
    """Information about a model."""
    id: str
    aliases: list[str] = field(default_factory=list)
    provider: str = ""
    family: str = ""
    context_window: int = 0
    max_output: int = 0
    supports_messages: bool = False
    supports_tools: bool = False
    supports_vision: bool = False
    counting_method: str = "heuristic"  # remote, local, heuristic
    tokenizer: str | None = None
    chat_overhead: int = 0
    message_overhead: int = 0
    tool_overhead: int = 0
    notes: str = ""