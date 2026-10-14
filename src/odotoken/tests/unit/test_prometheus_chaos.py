"""Tests for PLAN_v2 Phase 3: Prometheus format, MockAdapter, ChaosMiddleware."""

from __future__ import annotations

import os
import tempfile

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from odotoken.api.chaos import ChaosMiddleware, is_mock_upstream, synthesize_response
from odotoken.api.prometheus_metrics import register_prometheus_route, render_prometheus
from odotoken.db import Database, TokenEvent
from odotoken.guard import Guard


@pytest.fixture
def db():
    with tempfile.TemporaryDirectory() as d:
        yield Database(os.path.join(d, "t.db"))


class TestPrometheusFormat:
    def test_render_has_help_type_and_series(self, db):
        db.record_event(
            TokenEvent(
                provider="verboo",
                model="glm-5.3-flash",
                prompt_tokens=10,
                completion_tokens=5,
                cost_usd=0.01,
                shadow_cost=0.02,
                project_id="p1",
            )
        )
        out = render_prometheus(db)
        assert "# HELP odotoken_requests_total" in out
        assert "# TYPE odotoken_requests_total counter" in out
        assert "odotoken_requests_total 1" in out
        assert 'odotoken_tokens_total{direction="prompt"} 10' in out
        assert 'odotoken_cost_dollars_total{kind="actual"} 0.010000' in out
        assert 'odotoken_project_cost_dollars_total{project="p1"}' in out
        assert 'odotoken_model_requests_total{model="glm-5.3-flash",provider="verboo"} 1' in out

    def test_labels_escaped(self, db):
        db.record_event(TokenEvent(provider='p"1', model="m\\x", prompt_tokens=1, project_id='proj"q'))
        out = render_prometheus(db)
        assert 'proj\\"q' in out  # quote escaped in labels

    def test_guard_gauge_when_limit_set(self, db):
        from odotoken.guard import GuardLimits

        guard = Guard(db, GuardLimits(monthly_cost_limit_usd=10.0))
        out = render_prometheus(db, guard)
        assert "odotoken_budget_remaining_percentage" in out
        # no usage recorded → 100% remaining
        assert "odotoken_budget_remaining_percentage 100.00" in out

    def test_route_serves_text(self, db):
        app = FastAPI()
        register_prometheus_route(app, db)
        client = TestClient(app)
        res = client.get("/metrics")
        assert res.status_code == 200
        assert res.headers["content-type"].startswith("text/plain")
        assert "odotoken_requests_total" in res.text


class TestMockAdapter:
    def test_synthetic_response_shape(self):
        r = synthesize_response("mock-model", "hello world test")
        assert r["object"] == "chat.completion"
        assert r["model"] == "mock-model"
        u = r["usage"]
        assert u["prompt_tokens"] == 4  # len("hello world test")//4 = 16//4
        assert u["total_tokens"] == u["prompt_tokens"] + u["completion_tokens"]
        assert 10 <= u["completion_tokens"] <= 60

    def test_deterministic_per_input(self):
        a = synthesize_response("m", "same input")
        b = synthesize_response("m", "same input")
        assert a == b
        c = synthesize_response("m", "different input")
        assert a != c

    def test_is_mock_upstream(self):
        assert is_mock_upstream("mock://local")
        assert not is_mock_upstream("http://127.0.0.1:8791")
        assert not is_mock_upstream("https://api.openai.com")


class TestChaosMiddleware:
    def _app_with(self, cfg):
        app = FastAPI()
        app.add_middleware(ChaosMiddleware, chaos_cfg=cfg)

        @app.get("/ping")
        async def ping():
            return {"ok": True}

        return TestClient(app)

    def test_disabled_never_interferes(self):
        client = self._app_with({"enabled": False, "rate": 1.0})
        for _ in range(10):
            assert client.get("/ping").status_code == 200

    def test_zero_rate_never_fails(self):
        client = self._app_with({"enabled": True, "rate": 0.0, "errors": [500]})
        for _ in range(10):
            assert client.get("/ping").status_code == 200

    def test_rate_one_seed_forces_error(self):
        # seed chosen so the first roll lands in the error branch
        client = self._app_with({"enabled": True, "rate": 1.0, "errors": [429], "seed": 7})
        res = client.get("/ping")
        assert res.status_code in (429, 200)  # branch depends on roll
        # with rate=1 something ALWAYS happens; error OR latency path (still 200)
        chaos_detected = res.status_code == 429
        # try a few seeds to hit the error branch at least once
        for seed in range(20):
            c2 = self._app_with({"enabled": True, "rate": 1.0, "errors": [429], "seed": seed})
            if c2.get("/ping").status_code == 429:
                chaos_detected = True
                break
        assert chaos_detected

    def test_bypass_header(self):
        # rate 1.0 with a seed that errors; bypass must skip chaos entirely
        for seed in range(20):
            client = self._app_with({"enabled": True, "rate": 1.0, "errors": [429], "seed": seed})
            res = client.get("/ping", headers={"x-odo-no-chaos": "true"})
            assert res.status_code == 200
