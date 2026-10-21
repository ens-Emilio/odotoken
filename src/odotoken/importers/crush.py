"""Crush importer — resolves projects.json -> data_dir/**/*.jsonl (best effort)."""

from __future__ import annotations

import json
from pathlib import Path

from odotoken.importers.base import RawUsage, ScanResult, expand_path


class CrushImporter:
    name = "crush"

    def __init__(self, projects_json: str | Path = "~/.local/share/crush/projects.json"):
        self.projects_json = expand_path(projects_json)

    def detect(self) -> bool:
        return self.projects_json.exists()

    def _iter_dirs(self):
        if not self.projects_json.exists():
            return
        try:
            data = json.loads(self.projects_json.read_text(encoding="utf-8"))
        except Exception:
            return
        for p in data.get("projects", []):
            dd = p.get("data_dir")
            if dd:
                yield Path(dd)

    def _iter_files(self):
        for d in self._iter_dirs():
            if not d.exists():
                continue
            # sessions may be at d/*.jsonl or d/sessions/*.jsonl or d/**
            for pat in ("*.jsonl", "sessions/*.jsonl", "**/*.jsonl"):
                for f in d.glob(pat):
                    yield f
            break  # dedup: glob ** already covers
        # also check flat ** fallback without duplication
        seen = set()
        for d in self._iter_dirs():
            if not d.exists():
                continue
            for f in d.rglob("*.jsonl"):
                if str(f) not in seen:
                    seen.add(str(f))
                    # already yielded above, but rglob will re-yield; keep unique
                    yield f

    def scan(self, since: str | None = None) -> ScanResult:
        if not self.detect():
            return ScanResult(cli=self.name, available=False, path=str(self.projects_json))
        # unique files
        files = set(
            str(p)
            for d in self._iter_dirs()
            for p in Path(d).rglob("*.jsonl")
            if Path(p).exists()
        )
        return ScanResult(
            cli=self.name,
            available=True,
            session_files=len(files),
            path=str(self.projects_json),
        )

    def iter_usages(self, since: str | None = None):
        seen_files: set[str] = set()
        for filepath in list(self._iter_files()):
            sf = str(filepath)
            if sf in seen_files:
                continue
            seen_files.add(sf)
            session_id = filepath.stem
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
                        # Try to find usage-like fields (openai compat)
                        # crush may store assistant messages with usage
                        msg = obj.get("message") or obj
                        usage = msg.get("usage") or obj.get("usage") or {}
                        if not usage or not isinstance(usage, dict):
                            continue
                        # heuristic: must have prompt/completion tokens
                        prompt = int(usage.get("prompt_tokens") or usage.get("input_tokens") or 0)
                        completion = int(usage.get("completion_tokens") or usage.get("output_tokens") or 0)
                        if not prompt and not completion:
                            continue
                        ts = obj.get("timestamp") or msg.get("timestamp") or ""
                        if since and ts and ts < since:
                            continue
                        model = msg.get("model") or obj.get("model") or "unknown"
                        provider = msg.get("provider") or obj.get("provider") or "unknown"
                        yield RawUsage(
                            cli=self.name,
                            session_id=session_id,
                            session_file=sf,
                            entry_id=obj.get("id") or obj.get("uuid") or f"{session_id}:{idx}",
                            timestamp=ts,
                            provider=provider,
                            model=model,
                            prompt_tokens=prompt,
                            completion_tokens=completion,
                            cache_read_tokens=int(usage.get("cache_read_tokens") or 0),
                            cache_write_tokens=int(usage.get("cache_write_tokens") or 0),
                            project=str(filepath.parent),
                        )
            except OSError:
                continue
