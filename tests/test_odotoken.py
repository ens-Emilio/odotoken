"""Comprehensive unit and integration test suite for odotoken."""

from __future__ import annotations

import json
from pathlib import Path
import pytest
from starlette.testclient import TestClient

from odotoken.db import Database, TokenEvent
from odotoken.guard import Guard, GuardLimits
from odotoken.parser import StreamUsageAccumulator, UsageParser
from odotoken.pricing import PricingCalculator
from odotoken.proxy import create_proxy_app
from odotoken.dashboard import register_dashboard_routes


@pytest.fixture
def temp_db(tmp_path: Path):
    db_file = tmp_path / "test_odotoken.db"
    return Database(db_file)


@pytest.fixture
def pricing():
    return PricingCalculator()


# --- Database Tests ---

def test_database_record_and_summary(temp_db: Database):
    event = TokenEvent(
        provider="anthropic",
        model="claude-3-5-sonnet-20241022",
        client_app="omp",
        prompt_tokens=1000,
        completion_tokens=200,
        total_tokens=1200,
        cache_read_tokens=400,
        cache_write_tokens=100,
        cost_usd=0.006,
        latency_ms=450.0,
        status_code=200,
    )
    event_id = temp_db.record_event(event)
    assert event_id is not None
    assert event_id > 0

    summary = temp_db.get_summary()
    assert summary["total_requests"] == 1
    assert summary["total_prompt_tokens"] == 1000
    assert summary["total_completion_tokens"] == 200
    assert summary["total_tokens"] == 1200
    assert summary["total_cache_read_tokens"] == 400
    assert summary["total_cost_usd"] == pytest.approx(0.006, 0.0001)

    by_model = temp_db.get_summary_by_model()
    assert len(by_model) == 1
    assert by_model[0]["model"] == "claude-3-5-sonnet-20241022"

    by_client = temp_db.get_summary_by_client()
    assert len(by_client) == 1
    assert by_client[0]["client_app"] == "omp"

    recent = temp_db.get_recent_events(limit=10)
    assert len(recent) == 1
    assert recent[0]["client_app"] == "omp"


# --- Pricing Calculator Tests ---

def test_pricing_calculator(pricing: PricingCalculator):
    # claude-3-5-sonnet: $3.00 / M prompt, $15.00 / M completion
    # 10,000 prompt = 0.03 USD, 1,000 completion = 0.015 USD => 0.045 USD
    cost = pricing.calculate_cost(
        model="claude-3-5-sonnet-20241022",
        prompt_tokens=10000,
        completion_tokens=1000,
        cache_read_tokens=0,
        cache_write_tokens=0,
    )
    assert cost == pytest.approx(0.045, 0.0001)

    # Prefix match: gpt-4o-2024-11-20 -> gpt-4o ($2.50 / $10.00)
    cost_gpt = pricing.calculate_cost(
        model="gpt-4o-2024-11-20",
        prompt_tokens=10000,
        completion_tokens=1000,
    )
    assert cost_gpt == pytest.approx(0.035, 0.0001)


# --- Usage Parser Tests ---

def test_usage_parser_anthropic_sse():
    acc = StreamUsageAccumulator(provider="anthropic", default_model="")

    # 1. message_start chunk
    chunk1 = (
        b'event: message_start\n'
        b'data: {"type":"message_start","message":{"id":"msg_123","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":150,"cache_read_input_tokens":50,"cache_creation_input_tokens":20}}}\n\n'
    )
    acc.process_chunk(chunk1)

    # 2. content_block_delta chunk
    chunk2 = b'event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}\n\n'
    acc.process_chunk(chunk2)

    # 3. message_delta chunk
    chunk3 = (
        b'event: message_delta\n'
        b'data: {"type":"message_delta","usage":{"output_tokens":42}}\n\n'
    )
    acc.process_chunk(chunk3)

    result = acc.finalize()
    assert result.model == "claude-3-5-sonnet-20241022"
    assert result.prompt_tokens == 150
    assert result.completion_tokens == 42
    assert result.cache_read_tokens == 50
    assert result.cache_write_tokens == 20
    assert result.total_tokens == 192


def test_usage_parser_openai_sse():
    acc = StreamUsageAccumulator(provider="openai", default_model="gpt-4o")

    chunk = (
        b'data: {"id":"chatcmpl-123","model":"gpt-4o-mini","choices":[{"delta":{"content":"Hi"}}],"usage":{"prompt_tokens":80,"completion_tokens":25,"total_tokens":105,"prompt_tokens_details":{"cached_tokens":30}}}\n\n'
    )
    acc.process_chunk(chunk)

    result = acc.finalize()
    assert result.model == "gpt-4o-mini"
    assert result.prompt_tokens == 80
    assert result.completion_tokens == 25
    assert result.cache_read_tokens == 30
    assert result.total_tokens == 105


def test_usage_parser_json_responses():
    # Anthropic JSON
    anthropic_json = {
        "id": "msg_abc",
        "model": "claude-3-7-sonnet-20250219",
        "usage": {
            "input_tokens": 500,
            "output_tokens": 120,
            "cache_read_input_tokens": 100,
            "cache_creation_input_tokens": 0,
        },
    }
    u1 = UsageParser.parse_json_response("anthropic", anthropic_json)
    assert u1.model == "claude-3-7-sonnet-20250219"
    assert u1.prompt_tokens == 500
    assert u1.completion_tokens == 120
    assert u1.cache_read_tokens == 100

    # OpenAI JSON
    openai_json = {
        "id": "chatcmpl-999",
        "model": "gpt-4o",
        "usage": {
            "prompt_tokens": 300,
            "completion_tokens": 80,
            "prompt_tokens_details": {"cached_tokens": 50},
        },
    }
    u2 = UsageParser.parse_json_response("openai", openai_json)
    assert u2.model == "gpt-4o"
    assert u2.prompt_tokens == 300
    assert u2.completion_tokens == 80
    assert u2.cache_read_tokens == 50


def test_client_identification():
    assert UsageParser.identify_client_app({"user-agent": "omp/1.0.0"}) == "omp"
    assert UsageParser.identify_client_app({"user-agent": "pi-agent/0.9.1"}) == "pi agent"
    assert UsageParser.identify_client_app({"user-agent": "OpenCode-CLI/2.0"}) == "opencode"
    assert UsageParser.identify_client_app({"x-client-id": "custom-agent"}) == "custom-agent"


# --- Guard Tests ---

def test_guard_thresholds(temp_db: Database):
    guard = Guard(
        temp_db,
        GuardLimits(daily_cost_limit_usd=0.10, action_on_breach="block"),
    )
    # Check allowance initially -> OK
    res = guard.check_allowance()
    assert res.allowed is True
    assert res.status == "ok"

    # Add expense exceeding daily limit
    temp_db.record_event(
        TokenEvent(
            provider="anthropic",
            model="claude-3-5-sonnet",
            client_app="omp",
            prompt_tokens=50000,
            completion_tokens=10000,
            total_tokens=60000,
            cost_usd=0.30,  # Exceeds 0.10 limit
            latency_ms=500.0,
            status_code=200,
        )
    )

    res_breach = guard.check_allowance()
    assert res_breach.allowed is False
    assert res_breach.status == "breached"
    assert "Daily budget limit breached" in res_breach.reason


# --- App & Dashboard Routes Tests ---

def test_dashboard_and_stats_routes(temp_db: Database, pricing: PricingCalculator):
    guard = Guard(temp_db)
    app = create_proxy_app(temp_db, pricing, guard, {})
    register_dashboard_routes(app, temp_db, guard)

    client = TestClient(app)

    # Health check
    res_health = client.get("/health")
    assert res_health.status_code == 200
    assert res_health.json()["status"] == "ok"

    # Dashboard view HTML
    res_dash = client.get("/dashboard")
    assert res_dash.status_code == 200
    assert "odotoken" in res_dash.text  # React build (title or root div)

    # Stats API
    res_stats = client.get("/api/stats")
    assert res_stats.status_code == 200
    data = res_stats.json()
    assert "summary" in data
    assert "by_model" in data
    assert "by_client" in data
    assert "guard" in data
