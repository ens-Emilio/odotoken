#!/usr/bin/env python3
"""Python client example for odotoken token counter API."""

import asyncio

import httpx


class TokenCounterClient:
    """Async client for odotoken API."""

    def __init__(self, base_url: str = "http://127.0.0.1:8081"):
        self.base_url = base_url
        self.client = httpx.AsyncClient(timeout=30.0)

    async def close(self):
        await self.client.aclose()

    async def count_text(self, model: str, text: str, provider: str | None = None) -> dict:
        """Count tokens for plain text."""
        payload = {"model": model, "text": text}
        if provider:
            payload["provider"] = provider
        response = await self.client.post(f"{self.base_url}/v1/count", json=payload)
        response.raise_for_status()
        return response.json()

    async def count_messages(
        self,
        model: str,
        messages: list[dict],
        provider: str | None = None,
        tools: list[dict] | None = None,
    ) -> dict:
        """Count tokens for chat messages."""
        payload = {"model": model, "messages": messages}
        if provider:
            payload["provider"] = provider
        if tools:
            payload["tools"] = tools
        response = await self.client.post(f"{self.base_url}/v1/count", json=payload)
        response.raise_for_status()
        return response.json()

    async def count_batch(self, items: list[dict]) -> dict:
        """Count tokens for multiple requests."""
        response = await self.client.post(f"{self.base_url}/v1/count/batch", json={"items": items})
        response.raise_for_status()
        return response.json()

    async def list_models(self, provider: str | None = None) -> dict:
        """List supported models."""
        url = f"{self.base_url}/v1/models"
        if provider:
            url += f"?provider={provider}"
        response = await self.client.get(url)
        response.raise_for_status()
        return response.json()

    async def get_model_info(self, model_id: str) -> dict:
        """Get detailed model information."""
        response = await self.client.get(f"{self.base_url}/v1/models/{model_id}")
        response.raise_for_status()
        return response.json()

    async def health(self) -> dict:
        response = await self.client.get(f"{self.base_url}/health")
        response.raise_for_status()
        return response.json()

    async def ready(self) -> dict:
        response = await self.client.get(f"{self.base_url}/ready")
        response.raise_for_status()
        return response.json()

    async def metrics(self) -> dict:
        response = await self.client.get(f"{self.base_url}/metrics")
        response.raise_for_status()
        return response.json()


async def main():
    async with TokenCounterClient() as client:
        print("=== Health Check ===")
        health = await client.health()
        print(f"Status: {health['status']}")
        print(f"Version: {health['version']}")

        print("\n=== List Models ===")
        models = await client.list_models()
        print(f"Total models: {models['total']}")

        print("\n=== Count Text ===")
        result = await client.count_text("gpt-4o-mini", "Hello world!")
        print(f"Tokens: {result['total_tokens']} (method: {result['method']})")

        print("\n=== Count Chat ===")
        messages = [
            {"role": "system", "content": "You are helpful"},
            {"role": "user", "content": "What is Python?"},
        ]
        result = await client.count_messages("gpt-4o-mini", messages)
        print(f"Tokens: {result['total_tokens']} (overhead: {result['overhead_tokens']})")

        print("\n=== Batch Count ===")
        batch = [
            {"model": "gpt-4o-mini", "text": "Test 1"},
            {"model": "gpt-4o-mini", "text": "Test 2"},
        ]
        result = await client.count_batch(batch)
        print(f"Total items: {result['total_items']}, Successful: {result['successful_items']}")
        for i, r in enumerate(result['results']):
            print(f"  Item {i}: {r['total_tokens']} tokens ({r['method']})")


if __name__ == "__main__":
    asyncio.run(main())
