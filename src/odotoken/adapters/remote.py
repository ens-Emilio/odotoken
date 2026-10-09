"""Remote token counting adapters using provider APIs."""

from __future__ import annotations

import httpx
from typing import Any

from odotoken.adapters.base import BaseAdapter
from odotoken.domain.entities import CountRequest, CountResult, ModelInfo
from odotoken.domain.errors import ProviderError, ProviderTimeoutError, ProviderNotConfiguredError, TokenCountError
from odotoken.infra.config import ProviderConfig
from odotoken.infra.logging import get_logger, measure_time

logger = get_logger(__name__)


class RemoteAdapterBase(BaseAdapter):
    """Base class for remote provider token counting."""

    def __init__(self, config: ProviderConfig):
        self.config = config
        self._client: httpx.AsyncClient | None = None

    @property
    def name(self) -> str:
        return getattr(self.config, "name", "remote")

    async def _get_client(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = httpx.AsyncClient(
                timeout=httpx.Timeout(
                    connect=10.0,
                    read=self.config.timeout_seconds,
                    write=10.0,
                    pool=5.0,
                ),
                follow_redirects=True,
            )
        return self._client

    async def close(self) -> None:
        if self._client:
            await self._client.aclose()
            self._client = None

    def _get_auth_headers(self) -> dict[str, str]:
        api_key = getattr(self.config, "api_key", None)
        if not api_key:
            raise ProviderNotConfiguredError(self.name)
        return {"Authorization": f"Bearer {api_key}"}

    @measure_time("token_counter.remote.request_duration_ms")
    async def _post(self, url: str, payload: dict[str, Any]) -> dict[str, Any]:
        client = await self._get_client()
        headers = self._get_auth_headers()
        headers["Content-Type"] = "application/json"

        for attempt in range(self.config.max_retries + 1):
            try:
                response = await client.post(url, json=payload, headers=headers)
                if response.is_success:
                    return response.json()

                if response.status_code == 429:
                    retry_after = float(response.headers.get("Retry-After", 1))
                    import asyncio
                    await asyncio.sleep(retry_after)
                    continue

                raise ProviderError(self.name, response.status_code, response.text)

            except httpx.TimeoutException:
                if attempt >= self.config.max_retries:
                    raise ProviderTimeoutError(self.name) from None
            except httpx.RequestError as e:
                if attempt >= self.config.max_retries:
                    raise TokenCountError(f"Request failed: {e}") from e

        raise TokenCountError("Max retries exceeded")

    def supports_model(self, model: str) -> bool:
        return True

    @measure_time("token_counter.remote.count.duration_ms")
    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        raise NotImplementedError


class OpenAITokenAdapter(RemoteAdapterBase):
    """OpenAI token counting via /v1/tokens/count endpoint."""

    def __init__(self, config: ProviderConfig):
        super().__init__(config)
        self.config.name = "openai"

    @property
    def name(self) -> str:
        return "openai"

    def supports_model(self, model: str) -> bool:
        return model.startswith(("gpt-", "o1", "o3-"))

    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        import time
        start = time.perf_counter()

        payload = {
            "model": request.model,
        }

        if request.text:
            payload["prompt"] = request.text
        elif request.messages:
            payload["messages"] = request.messages
        else:
            raise TokenCountError("Either text or messages required for OpenAI token counting")

        if request.tools:
            payload["tools"] = request.tools

        url = f"{self.config.base_url.rstrip('/')}{self.config.count_endpoint}"
        response = await self._post(url, payload)

        total = response.get("total_tokens", 0)

        duration_ms = (time.perf_counter() - start) * 1000

        return CountResult(
            model=request.model,
            resolved_model=model_info.id,
            provider="openai",
            tokenizer_used="openai-remote",
            method="remote",
            input_tokens=total,
            output_tokens=0,
            total_tokens=total,
            context_window=model_info.context_window,
            context_utilization=total / model_info.context_window if model_info.context_window else None,
            duration_ms=duration_ms,
        )


class AnthropicTokenAdapter(RemoteAdapterBase):
    """Anthropic token counting via /v1/messages/count_tokens endpoint."""

    def __init__(self, config: ProviderConfig):
        super().__init__(config)
        self.config.name = "anthropic"

    @property
    def name(self) -> str:
        return "anthropic"

    def supports_model(self, model: str) -> bool:
        return model.startswith("claude-")

    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        import time
        start = time.perf_counter()

        payload: dict[str, Any] = {"model": request.model}

        if request.messages:
            payload["messages"] = request.messages
        elif request.text:
            payload["messages"] = [{"role": "user", "content": request.text}]
        else:
            raise TokenCountError("Either text or messages required for Anthropic token counting")

        if request.tools:
            payload["tools"] = request.tools

        url = f"{self.config.base_url.rstrip('/')}{self.config.count_endpoint}"
        response = await self._post(url, payload)

        total = response.get("input_tokens", 0)

        duration_ms = (time.perf_counter() - start) * 1000

        return CountResult(
            model=request.model,
            resolved_model=model_info.id,
            provider="anthropic",
            tokenizer_used="anthropic-remote",
            method="remote",
            input_tokens=total,
            output_tokens=0,
            total_tokens=total,
            context_window=model_info.context_window,
            context_utilization=total / model_info.context_window if model_info.context_window else None,
            duration_ms=duration_ms,
        )


class GeminiTokenAdapter(RemoteAdapterBase):
    """Google Gemini token counting via /v1/models/{model}:countTokens endpoint."""

    def __init__(self, config: ProviderConfig):
        super().__init__(config)
        self.config.name = "gemini"

    @property
    def name(self) -> str:
        return "gemini"

    def supports_model(self, model: str) -> bool:
        return model.startswith("gemini-")

    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        import time
        start = time.perf_counter()

        payload: dict[str, Any] = {
            "contents": [],
        }

        if request.messages:
            for msg in request.messages:
                role = msg.get("role", "user")
                content = msg.get("content", "")
                if role == "assistant":
                    role = "model"
                payload["contents"].append({"role": role, "parts": [{"text": content}]})
        elif request.text:
            payload["contents"] = [{"role": "user", "parts": [{"text": request.text}]}]
        else:
            raise TokenCountError("Either text or messages required for Gemini token counting")

        url = f"{self.config.base_url.rstrip('/')}{self.config.count_endpoint.replace('{model}', request.model)}"
        response = await self._post(url, payload)

        total = response.get("totalTokens", 0)

        duration_ms = (time.perf_counter() - start) * 1000

        return CountResult(
            model=request.model,
            resolved_model=model_info.id,
            provider="gemini",
            tokenizer_used="gemini-remote",
            method="remote",
            input_tokens=total,
            output_tokens=0,
            total_tokens=total,
            context_window=model_info.context_window,
            context_utilization=total / model_info.context_window if model_info.context_window else None,
            duration_ms=duration_ms,
        )


async def create_remote_adapters(provider_configs: dict[str, ProviderConfig]) -> list[BaseAdapter]:
    """Factory to create remote adapters from config."""
    adapters = []

    if provider_configs.get("openai", ProviderConfig()).enabled:
        adapters.append(OpenAITokenAdapter(provider_configs["openai"]))

    if provider_configs.get("anthropic", ProviderConfig()).enabled:
        adapters.append(AnthropicTokenAdapter(provider_configs["anthropic"]))

    if provider_configs.get("gemini", ProviderConfig()).enabled:
        adapters.append(GeminiTokenAdapter(provider_configs["gemini"]))

    return adapters