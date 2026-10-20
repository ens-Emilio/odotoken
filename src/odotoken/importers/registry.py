"""Registry: auto-detect all available importers."""

from __future__ import annotations

from pathlib import Path

from odotoken.importers.base import ScanResult
from odotoken.importers.claude import ClaudeImporter
from odotoken.importers.codex import CodexImporter
from odotoken.importers.crush import CrushImporter
from odotoken.importers.omp import OmpImporter


def get_importers(config: dict | None = None):
    paths = {}
    if config:
        paths = config.get("sessions", {}).get("paths", {}) or {}
    importers = []
    importers.append(ClaudeImporter(paths.get("claude", "~/.claude/projects")))
    importers.append(CodexImporter(paths.get("codex", "~/.codex/sessions")))
    importers.append(OmpImporter(paths.get("omp", "~/.omp/stats.db")))
    importers.append(CrushImporter(paths.get("crush", "~/.local/share/crush/projects.json")))
    # allow custom path overrides but still include defaults if not overridden
    return importers


def scan_all(config: dict | None = None, since: str | None = None) -> list[ScanResult]:
    results = []
    for imp in get_importers(config):
        results.append(imp.scan(since=since))
    return results


def get_importer_by_name(name: str, config: dict | None = None):
    for imp in get_importers(config):
        if imp.name == name:
            return imp
    return None
