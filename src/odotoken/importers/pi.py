"""Pi agent importer — ~/.pi/agent/sessions/**/*.jsonl (pi agent transcripts).

Assistant events carry provider/model and a usage object:
  {input, output, cacheRead, cacheWrite, reasoning, totalTokens, cost:{total}}.
project dir is encoded in the parent folder name (--home-zens-.od-projects-...--).
"""

from __future__ import annotations

import json
from pathlib import Path

from odotoken.importers.base import RawUsage, ScanResult, expand_path


class PiImporter:
    name = "pi"

    def __init__(self, sessions_dir: str | Path = "~/.pi/agent/sessions"):
        self.sessions_dir = expand_path(sessions_dir)

    def detect(self) -> bool:
        return self.sessions_dir.exists() and any(self.sessions_dir.rglob("*.jsonl"))

    def _iter_files(self):
        if not self.sessions_dir.exists():
            return
        yield from sorted(self.sessions_dir.rglob("*.jsonl"))

    def scan(self, since: str | None = None) -> ScanResult:
        if not self.detect():
            return ScanResult(cli=self.name, available=False, path=str(self.sessions_dir))
        files = list(self._iter_files())
        return ScanResult(
            cli=self.name,
            available=True,
            session_files=len(files),
            path=str(self.sessions_dir),
        )

    def iter_usages(self, since: str | None = None):
        for filepath in self._iter_files():
            session_id = filepath.stem.split("_")[-1] if "_" in filepath.stem else filepath.stem
            project = self._project_from_dir(filepath)
            try:
                with open(filepath, encoding="utf-8") as f:
                    for idx, line in enumerate(f):
                        line = line.strip()
                        if not line:
                            continue
                        try:
                            obj = json.loads(line)
                        except json.JSONDecodeError:
                            continue
                        if obj.get("type") != "message":
                            continue
                        msg = obj.get("message") or {}
                        if msg.get("role") != "assistant":
                            continue
                        usage = msg.get("usage") or {}
                        if not usage:
                            continue
                        ts = obj.get("timestamp") or ""
                        if since and ts and ts < since:
                            continue
                        model = msg.get("model") or "unknown"
                        provider = msg.get("provider") or "unknown"
                        cost = (usage.get("cost") or {}).get("total")
                        yield RawUsage(
                            cli=self.name,
                            session_id=session_id,
                            session_file=str(filepath),
                            entry_id=msg.get("responseId") or f"{filepath}:{idx}",
                            timestamp=ts,
                            provider=provider,
                            model=model,
                            prompt_tokens=int(usage.get("input", 0) or 0),
                            completion_tokens=int(usage.get("output", 0) or 0),
                            cache_read_tokens=int(usage.get("cacheRead", 0) or 0),
                            cache_write_tokens=int(usage.get("cacheWrite", 0) or 0),
                            project=project,
                            raw_cost=float(cost or 0.0),
                        )
            except OSError:
                continue

    @staticmethod
    def _project_from_dir(filepath: Path) -> str:
        # parent folder like --home-zens-.od-projects-<uuid>-- -> decoded path
        name = filepath.parent.name.strip("-")
        if name.startswith("-") and "-" in name:
            # pi encodes path as -a-b-c- under -- markers
            return name.lstrip("-").replace("-", "/")
        return name or "default"
