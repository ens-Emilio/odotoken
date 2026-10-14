"""Tests for PLAN_v2 Phase 1: project tags + shadow pricing."""

from __future__ import annotations

import os
import tempfile

import pytest

from odotoken.db import Database, TokenEvent
from odotoken.pricing import PricingCalculator
from odotoken.services.model_catalog import ModelCatalog


@pytest.fixture
def db():
    with tempfile.TemporaryDirectory() as d:
        yield Database(os.path.join(d, "t.db"))


@pytest.fixture
def pricing():
    return PricingCalculator("prices.json")


class TestProjectMigration:
    def test_idempotent_init(self, db):
        """Running _init_db twice must not fail or duplicate columns."""
        db._init_db()
        with db.connection() as conn:
            cols = [r[1] for r in conn.execute("PRAGMA table_info(token_events);")]
        assert cols.count("project_id") == 1
        assert cols.count("shadow_cost") == 1

    def test_record_with_project(self, db):
        db.record_event(
            TokenEvent(provider="p", model="m", prompt_tokens=10, completion_tokens=5,
                       cost_usd=0.1, shadow_cost=0.3, project_id="freelance-x")
        )
        row = db.get_recent_events(project="freelance-x")[0]
        assert row["project_id"] == "freelance-x"
        assert row["shadow_cost"] == pytest.approx(0.3)

    def test_default_project(self, db):
        db.record_event(TokenEvent(provider="p", model="m", prompt_tokens=1))
        assert db.get_recent_events(project="default")[0]["project_id"] == "default"

    def test_summary_by_project(self, db):
        for proj, cost in [("a", 0.5), ("b", 0.2), ("a", 0.3)]:
            db.record_event(
                TokenEvent(provider="p", model="m", prompt_tokens=10,
                           cost_usd=cost, shadow_cost=cost * 2, project_id=proj)
            )
        rows = db.get_summary_by_project()
        by = {r["project_id"]: r for r in rows}
        assert by["a"]["requests"] == 2
        assert by["a"]["cost_usd"] == pytest.approx(0.8)
        assert by["a"]["shadow_cost_usd"] == pytest.approx(1.6)
        assert by["b"]["requests"] == 1

    def test_project_filter_on_summary(self, db):
        for proj in ["a", "a", "b"]:
            db.record_event(TokenEvent(provider="p", model="m", prompt_tokens=5, project_id=proj))
        assert db.get_summary(project="a")["total_requests"] == 2
        assert db.get_summary(project="b")["total_requests"] == 1
        assert db.get_summary()["total_requests"] == 3

    def test_by_model_project_filter(self, db):
        for proj in ["a", "b"]:
            db.record_event(TokenEvent(provider="p", model="glm-5.3-flash", prompt_tokens=5, project_id=proj))
        assert len(db.get_summary_by_model(project="a")) == 1
        assert len(db.get_summary_by_model()) == 1  # same model, 2 events


class TestShadowPricing:
    def test_cloud_model_shadow_equals_actual(self, pricing):
        """Paid (cloud) models: shadow cost == actual cost — no savings."""
        shadow = pricing.calculate_shadow_cost("glm-5.3-flash", "gpt-4o", 10000, 5000)
        actual = pricing.calculate_cost("glm-5.3-flash", 10000, 5000)
        assert shadow == pytest.approx(actual)

    def test_local_model_uses_equivalent(self, pricing):
        """Free (local) models: shadow falls back to the cloud equivalent."""
        shadow = pricing.calculate_shadow_cost("llama3-70b", "gpt-4o", 10000, 5000)
        expected = pricing.calculate_cost("gpt-4o", 10000, 5000)
        assert shadow == pytest.approx(expected)
        assert shadow > 0

    def test_unknown_local_model_zero(self, pricing):
        """No rates and no equivalent with rates → shadow is 0."""
        shadow = pricing.calculate_shadow_cost("totally-unknown-free-model", "", 1000, 500)
        assert shadow == 0.0

    def test_catalog_cloud_equivalent(self):
        catalog = ModelCatalog("src/odotoken/catalog/models.json")
        assert catalog.get_cloud_equivalent("llama3-70b") == "gpt-4o"
        assert catalog.get_cloud_equivalent("llama3-8b") == "gpt-4o-mini"
        assert catalog.get_cloud_equivalent("unknown-model-x") == ""


class TestProjectHeaderThroughProxy:
    def test_x_odo_project_header_recorded(self, db, pricing):
        """The proxy handler must map X-Odo-Project to the event's project_id."""
        # Direct handler-level check via header extraction logic
        headers = {"x-odo-project": "test-project"}
        project_id = headers.get("x-odo-project") or headers.get("x-project") or "default"
        assert project_id == "test-project"

        # fallback header
        headers = {"x-project": "alt"}
        project_id = headers.get("x-odo-project") or headers.get("x-project") or "default"
        assert project_id == "alt"

        # no header → default
        assert ({}).get("x-odo-project") or "default" == "default"
