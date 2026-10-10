"""Unit tests for model catalog."""

from __future__ import annotations

import pytest

from odotoken.services.model_catalog import ModelCatalog
from odotoken.domain.errors import ModelNotSupportedError


class TestModelCatalog:
    """Tests for ModelCatalog."""

    @pytest.fixture
    def catalog(self):
        return ModelCatalog("src/odotoken/catalog/models.json")

    def test_load_catalog(self, catalog):
        models = catalog.list_models()
        assert len(models) == 8

    def test_get_existing_model(self, catalog):
        model = catalog.get("gpt-4o-mini")
        assert model.id == "gpt-4o-mini"
        assert model.provider == "openai"
        assert model.counting_method == "remote"

    def test_get_model_by_alias(self, catalog):
        model = catalog.get("gpt-4o-mini-2024-07-18")
        assert model.id == "gpt-4o-mini"

    def test_get_unknown_model_raises(self, catalog):
        with pytest.raises(ModelNotSupportedError):
            catalog.get("unknown-model-xyz")

    def test_get_optional_returns_none(self, catalog):
        assert catalog.get_optional("unknown-model-xyz") is None

    def test_list_by_provider(self, catalog):
        openai_models = catalog.list_by_provider("openai")
        assert len(openai_models) == 2
        assert all(m.provider == "openai" for m in openai_models)

        anthropic_models = catalog.list_by_provider("anthropic")
        assert len(anthropic_models) == 2

    def test_resolve_model_known(self, catalog):
        model = catalog.resolve_model("gpt-4o-mini")
        assert model.id == "gpt-4o-mini"

    def test_resolve_model_unknown_gpt_prefix(self, catalog):
        model = catalog.resolve_model("gpt-4-turbo")
        assert model.id == "gpt-4-turbo"
        assert model.provider == "openai"
        assert model.counting_method == "heuristic"

    def test_resolve_model_unknown_claude_prefix(self, catalog):
        model = catalog.resolve_model("claude-3-opus")
        assert model.id == "claude-3-opus"
        assert model.provider == "anthropic"

    def test_resolve_model_unknown_gemini_prefix(self, catalog):
        model = catalog.resolve_model("gemini-1.5-pro")
        assert model.id == "gemini-1.5-pro"
        assert model.provider == "gemini"

    def test_catalog_version(self, catalog):
        assert catalog.version == "1.0.0"