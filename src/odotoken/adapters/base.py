"""Base adapter protocol for token counting."""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Protocol

from odotoken.domain.entities import CountRequest, CountResult, ModelInfo


class TokenCountAdapter(Protocol):
    """Protocol for token counting adapters."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Unique adapter name."""
        ...

    @abstractmethod
    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        """Count tokens for a request."""
        ...

    @abstractmethod
    def supports_model(self, model: str) -> bool:
        """Check if this adapter can handle the given model."""
        ...


class BaseAdapter(ABC):
    """Abstract base class for token counting adapters."""

    @property
    @abstractmethod
    def name(self) -> str:
        pass

    @abstractmethod
    async def count(self, request: CountRequest, model_info: ModelInfo) -> CountResult:
        pass

    @abstractmethod
    def supports_model(self, model: str) -> bool:
        pass