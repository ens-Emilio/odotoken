"""Model catalog service for token counting."""

from __future__ import annotations

import json
from pathlib import Path

from odotoken.domain.entities import ModelInfo
from odotoken.domain.errors import ModelNotSupportedError
from odotoken.infra.logging import get_logger

logger = get_logger(__name__)

ALLOWED_COUNTING_METHODS = {"remote", "local", "heuristic"}
# ponytail: manual validation, no jsonschema dep; add schema file if catalog grows past 50 models


def _validate_model_dict(data: dict) -> str | None:
    """Return error string if invalid, else None. Cheap structural checks only."""
    mid = data.get("id")
    if not isinstance(mid, str) or not mid.strip():
        return "missing or empty 'id'"
    aliases = data.get("aliases", [])
    if not isinstance(aliases, list) or not all(isinstance(a, str) for a in aliases):
        return "'aliases' must be list[str]"
    cm = data.get("counting_method", "heuristic")
    if cm not in ALLOWED_COUNTING_METHODS:
        return f"invalid counting_method '{cm}' (allowed: {ALLOWED_COUNTING_METHODS})"
    for k in ("context_window", "max_output", "chat_overhead", "message_overhead", "tool_overhead"):
        if k in data:
            v = data[k]
            if not isinstance(v, int) or isinstance(v, bool):
                return f"'{k}' must be int"
            if v < 0:
                return f"'{k}' must be >= 0"
    for k in ("supports_messages", "supports_tools", "supports_vision"):
        if k in data and not isinstance(data[k], bool):
            return f"'{k}' must be bool"
    for k in ("provider", "family", "notes", "cloud_equivalent"):
        if k in data and not isinstance(data[k], str):
            return f"'{k}' must be str"
    tok = data.get("tokenizer", None)
    if tok is not None and not isinstance(tok, str):
        return "'tokenizer' must be str or null"
    return None


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

        try:
            with open(self.catalog_path, encoding="utf-8") as f:
                data = json.load(f)
        except (json.JSONDecodeError, OSError) as e:
            logger.warning("Failed to load model catalog %s: %s", self.catalog_path, e)
            return

        if not isinstance(data, dict):
            logger.warning("Model catalog top-level must be object, got %s", type(data).__name__)
            return
        models_raw = data.get("models", [])
        if not isinstance(models_raw, list):
            logger.warning("Model catalog 'models' must be list, got %s", type(models_raw).__name__)
            return

        self._version = str(data.get("version", "unknown"))
        seen_ids: set[str] = set()

        for model_data in models_raw:
            if not isinstance(model_data, dict):
                logger.warning("Skipping non-object model entry: %r", model_data)
                continue
            err = _validate_model_dict(model_data)
            if err:
                logger.warning("Skipping invalid model entry %r: %s", model_data.get("id"), err)
                continue
            mid = str(model_data["id"])
            if mid.lower() in seen_ids:
                logger.warning("Duplicate model id '%s' — keeping last occurrence", mid)
            try:
                info = ModelInfo(**{k: v for k, v in model_data.items() if k in ModelInfo.__dataclass_fields__})  # type: ignore[attr-defined]
            except TypeError as e:
                logger.warning("Skipping model '%s': %s", mid, e)
                continue
            self._models[info.id] = info
            self._alias_map[info.id.lower()] = info.id
            for alias in info.aliases:
                # last alias wins silently; cheap and predictable
                self._alias_map[alias.lower()] = info.id
            seen_ids.add(mid.lower())

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

    def get_cheaper_alternatives(self, model: str) -> list[ModelInfo]:
        """Cheaper same-family alternatives sorted cheapest-first (by input+output rate)."""
        info = self.get_optional(model)
        if info is None:
            return []
        # late import to avoid circular
        try:
            from odotoken.pricing import DEFAULT_PRICES
        except Exception:
            return []
        # build pricing map (respect prices.json if present, cheapest without new deps)
        prices: dict[str, dict[str, float]] = dict(DEFAULT_PRICES)
        try:
            p = Path("prices.json")
            if p.exists():
                data = json.loads(p.read_text(encoding="utf-8"))
                if isinstance(data, dict):
                    for k, v in data.items():
                        if isinstance(v, dict):
                            prices[k.lower()] = v  # type: ignore[assignment]
        except Exception:
            pass

        def _rates(mid: str) -> dict[str, float]:
            clean = mid.lower().strip()
            if clean in prices:
                return prices[clean]
            for pat, rates in prices.items():
                if pat != "default" and pat in clean:
                    return rates
            return prices.get("default", {"input": 0.0, "output": 0.0, "cache_read": 0.0, "cache_write": 0.0})

        def _cost(r: dict[str, float]) -> float:
            return float(r.get("input", 0.0)) + float(r.get("output", 0.0))

        cur = _cost(_rates(info.id))
        # zero-rate (local free) has no cheaper alternative
        if cur == 0:
            return []
        cands: list[ModelInfo] = []
        for cand in self._models.values():
            if cand.id.lower() == info.id.lower():
                continue
            # same family only
            if info.family and cand.family and cand.family.lower() != info.family.lower():
                continue
            if info.family and not cand.family:
                continue
            cand_cost = _cost(_rates(cand.id))
            if cand_cost < cur - 1e-9:
                cands.append(cand)
        cands.sort(key=lambda m: _cost(_rates(m.id)))
        return cands

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
