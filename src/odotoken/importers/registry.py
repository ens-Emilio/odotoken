"""Registry: auto-detect all available importers."""

from __future__ import annotations

from odotoken.importers.base import ScanResult
from odotoken.importers.claude import ClaudeImporter
from odotoken.importers.codex import CodexImporter
from odotoken.importers.crush import CrushImporter
from odotoken.importers.omp import OmpImporter
from odotoken.importers.opencode import OpencodeImporter
from odotoken.importers.pi import PiImporter


def get_importers(config: dict | None = None):
    paths = {}
    if config:
        paths = config.get("sessions", {}).get("paths", {}) or {}
    importers = []
    importers.append(ClaudeImporter(paths.get("claude", "~/.claude/projects")))
    importers.append(CodexImporter(paths.get("codex", "~/.codex/sessions")))
    importers.append(OmpImporter(paths.get("omp", "~/.omp/stats.db")))
    importers.append(CrushImporter(paths.get("crush", "~/.local/share/crush/projects.json")))
    importers.append(OpencodeImporter(paths.get("opencode", "~/.local/share/opencode/opencode.db")))
    importers.append(PiImporter(paths.get("pi", "~/.pi/agent/sessions")))
    # allow custom path overrides but still include defaults if not overridden
    return importers


def scan_all(config: dict | None = None, since: str | None = None) -> list[ScanResult]:
    return [imp.scan(since=since) for imp in get_importers(config)]


def get_importer_by_name(name: str, config: dict | None = None):
    for imp in get_importers(config):
        if imp.name == name:
            return imp
    return None

