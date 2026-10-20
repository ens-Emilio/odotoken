"""Base importer protocol + shared types."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Protocol


@dataclass
class RawUsage:
    cli: str
    session_id: str
    session_file: str
    entry_id: str
    timestamp: str  # ISO-8601 UTC
    provider: str
    model: str
    prompt_tokens: int = 0
    completion_tokens: int = 0
    cache_read_tokens: int = 0
    cache_write_tokens: int = 0
    project: str = "default"
    # optional: raw cost from original storage (e.g. OMP)
    raw_cost: float | None = None


@dataclass
class ScanResult:
    cli: str
    available: bool
    session_files: int = 0
    total_entries: int = 0
    path: str = ""


class Importer(Protocol):
    name: str

    def detect(self) -> bool: ...

    def scan(self, since: str | None = None) -> ScanResult: ...

    def iter_usages(self, since: str | None = None):  # type: ignore[no-untyped-def]
        """Yield RawUsage records (streaming)."""
        ...


def expand_path(p: str | Path) -> Path:
    return Path(p).expanduser().resolve() if str(p).startswith("~") else Path(p).expanduser()
