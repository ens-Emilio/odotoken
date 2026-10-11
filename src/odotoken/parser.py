"""Parsers for response payloads and SSE streams from different LLM providers
(Anthropic, OpenAI, Gemini, Ollama) to extract token counts and model info.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any, Optional


@dataclass
class UsageResult:
    provider: str
    model: str
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0
    cache_read_tokens: int = 0
    cache_write_tokens: int = 0


class UsageParser:
    @staticmethod
    def identify_provider(path: str, headers: dict[str, str]) -> str:
        path_lower = path.lower()
        if "anthropic" in path_lower or "x-api-key" in headers or "anthropic-version" in headers:
            return "anthropic"
        if "gemini" in path_lower or "models/gemini" in path_lower:
            return "gemini"
        if "/api/generate" in path_lower or "/api/chat" in path_lower:
            return "ollama"
        # Default standard OpenAI compatibility
        return "openai"

    @staticmethod
    def identify_client_app(headers: dict[str, str]) -> str:
        """Infer client app name from User-Agent or custom headers."""
        custom_app = headers.get("x-client-app") or headers.get("x-app-name") or headers.get("x-client-id")
        if custom_app:
            return custom_app.strip().lower()

        ua = headers.get("user-agent", "").lower()
        if "pi-agent" in ua or "pi agent" in ua:
            return "pi agent"
        if "pi" in ua:
            return "pi agent"
        if "omp" in ua:
            return "omp"
        if "opencode" in ua:
            return "opencode"
        if "claude" in ua:
            return "claude-client"
        if "curl" in ua:
            return "curl"
        if "python" in ua or "httpx" in ua:
            return "python-client"

        return "unknown"

    @classmethod
    def parse_json_response(
        cls,
        provider: str,
        data: dict[str, Any],
        fallback_model: str = "",
    ) -> UsageResult:
        model = data.get("model") or fallback_model or "unknown"
        res = UsageResult(provider=provider, model=model)

        if provider == "anthropic":
            usage = data.get("usage", {})
            res.prompt_tokens = usage.get("input_tokens", 0)
            res.completion_tokens = usage.get("output_tokens", 0)
            res.cache_read_tokens = usage.get("cache_read_input_tokens", 0)
            res.cache_write_tokens = usage.get("cache_creation_input_tokens", 0)
            res.total_tokens = res.prompt_tokens + res.completion_tokens

        elif provider == "openai":
            usage = data.get("usage", {})
            res.prompt_tokens = usage.get("prompt_tokens", 0)
            res.completion_tokens = usage.get("completion_tokens", 0)
            res.total_tokens = usage.get("total_tokens", res.prompt_tokens + res.completion_tokens)
            details = usage.get("prompt_tokens_details", {})
            if isinstance(details, dict):
                res.cache_read_tokens = details.get("cached_tokens", 0)

        elif provider == "gemini":
            usage = data.get("usageMetadata", {})
            res.prompt_tokens = usage.get("promptTokenCount", 0)
            res.completion_tokens = usage.get("candidatesTokenCount", 0)
            res.total_tokens = usage.get("totalTokenCount", res.prompt_tokens + res.completion_tokens)
            res.cache_read_tokens = usage.get("cachedContentTokenCount", 0)

        elif provider == "ollama":
            res.prompt_tokens = data.get("prompt_eval_count", 0)
            res.completion_tokens = data.get("eval_count", 0)
            res.total_tokens = res.prompt_tokens + res.completion_tokens

        else:
            # Custom providers (verboo, kodeo, useoneai, pegasus, cheapestinference…)
            # are OpenAI-compatible: use the OpenAI shape, falling back to
            # Anthropic-style keys for routers that translate the format.
            usage = data.get("usage", {})
            res.prompt_tokens = usage.get("prompt_tokens", usage.get("input_tokens", 0))
            res.completion_tokens = usage.get("completion_tokens", usage.get("output_tokens", 0))
            res.total_tokens = usage.get("total_tokens", res.prompt_tokens + res.completion_tokens)
            details = usage.get("prompt_tokens_details", {})
            if isinstance(details, dict):
                res.cache_read_tokens = details.get("cached_tokens", 0)

        return res


class StreamUsageAccumulator:
    """Accumulates and parses SSE chunks across streaming responses."""

    def __init__(self, provider: str, default_model: str = ""):
        self.provider = provider
        self.model = default_model
        self.prompt_tokens = 0
        self.completion_tokens = 0
        self.total_tokens = 0
        self.cache_read_tokens = 0
        self.cache_write_tokens = 0
        self._buffer = ""

    def process_chunk(self, chunk: bytes | str) -> None:
        if isinstance(chunk, bytes):
            text = chunk.decode("utf-8", errors="replace")
        else:
            text = chunk

        self._buffer += text
        lines = self._buffer.split("\n")
        # Keep trailing incomplete line in buffer
        self._buffer = lines[-1]

        current_event = ""
        for line in lines[:-1]:
            line = line.strip()
            if not line:
                current_event = ""
                continue
            if line.startswith("event:"):
                current_event = line[len("event:"):].strip()
            elif line.startswith("data:"):
                data_str = line[len("data:"):].strip()
                if data_str == "[DONE]":
                    continue
                try:
                    payload = json.loads(data_str)
                    self._handle_payload(payload, current_event)
                except Exception:
                    pass

    def _handle_payload(self, data: dict[str, Any], event: str) -> None:
        if "model" in data:
            self.model = data["model"]

        if self.provider == "anthropic":
            if event == "message_start" or data.get("type") == "message_start":
                msg = data.get("message", {})
                if "model" in msg:
                    self.model = msg["model"]
                usage = msg.get("usage", {})
                self.prompt_tokens = usage.get("input_tokens", self.prompt_tokens)
                self.cache_read_tokens = usage.get("cache_read_input_tokens", self.cache_read_tokens)
                self.cache_write_tokens = usage.get("cache_creation_input_tokens", self.cache_write_tokens)
            elif event == "message_delta" or data.get("type") == "message_delta":
                usage = data.get("usage", {})
                self.completion_tokens = usage.get("output_tokens", self.completion_tokens)

        elif self.provider == "openai":
            if "usage" in data and data["usage"]:
                usage = data["usage"]
                self.prompt_tokens = usage.get("prompt_tokens", self.prompt_tokens)
                self.completion_tokens = usage.get("completion_tokens", self.completion_tokens)
                self.total_tokens = usage.get("total_tokens", self.total_tokens)
                details = usage.get("prompt_tokens_details", {})
                if isinstance(details, dict):
                    self.cache_read_tokens = details.get("cached_tokens", self.cache_read_tokens)

        elif self.provider == "gemini":
            if "usageMetadata" in data:
                usage = data["usageMetadata"]
                self.prompt_tokens = usage.get("promptTokenCount", self.prompt_tokens)
                self.completion_tokens = usage.get("candidatesTokenCount", self.completion_tokens)
                self.total_tokens = usage.get("totalTokenCount", self.total_tokens)
                self.cache_read_tokens = usage.get("cachedContentTokenCount", self.cache_read_tokens)

        else:
            # Custom OpenAI-compatible providers: usage arrives in a final
            # chunk (often with empty choices) or alongside "type": "usage".
            if "usage" in data and data["usage"]:
                usage = data["usage"]
                self.prompt_tokens = usage.get("prompt_tokens", usage.get("input_tokens", self.prompt_tokens))
                self.completion_tokens = usage.get("completion_tokens", usage.get("output_tokens", self.completion_tokens))
                self.total_tokens = usage.get("total_tokens", self.total_tokens)
                details = usage.get("prompt_tokens_details", {})
                if isinstance(details, dict):
                    self.cache_read_tokens = details.get("cached_tokens", self.cache_read_tokens)

    def finalize(self) -> UsageResult:
        if not self.total_tokens:
            self.total_tokens = self.prompt_tokens + self.completion_tokens
        return UsageResult(
            provider=self.provider,
            model=self.model or "unknown",
            prompt_tokens=self.prompt_tokens,
            completion_tokens=self.completion_tokens,
            total_tokens=self.total_tokens,
            cache_read_tokens=self.cache_read_tokens,
            cache_write_tokens=self.cache_write_tokens,
        )
