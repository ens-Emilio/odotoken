"""Integration tests for token counting API."""

from __future__ import annotations

import pytest
import pytest_asyncio
from httpx import AsyncClient, ASGITransport

from odotoken.api.app import create_app


@pytest.fixture
def app():
    return create_app("config.json")


@pytest_asyncio.fixture
async def client(app):
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac


class TestHealthEndpoints:
    """Tests for health and readiness endpoints."""

    @pytest.mark.asyncio
    async def test_health(self, client):
        response = await client.get("/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"
        assert "timestamp" in data

    @pytest.mark.asyncio
    async def test_ready(self, client):
        response = await client.get("/ready")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] in ("ready", "not_ready")
        assert "checks" in data

    @pytest.mark.asyncio
    async def test_metrics(self, client):
        response = await client.get("/metrics")
        assert response.status_code == 200
        data = response.json()
        assert "counters" in data
        assert "histograms" in data
        assert "gauges" in data


class TestModelsEndpoints:
    """Tests for models endpoints."""

    @pytest.mark.asyncio
    async def test_list_models(self, client):
        response = await client.get("/v1/models")
        assert response.status_code == 200
        data = response.json()
        assert "models" in data
        assert "total" in data
        assert data["total"] == 8

    @pytest.mark.asyncio
    async def test_list_models_filter_provider(self, client):
        response = await client.get("/v1/models?provider=openai")
        assert response.status_code == 200
        data = response.json()
        assert all(m["provider"] == "openai" for m in data["models"])

    @pytest.mark.asyncio
    async def test_get_model(self, client):
        response = await client.get("/v1/models/gpt-4o-mini")
        assert response.status_code == 200
        data = response.json()
        assert data["id"] == "gpt-4o-mini"
        assert data["provider"] == "openai"

    @pytest.mark.asyncio
    async def test_get_unknown_model(self, client):
        response = await client.get("/v1/models/unknown-model")
        assert response.status_code == 404


class TestCountEndpoints:
    """Tests for token counting endpoints."""

    @pytest.mark.asyncio
    async def test_count_text(self, client):
        response = await client.post(
            "/v1/count",
            json={"model": "gpt-4o-mini", "text": "Hello world"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["model"] == "gpt-4o-mini"
        assert data["total_tokens"] > 0
        assert data["method"] == "heuristic"
        assert "warnings" in data

    @pytest.mark.asyncio
    async def test_count_messages(self, client):
        response = await client.post(
            "/v1/count",
            json={
                "model": "gpt-4o-mini",
                "messages": [
                    {"role": "user", "content": "Hello"},
                    {"role": "assistant", "content": "Hi there!"},
                ],
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["total_tokens"] > 0
        assert data["overhead_tokens"] > 0
        assert len(data["tokens_per_message"]) == 2

    @pytest.mark.asyncio
    async def test_count_invalid_model(self, client):
        response = await client.post(
            "/v1/count",
            json={"model": "unknown-model", "text": "test"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["method"] == "heuristic"

    @pytest.mark.asyncio
    async def test_count_batch(self, client):
        response = await client.post(
            "/v1/count/batch",
            json={
                "items": [
                    {"model": "gpt-4o-mini", "text": "Test 1"},
                    {"model": "gpt-4o-mini", "text": "Test 2"},
                ]
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["total_items"] == 2
        assert data["successful_items"] == 2
        assert data["total_tokens"] > 0
        assert len(data["results"]) == 2

    @pytest.mark.asyncio
    async def test_count_batch_messages(self, client):
        response = await client.post(
            "/v1/count/batch",
            json={
                "items": [
                    {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]},
                    {"model": "gpt-4o-mini", "text": "Plain text"},
                ]
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["total_items"] == 2
        assert data["successful_items"] == 2

    @pytest.mark.asyncio
    async def test_count_batch_too_large(self, client):
        items = [{"model": "gpt-4o-mini", "text": f"Test {i}"} for i in range(101)]
        response = await client.post("/v1/count/batch", json={"items": items})
        assert response.status_code == 422