"""Model catalog service for token counting."""

from __future__ import annotations

import json
from pathlib import Path

from odotoken.domain.entities import ModelInfo
from odotoken.domain.errors import ModelNotSupportedError
from odotoken.infra.logging import get_logger

logger = get_logger(__name__)


class ModelCatalog:
    """Loads and manages model information from catalog file."""

    def __init__(self, catalog_path: str | Path = "src/odotoken/catalog/models.json"):
        self.catalog_path = Path(catalog_path)
        self._models: dict[str, ModelInfo] = {}
        self._alias_map: dict[str, str] = {}
        self._version: str = ""
        self._load()

    def _load(self) -> None:
        if not self.catalog_path.exists():
            logger.warning("Model catalog not found at %s", self.catalog_path)
            return

        with open(self.catalog_path, encoding="utf-8") as f:
            data = json.load(f)

        self._version = data.get("version", "unknown")

        for model_data in data.get("models", []):
            info = ModelInfo(**model_data)
            self._models[info.id] = info
            self._alias_map[info.id.lower()] = info.id
            for alias in info.aliases:
                self._alias_map[alias.lower()] = info.id

        logger.info("Loaded %d models from catalog (version %s)", len(self._models), self._version)

    def get(self, model: str) -> ModelInfo:
        """Get model info by ID or alias."""
        normalized = model.lower().strip()
        canonical = self._alias_map.get(normalized)
        if not canonical:
            raise ModelNotSupportedError(model)
        return self._models[canonical]

    def get_optional(self, model: str) -> ModelInfo | None:
        """Get model info or None if not found."""
        try:
            return self.get(model)
        except ModelNotSupportedError:
            return None

    def list_models(self) -> list[ModelInfo]:
        """List all models in catalog."""
        return list(self._models.values())

    def list_by_provider(self, provider: str) -> list[ModelInfo]:
        """List models for a specific provider."""
        return [m for m in self._models.values() if m.provider.lower() == provider.lower()]

    def resolve_model(self, model: str, default_provider: str = "openai") -> ModelInfo:
        """Resolve model with fallback to default provider heuristics."""
        info = self.get_optional(model)
        if info:
            return info

        # Try to infer provider from model name prefix
        if model.startswith(("gpt-", "o1", "o3-")):
            provider = "openai"
        elif model.startswith("claude-"):
            provider = "anthropic"
        elif model.startswith("gemini-"):
            provider = "gemini"
        else:
            provider = default_provider

        # Return minimal ModelInfo for unknown models
        logger.warning("Model '%s' not in catalog, using default for provider '%s'", model, provider)
        return ModelInfo(
            id=model,
            provider=provider,
            context_window=128000,
            max_output=4096,
            supports_messages=True,
            supports_tools=False,
            supports_vision=False,
            counting_method="heuristic",
        )

    def get_cloud_equivalent(self, model: str) -> str:
        """Cloud-equivalent model id for shadow pricing (empty if unknown)."""
        info = self.get_optional(model)
        return info.cloud_equivalent if info else ""

    @property
    def version(self) -> str:
        return self._version

    def reload(self) -> None:
        """Reload catalog from file."""
        self._models.clear()
        self._alias_map.clear()
        self._load()
