"""Smoke tests for Tier-1/2 dashboard endpoints (tps, forecast, export, guard limits, replay, tags)."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from pathlib import Path

import pytest
from starlette.testclient import TestClient

from odotoken.dashboard import register_dashboard_routes
from odotoken.db import Database, TokenEvent
from odotoken.guard import Guard
from odotoken.pricing import PricingCalculator
from odotoken.proxy import create_proxy_app


@pytest.fixture
def temp_db(tmp_path: Path):
    return Database(tmp_path / "test_dashboard.db")


@pytest.fixture
def pricing():
    return PricingCalculator()


def seed_events(db: Database) -> None:
    """Three distinct days of events so forecast/tps have data."""
    now = datetime.now(UTC)
    days = [now - timedelta(days=3), now - timedelta(days=2), now - timedelta(days=1)]
    for i, _ in enumerate(days):
        ev = TokenEvent(
            provider="anthropic",
            model="claude-3-5-sonnet-20241022",
            client_app="omp",
            prompt_tokens=1000,
            completion_tokens=200,
            total_tokens=1200,
            cost_usd=0.006 * (i + 1),
            latency_ms=450.0,
            status_code=200,
            tags="prod" if i % 2 == 0 else "dev",
        )
        db.record_event(ev)


def make_client(temp_db: Database, pricing) -> TestClient:
    guard = Guard(temp_db)
    app = create_proxy_app(temp_db, pricing, guard, {})
    register_dashboard_routes(app, temp_db, guard)
    return TestClient(app)


def test_tier2_endpoints_respond(temp_db: Database, pricing):
    seed_events(temp_db)
    client = make_client(temp_db, pricing)

    # TPS endpoint
    res = client.get("/api/tps?interval=1h")
    assert res.status_code == 200
    data = res.json()
    assert "series" in data and "summary" in data and "by_model" in data
    assert data["interval"] == "1h"

    # Forecast endpoint
    res = client.get("/api/forecast")
    assert res.status_code == 200
    fdata = res.json()
    assert "forecast" in fdata and "anomalies" in fdata and "daily" in fdata
    # 3 event days → linear forecast, anomalies list present
    assert len(fdata["daily"]) >= 1

    # CSV export
    res = client.get("/api/export?format=csv")
    assert res.status_code == 200
    assert res.headers["content-type"].startswith("text/csv")
    assert "id,timestamp,provider" in res.text

    # JSON export
    res = client.get("/api/export?format=json")
    assert res.status_code == 200
    assert isinstance(res.json(), list)

    # Stats with tag filter
    res = client.get("/api/stats?tag=prod")
    assert res.status_code == 200
    sdata = res.json()
    assert "by_tag" in sdata
    assert any(t["tag"] == "prod" for t in sdata["by_tag"])


def test_guard_limits_patch_and_stats_guard(temp_db: Database, pricing):
    seed_events(temp_db)
    client = make_client(temp_db, pricing)

    # /api/stats carries guard block
    res = client.get("/api/stats")
    assert res.status_code == 200
    guard_info = res.json().get("guard")
    assert guard_info is not None and "status" in guard_info

    # PATCH guard limits (merge)
    body = {"daily_cost_limit_usd": 5.0, "projects": {"web": {"daily_cost_limit_usd": 1.0}}}
    res = client.patch("/api/guard/limits", json=body)
    assert res.status_code == 200
    patched = res.json()["limits"]
    assert patched.get("daily_cost_limit_usd") == 5.0
    assert patched.get("projects", {}).get("web", {}).get("daily_cost_limit_usd") == 1.0


def test_replay_dry_run_reports_diff(temp_db: Database, pricing):
    seed_events(temp_db)
    client = make_client(temp_db, pricing)

    res = client.get("/api/replay?dry_run=true")
    assert res.status_code == 200
    r = res.json()
    assert "total" in r and "changed" in r and "dry_run" in r
    assert r["dry_run"] is True
    assert r["total"] == 3
